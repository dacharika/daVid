// Package worker defines the boundary to cutvec's Python ML workers. Each worker
// is a subprocess that speaks a versioned JSON contract over stdin/stdout — one
// Request in, one Response out — keeping the ML stack (whisper, mediapipe,
// embeddings, MLT tooling) out of the Go core and behind a testable interface.
package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/tahabsn/cutvec/internal/axerror"
)

// Request is the JSON document written to a worker's stdin.
type Request struct {
	// Version pins the contract shape so a worker can reject inputs it predates.
	Version int `json:"version"`
	// Op names the operation (e.g. "transcribe", "embed", "reframe").
	Op string `json:"op"`
	// Params carries op-specific arguments as decoded JSON.
	Params map[string]any `json:"params,omitempty"`
}

// Response is the JSON document read from a worker's stdout. Result is set on
// success; Error (the axerror shape) is set on failure. Exactly one is present.
type Response struct {
	Version int            `json:"version"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *ResponseError `json:"error,omitempty"`
}

// ResponseError mirrors the structured-error shape a worker reports on failure.
type ResponseError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Hint      string `json:"hint,omitempty"`
	Retryable bool   `json:"retryable"`
}

// ContractVersion is the JSON contract version the Go core currently speaks.
const ContractVersion = 1

// Worker is a handle to a Python worker executable. Path is the program to run
// (an interpreter or a script); Args are fixed leading arguments (typically the
// worker script's path). Env holds extra KEY=VALUE pairs appended to the
// process environment (e.g. PYTHONPATH or a model directory); when empty the
// worker inherits the parent environment unchanged.
type Worker struct {
	Path string
	Args []string
	Env  []string
}

// Invoke runs the worker as a subprocess, writes req as a single JSON document
// to its stdin, and decodes the JSON Response from its stdout. It stamps
// req.Version with ContractVersion so the contract shape always travels with
// the request. Every failure mode is returned as an *axerror.Error: a worker
// that cannot start (worker.missing), one that exits with an error
// (worker.crashed, retryable), output that is missing or not valid JSON
// (worker.protocol), a response whose version the core does not speak
// (worker.version), and a structured error the worker itself reports, which is
// mapped through to the matching axerror {code,hint,retryable}. On success the
// Response is returned with a nil error; on a mapped worker error the decoded
// Response is returned alongside the error so callers can inspect it.
func (w Worker) Invoke(ctx context.Context, req Request) (*Response, error) {
	req.Version = ContractVersion

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, axerror.New("worker.request", "cannot encode worker request: "+err.Error())
	}

	cmd := exec.CommandContext(ctx, w.Path, w.Args...)
	cmd.Stdin = bytes.NewReader(reqBytes)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if len(w.Env) > 0 {
		cmd.Env = append(os.Environ(), w.Env...)
	}

	if err := cmd.Run(); err != nil {
		// A cancelled or timed-out context is worth retrying with more headroom.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, axerror.New("worker.timeout", "worker did not finish: "+ctxErr.Error()).
				WithHint("increase the timeout or check why the worker is slow").AsRetryable()
		}
		// The binary could not be started at all: not found on PATH
		// (*exec.Error), an absolute path that does not exist or is not
		// executable (a *fs.PathError satisfying os.ErrNotExist /
		// os.ErrPermission). None of these ran, so retrying is pointless until
		// the worker is installed.
		var execErr *exec.Error
		if errors.As(err, &execErr) || errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			return nil, axerror.New("worker.missing", "cannot start worker "+w.Path+": "+err.Error()).
				WithHint("install the Python workers and ensure the interpreter is on PATH")
		}
		// Non-zero exit: honour a structured error printed to stdout if there is
		// one, otherwise surface the crash with whatever the worker logged.
		if resp, perr := decodeResponse(stdout.Bytes()); perr == nil && resp.Error != nil {
			return resp, resp.Error.toAxError()
		}
		detail := stderr.String()
		if detail == "" {
			detail = err.Error()
		}
		return nil, axerror.New("worker.crashed", "worker exited with an error: "+detail).
			WithHint("check the worker's stderr for a traceback").AsRetryable()
	}

	resp, err := decodeResponse(stdout.Bytes())
	if err != nil {
		return nil, err
	}
	if resp.Version != ContractVersion {
		return resp, axerror.New("worker.version",
			fmt.Sprintf("worker speaks protocol version %d, core speaks %d", resp.Version, ContractVersion)).
			WithHint("upgrade the worker or the cutvec core so both speak the same contract version")
	}
	if resp.Error != nil {
		return resp, resp.Error.toAxError()
	}
	return resp, nil
}

// decodeResponse parses the worker's stdout into a Response, mapping an empty or
// malformed stream to a worker.protocol axerror.
func decodeResponse(b []byte) (*Response, error) {
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, axerror.New("worker.protocol", "worker produced no response on stdout").
			WithHint("the worker must print exactly one JSON Response to stdout").AsRetryable()
	}
	var resp Response
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, axerror.New("worker.protocol", "worker response is not valid JSON: "+err.Error()).
			WithHint("the worker must print exactly one JSON object to stdout")
	}
	return &resp, nil
}

// toAxError lifts the worker-reported error shape into the core's axerror,
// preserving its code, message, hint and retryable flag.
func (e *ResponseError) toAxError() *axerror.Error {
	ax := axerror.New(e.Code, e.Message)
	if e.Hint != "" {
		ax = ax.WithHint(e.Hint)
	}
	if e.Retryable {
		ax = ax.AsRetryable()
	}
	return ax
}
