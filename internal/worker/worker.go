// Package worker defines the boundary to cutvec's Python ML workers. Each worker
// is a subprocess that speaks a versioned JSON contract over stdin/stdout — one
// Request in, one Response out — keeping the ML stack (whisper, mediapipe,
// embeddings, MLT tooling) out of the Go core and behind a testable interface.
package worker

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
