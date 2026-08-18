// Package axerror defines daVid's structured error type. Every failure an
// agent can observe is reported as an Error carrying a stable machine code, a
// remediation hint, and whether retrying could plausibly succeed. This is the
// error half of the AX contract: an agent branches on Code, never on prose.
package axerror

import "fmt"

// Error is a structured, agent-legible failure. It marshals to the JSON shape
// {"code","hint","retryable","message"} used on every CLI and MCP surface.
type Error struct {
	// Code is a stable, machine-branchable identifier (e.g. "worker.missing").
	Code string `json:"code"`
	// Message is a human-readable one-line description of what went wrong.
	Message string `json:"message"`
	// Hint tells the caller what to do about it; empty when there is nothing useful to say.
	Hint string `json:"hint,omitempty"`
	// Retryable is true when the same call could plausibly succeed on retry.
	Retryable bool `json:"retryable"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Hint)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// New builds a non-retryable Error.
func New(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WithHint returns a copy of e carrying the given remediation hint.
func (e *Error) WithHint(hint string) *Error {
	c := *e
	c.Hint = hint
	return &c
}

// AsRetryable returns a copy of e marked retryable.
func (e *Error) AsRetryable() *Error {
	c := *e
	c.Retryable = true
	return &c
}
