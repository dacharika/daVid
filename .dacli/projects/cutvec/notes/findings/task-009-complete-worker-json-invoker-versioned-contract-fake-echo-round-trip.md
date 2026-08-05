---
id: f-task-009-complete-worker-json-invoker-versioned-contract-fake-echo-round-trip
kind: note
note_kind: finding
created: 2026-08-05T23:11:32Z
created_by: a-fixer-978dgv
about: "[[009]]"
severity: major
---
# Task 009 complete: worker JSON invoker + versioned contract + fake-echo round-trip test, all 3 ACs met
AC1 (invoke python worker: JSON on stdin, JSON on stdout): internal/worker/worker.go Worker.Invoke marshals a Request to the subprocess stdin (worker.go:79) and decodes a Response from stdout (worker.go:112). AC2 (version field + errors->axerror {code,hint,retryable}): Request.Version/Response.Version fields (worker.go:22,32) stamped with ContractVersion=1 on every request (worker.go:71); every failure returns *axerror.Error — worker.missing/timeout/crashed/protocol/version — and a worker-reported ResponseError maps through toAxError preserving code+hint+retryable (worker.go:144). AC3 (fake echo worker round-trips in a go test): internal/worker/worker_test.go re-execs the test binary as the worker via the os/exec TestHelperProcess pattern (no python needed); TestEchoRoundTrip round-trips op+params and asserts the reflected result. Companion tests cover version stamping, worker-reported error mapping, worker.missing (absolute path), worker.crashed, structured-error-wins-over-crash, protocol (empty/malformed), version mismatch, and context-cancellation->worker.timeout. Verified: go build ./... , go vet ./... , gofmt -l (clean), go test ./... all exit 0. Commit fec74c1.
