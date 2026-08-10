---
id: d-progress-is-a-go-owned-side-channel-an-internal-progress-reporter-emits-jsonl
kind: note
note_kind: decision
created: 2026-08-10T17:42:25Z
created_by: a-fixer-3z558s
about: "[[025]]"
---
# progress is a Go-owned side channel: an internal/progress Reporter emits JSONL start/heartbeat/done events to stderr around each long worker Invoke; run() gains an optional variadic stderr so all 41 existing 3-arg run() test callers keep compiling
## Chose
progress is a Go-owned side channel: an internal/progress Reporter emits JSONL start/heartbeat/done events to stderr around each long worker Invoke; run() gains an optional variadic stderr so all 41 existing 3-arg run() test callers keep compiling
## Rejected
have each Python worker print progress to its own stderr and forward it; or change run() to a mandatory 4th stderr param
## Because
The three long steps (render/transcribe/speakers) already funnel through w.Invoke in Go; wrapping that call with a ticker-driven heartbeat gives real slow-vs-hung signal without touching any worker or its crash-stderr buffering, and is fully unit-testable with a bytes.Buffer. A variadic optional stderr keeps the honest stdin/stdout/stderr seam while avoiding churn/risk across 41 test call sites in a headless run; stdout still carries exactly one JSON result.
