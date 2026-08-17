package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The Go core's every failure path assumes a worker answers with one JSON
// Response — even when it cannot do the work. A worker that dies with a
// traceback instead produces a worker.protocol error at some unpredictable
// point in a pipeline, so that promise is worth checking against the real
// scripts rather than a fake.
//
// These tests deliberately invoke the workers with nothing configured: no ML
// packages guaranteed, no models, no media. Every one must still answer.

func workersDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("cannot locate the workers directory")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "workers")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("workers directory not present: %v", err)
	}
	return dir
}

func python(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	t.Skip("no python interpreter on PATH")
	return ""
}

// runWorker feeds a request to a worker script and returns its raw stdout.
func runWorker(t *testing.T, script, request string) []byte {
	t.Helper()
	cmd := exec.Command(python(t), filepath.Join(workersDir(t), script))
	cmd.Stdin = strings.NewReader(request)
	out, _ := cmd.Output() // a non-zero exit is allowed; silence is not
	return out
}

func TestWorkerContractAlwaysAnswersWithAJSONResponse(t *testing.T) {
	cases := []struct {
		script  string
		request string
	}{
		// ffmpeg_worker is the one every ingest goes through, and all three of its
		// ops must fail structurally rather than tracebacking on a missing file.
		{"ffmpeg_worker.py", `{"version":1,"op":"probe","params":{"source":"/nonexistent.mp4"}}`},
		{"ffmpeg_worker.py", `{"version":1,"op":"proxy","params":{"source":"/nonexistent.mp4","dest":"/tmp/p.mp4"}}`},
		{"ffmpeg_worker.py", `{"version":1,"op":"audio","params":{"source":"/nonexistent.mp4","dest":"/tmp/a.wav"}}`},
		{"whisper_worker.py", `{"version":1,"op":"transcribe","params":{"source":"/nonexistent.mp4"}}`},
		{"mediapipe_worker.py", `{"version":1,"op":"speakers","params":{"source":"/nonexistent.mp4"}}`},
		{"index_worker.py", `{"version":1,"op":"index","params":{"transcript":"/nonexistent.json","table":"/tmp/x.lance"}}`},
		{"search_worker.py", `{"version":1,"op":"search","params":{"query":"x","table":"/nonexistent.lance"}}`},
		{"melt_worker.py", `{"version":1,"op":"render","params":{"timeline":"/nonexistent.mlt","output":"/tmp/o.mp4"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.script, func(t *testing.T) {
			out := runWorker(t, tc.script, tc.request)
			if len(out) == 0 {
				t.Fatal("worker produced no response at all; the core would see worker.protocol")
			}
			var resp struct {
				Version int `json:"version"`
				Error   *struct {
					Code      string `json:"code"`
					Message   string `json:"message"`
					Hint      string `json:"hint"`
					Retryable bool   `json:"retryable"`
				} `json:"error"`
				Result map[string]any `json:"result"`
			}
			if err := json.Unmarshal(out, &resp); err != nil {
				t.Fatalf("response is not valid JSON: %v\n%s", err, out)
			}
			if resp.Version != 1 {
				t.Errorf("version = %d, want 1 (the contract the core speaks)", resp.Version)
			}
			// These requests cannot succeed, so a structured error is the
			// correct answer — and it has to carry something actionable.
			if resp.Error == nil {
				t.Fatalf("worker reported success for an impossible request: %s", out)
			}
			if resp.Error.Code == "" || resp.Error.Message == "" {
				t.Errorf("error is not structured: %+v", *resp.Error)
			}
			if resp.Error.Hint == "" {
				t.Errorf("error %q carries no hint, so a caller cannot act on it", resp.Error.Code)
			}
		})
	}
}

// A worker asked for an op it does not implement must say so, naming what it
// does implement, rather than silently doing the wrong work.
func TestWorkerContractRejectsAnUnknownOp(t *testing.T) {
	out := runWorker(t, "melt_worker.py", `{"version":1,"op":"transcribe","params":{}}`)
	var resp struct {
		Error *struct {
			Code string `json:"code"`
			Hint string `json:"hint"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, out)
	}
	if resp.Error == nil || resp.Error.Code != "input.invalid" {
		t.Fatalf("unknown op was not rejected: %s", out)
	}
}

// A version the worker does not speak must be refused rather than guessed at:
// silently accepting a future contract is how a shape change becomes corrupt
// output instead of an error.
func TestWorkerContractRefusesAnUnknownProtocolVersion(t *testing.T) {
	out := runWorker(t, "search_worker.py", `{"version":99,"op":"search","params":{}}`)
	var resp struct {
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, out)
	}
	if resp.Error == nil || resp.Error.Code != "worker.version" {
		t.Fatalf("a future protocol version was not refused: %s", out)
	}
}
