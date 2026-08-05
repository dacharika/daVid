---
id: f-009-re-verified-green-all-3-acs-met-against-real-code-code-already-on-main-via
kind: note
note_kind: finding
created: 2026-08-05T23:19:18Z
created_by: a-fixer-yx2zzj
about: "[[009]]"
severity: major
---
# 009 re-verified green: all 3 ACs met against real code, code already on main via PR #5
Independently re-verified on branch dacli/009. git diff origin/main -- cmd/ internal/ is EMPTY (code landed on main via merged PR #5, merge e3e7b44). AC1 (JSON stdin/stdout): internal/worker/worker.go:78-81 exec.CommandContext + cmd.Stdin=bytes.NewReader(reqBytes), stdout captured; worker.go:116 decodeResponse(stdout.Bytes()) into Response. AC2 (version + axerror {code,hint,retryable}): Request.Version worker.go:22, Response.Version :32, ContractVersion=1 stamped every request :71; failures return *axerror.Error (worker.missing :100, worker.timeout :90, worker.crashed :112, worker.protocol :135/140, worker.version :121); ResponseError.toAxError :148 preserves code+hint+retryable. AC3 (fake echo round-trip): worker_test.go re-execs test binary via TestHelperProcess pattern (no python). Ran UNCACHED 'go test -count=1 ./internal/worker ./cmd/cutvec': both PASS. 'go build ./...' exit 0; 'go test ./...' all packages ok. No further code work needed.
