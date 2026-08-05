---
id: f-009-re-verified-green-all-3-acs-met-against-real-code-pr-5-merged-1d0zwd
kind: note
note_kind: finding
created: 2026-08-05T23:18:11Z
created_by: a-fixer-bd7bmd
about: "[[009]]"
severity: major
---
# 009 re-verified green: all 3 ACs met against real code, PR #5 merged
Independently re-verified on branch dacli/009 at commit fec74c1 (merged to main via PR #5, merge e3e7b44). AC1 (JSON stdin/stdout): internal/worker/worker.go:78-81 cmd.Stdin=bytes.NewReader(reqBytes), stdout captured, worker.go:116 decodeResponse(stdout.Bytes()) into Response. AC2 (version + axerror {code,hint,retryable}): Request.Version worker.go:22, Response.Version :32, ContractVersion=1 stamped every request :71; failures return *axerror.Error (worker.missing :100, worker.timeout :90, worker.crashed :112, worker.protocol :135/140, worker.version :121) and ResponseError.toAxError :148 preserves code+hint+retryable. AC3 (fake echo round-trip): worker_test.go re-execs test binary via TestHelperProcess pattern (no python); TestEchoRoundTrip PASS. Ran uncached 'go test -count=1 ./internal/worker': all 10 tests PASS. 'go build ./...' and 'go test ./...' both exit 0 (cmd/cutvec, axerror, catalog, project, worker all ok). Owner (a-root) can check boxes 1-3 and mark done; no further code work or PR needed.
