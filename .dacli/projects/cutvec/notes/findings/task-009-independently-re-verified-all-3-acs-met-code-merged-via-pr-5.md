---
id: f-task-009-independently-re-verified-all-3-acs-met-code-merged-via-pr-5
kind: note
note_kind: finding
created: 2026-08-05T23:13:40Z
created_by: a-fixer-1prtkf
about: "[[009]]"
severity: major
---
# Task 009 independently re-verified: all 3 ACs met, code merged via PR #5
Re-verified on branch dacli/009 at commit fec74c1 (merged to main via PR #5, merge commit e3e7b44). AC1 (invoke python worker, JSON stdin/JSON stdout): internal/worker/worker.go Worker.Invoke marshals Request to subprocess stdin (worker.go:79) and decodes Response from stdout (worker.go:116). AC2 (version field + errors->axerror {code,hint,retryable}): Request.Version/Response.Version (worker.go:22,32) stamped with ContractVersion=1 every request (worker.go:71); every failure returns *axerror.Error (worker.missing worker.go:100, worker.timeout :90, worker.crashed :112, worker.protocol :135/:140, worker.version :121) and a worker-reported ResponseError maps through toAxError preserving code+hint+retryable (worker.go:148). AC3 (fake echo worker round-trips in a go test): internal/worker/worker_test.go re-execs the test binary as the worker via os/exec TestHelperProcess pattern (no python needed); TestEchoRoundTrip PASSES. All 10 worker tests pass (TestEchoRoundTrip, TestRequestVersionStamped, TestWorkerReportedError, TestMissingWorker, TestCrashedWorker, TestCrashWithStructuredError, TestProtocolErrors, TestVersionMismatch, TestContextCancellation, TestHelperProcess). Verified: go build ./..., go vet ./..., gofmt -l internal/worker (clean), go test ./... all exit 0. Owner may check acceptance boxes 1-3 and ship.
