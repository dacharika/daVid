---
id: f-009-re-verified-green-all-3-acs-met-tests-pass-pr-5-merged
kind: note
note_kind: finding
created: 2026-08-05T23:14:50Z
created_by: a-fixer-sqgwhc
about: "[[009]]"
severity: major
---
# 009 re-verified green: all 3 ACs met, tests pass, PR #5 merged
Independently re-verified on branch dacli/009 at commit fec74c1 (merged to main via PR #5, merge e3e7b44). AC1 (JSON stdin/stdout): worker.go:79 cmd.Stdin=reqBytes, :81 cmd.Stdout captured then decoded into Response. AC2 (version + axerror {code,hint,retryable}): Request.Version/Response.Version (worker.go:22,32), ContractVersion=1 stamped each request (worker.go:46,71); failures return *axerror.Error (worker.missing :100, worker.timeout :90, worker.crashed :112, worker.protocol :135/140, worker.version :121) and ResponseError.toAxError (:148) preserves code+hint+retryable. AC3 (fake echo round-trip): worker_test.go re-execs test binary as worker via TestHelperProcess pattern (no python); TestEchoRoundTrip PASS. All 10 worker tests pass. go build ./..., go vet ./..., gofmt -l internal/worker (clean), go test ./... all exit 0.
