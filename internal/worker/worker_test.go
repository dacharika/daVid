package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/mlnomadpy/daVid/internal/axerror"
)

// fakeWorker returns a Worker that re-execs this test binary as the subprocess,
// running only TestHelperProcess. The mode argument selects which fake-worker
// behaviour that helper performs, so a single test binary can stand in for a
// real Python worker over real stdin/stdout/exit-code paths without requiring a
// python interpreter on PATH.
func fakeWorker(mode string) Worker {
	return Worker{
		Path: os.Args[0],
		Args: []string{"-test.run=TestHelperProcess"},
		Env:  []string{"GO_WANT_HELPER_PROCESS=1", "WORKER_MODE=" + mode},
	}
}

// TestHelperProcess is not a real test: when GO_WANT_HELPER_PROCESS is set it is
// re-exec'd by fakeWorker as a stand-in Python worker. It reads the JSON Request
// from stdin and, depending on WORKER_MODE, prints a JSON Response to stdout
// (and/or exits non-zero) to exercise each of the invoker's paths.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// From here on we ARE the worker subprocess; exit explicitly so the go test
	// framework's own reporting never reaches the parent's stdout.
	in, _ := io.ReadAll(os.Stdin)
	var req Request
	_ = json.Unmarshal(in, &req)

	switch os.Getenv("WORKER_MODE") {
	case "echo":
		// Round-trip: reflect the request's op and params back as the result,
		// speaking the same contract version the request carried.
		resp := Response{
			Version: req.Version,
			Result: map[string]any{
				"op":     req.Op,
				"params": req.Params,
			},
		}
		out, _ := json.Marshal(resp)
		os.Stdout.Write(out)
		os.Exit(0)
	case "worker-error":
		// A structured, worker-reported failure printed on stdout with exit 0.
		resp := Response{
			Version: ContractVersion,
			Error: &ResponseError{
				Code:      "transcribe.model_missing",
				Message:   "whisper model not found",
				Hint:      "download the model into the cache directory",
				Retryable: true,
			},
		}
		out, _ := json.Marshal(resp)
		os.Stdout.Write(out)
		os.Exit(0)
	case "bad-version":
		resp := Response{Version: ContractVersion + 1, Result: map[string]any{"ok": true}}
		out, _ := json.Marshal(resp)
		os.Stdout.Write(out)
		os.Exit(0)
	case "not-json":
		os.Stdout.WriteString("this is not json\n")
		os.Exit(0)
	case "silent":
		// Exit cleanly having printed nothing at all.
		os.Exit(0)
	case "crash":
		os.Stderr.WriteString("Traceback (most recent call last): boom\n")
		os.Exit(3)
	case "crash-with-error":
		// Non-zero exit but a structured error was still printed to stdout; the
		// invoker should honour the structured error over a generic crash.
		resp := Response{
			Version: ContractVersion,
			Error: &ResponseError{
				Code:    "reframe.no_face",
				Message: "no active speaker detected",
			},
		}
		out, _ := json.Marshal(resp)
		os.Stdout.Write(out)
		os.Exit(1)
	}
	os.Exit(0)
}

// asAxError unwraps err into an *axerror.Error, failing the test if it is not one.
func asAxError(t *testing.T, err error) *axerror.Error {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var ax *axerror.Error
	if !errors.As(err, &ax) {
		t.Fatalf("expected *axerror.Error, got %T: %v", err, err)
	}
	return ax
}

// TestEchoRoundTrip is the acceptance criterion: a fake echo worker round-trips
// a JSON request through real stdin/stdout and the invoker decodes its reply.
func TestEchoRoundTrip(t *testing.T) {
	req := Request{
		Op:     "embed",
		Params: map[string]any{"text": "hello", "dim": float64(384)},
	}
	resp, err := fakeWorker("echo").Invoke(context.Background(), req)
	if err != nil {
		t.Fatalf("Invoke: unexpected error: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("expected no worker error, got %+v", resp.Error)
	}
	// The invoker must stamp the contract version onto the outbound request,
	// and the echo worker reflects it back verbatim.
	if resp.Version != ContractVersion {
		t.Fatalf("response version = %d, want %d", resp.Version, ContractVersion)
	}
	if got := resp.Result["op"]; got != "embed" {
		t.Fatalf("echoed op = %v, want embed", got)
	}
	params, ok := resp.Result["params"].(map[string]any)
	if !ok {
		t.Fatalf("echoed params has wrong shape: %T", resp.Result["params"])
	}
	if params["text"] != "hello" || params["dim"] != float64(384) {
		t.Fatalf("echoed params = %v, want text=hello dim=384", params)
	}
}

// TestRequestVersionStamped proves the caller need not set Version: Invoke
// overwrites whatever was supplied with ContractVersion before sending.
func TestRequestVersionStamped(t *testing.T) {
	resp, err := fakeWorker("echo").Invoke(context.Background(), Request{Op: "noop", Version: 999})
	if err != nil {
		t.Fatalf("Invoke: unexpected error: %v", err)
	}
	if resp.Version != ContractVersion {
		t.Fatalf("echoed version = %d, want %d (Invoke must overwrite caller's Version)", resp.Version, ContractVersion)
	}
}

// TestWorkerReportedError maps a structured worker error to the matching axerror.
func TestWorkerReportedError(t *testing.T) {
	resp, err := fakeWorker("worker-error").Invoke(context.Background(), Request{Op: "transcribe"})
	ax := asAxError(t, err)
	if ax.Code != "transcribe.model_missing" {
		t.Fatalf("axerror code = %q, want transcribe.model_missing", ax.Code)
	}
	if ax.Hint != "download the model into the cache directory" {
		t.Fatalf("axerror hint = %q, not carried through", ax.Hint)
	}
	if !ax.Retryable {
		t.Fatal("axerror should be retryable, worker marked it so")
	}
	// The decoded Response is returned alongside the error for inspection.
	if resp == nil || resp.Error == nil || resp.Error.Code != "transcribe.model_missing" {
		t.Fatalf("expected decoded Response with the error attached, got %+v", resp)
	}
}

// TestMissingWorker maps a non-existent executable to worker.missing.
func TestMissingWorker(t *testing.T) {
	w := Worker{Path: "/nonexistent/cutvec-worker-does-not-exist"}
	_, err := w.Invoke(context.Background(), Request{Op: "noop"})
	ax := asAxError(t, err)
	if ax.Code != "worker.missing" {
		t.Fatalf("axerror code = %q, want worker.missing", ax.Code)
	}
	if ax.Retryable {
		t.Fatal("a missing worker is not retryable")
	}
}

// TestCrashedWorker maps a non-zero exit with no structured error to a
// retryable worker.crashed, surfacing the worker's stderr.
func TestCrashedWorker(t *testing.T) {
	_, err := fakeWorker("crash").Invoke(context.Background(), Request{Op: "noop"})
	ax := asAxError(t, err)
	if ax.Code != "worker.crashed" {
		t.Fatalf("axerror code = %q, want worker.crashed", ax.Code)
	}
	if !ax.Retryable {
		t.Fatal("a crashed worker is retryable")
	}
}

// TestCrashWithStructuredError proves a structured error on stdout is honoured
// even when the worker also exits non-zero.
func TestCrashWithStructuredError(t *testing.T) {
	_, err := fakeWorker("crash-with-error").Invoke(context.Background(), Request{Op: "reframe"})
	ax := asAxError(t, err)
	if ax.Code != "reframe.no_face" {
		t.Fatalf("axerror code = %q, want reframe.no_face (structured error should win over worker.crashed)", ax.Code)
	}
}

// TestProtocolErrors covers empty and malformed stdout.
func TestProtocolErrors(t *testing.T) {
	for _, mode := range []string{"silent", "not-json"} {
		_, err := fakeWorker(mode).Invoke(context.Background(), Request{Op: "noop"})
		ax := asAxError(t, err)
		if ax.Code != "worker.protocol" {
			t.Fatalf("mode %q: axerror code = %q, want worker.protocol", mode, ax.Code)
		}
	}
}

// TestVersionMismatch maps a response the core cannot speak to worker.version.
func TestVersionMismatch(t *testing.T) {
	resp, err := fakeWorker("bad-version").Invoke(context.Background(), Request{Op: "noop"})
	ax := asAxError(t, err)
	if ax.Code != "worker.version" {
		t.Fatalf("axerror code = %q, want worker.version", ax.Code)
	}
	// The mismatched Response is returned so a caller can inspect the version.
	if resp == nil || resp.Version != ContractVersion+1 {
		t.Fatalf("expected decoded Response carrying the foreign version, got %+v", resp)
	}
}

// TestContextCancellation maps a cancelled context to a retryable worker.timeout.
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before Invoke runs the process
	_, err := fakeWorker("echo").Invoke(ctx, Request{Op: "noop"})
	ax := asAxError(t, err)
	if ax.Code != "worker.timeout" {
		t.Fatalf("axerror code = %q, want worker.timeout", ax.Code)
	}
	if !ax.Retryable {
		t.Fatal("a timeout is retryable")
	}
}
