---
id: f-009-re-verified-green-all-3-acs-pass-code-merged-to-main-via-pr-5
kind: note
note_kind: finding
created: 2026-08-05T23:22:12Z
created_by: a-fixer-n3a5sw
about: "[[009]]"
severity: major
---
# 009 re-verified green: all 3 ACs pass, code merged to main via PR #5
Independently re-verified in isolated worktree on branch dacli/009. 'git diff origin/main -- cmd/ internal/' is EMPTY (code landed on main via merged PR #5, merge e3e7b44). Ran UNCACHED 'go build ./...' (exit 0) and 'go test -count=1 ./...' — ALL packages ok (cmd/cutvec, axerror, catalog, project, worker; timeline no test files). AC1 (JSON stdin/stdout invoker): internal/worker/worker.go:78-81 exec.CommandContext + cmd.Stdin=bytes.NewReader(reqBytes), stdout captured; worker.go:116 decodeResponse(stdout.Bytes()) into Response. AC2 (version + axerror {code,hint,retryable}): Request.Version worker.go:22, Response.Version:32, ContractVersion=1 stamped every request :71; failures return *axerror.Error (worker.missing:100, worker.timeout:90, worker.crashed:112, worker.protocol:135/140, worker.version:121); ResponseError.toAxError:148 preserves code+hint+retryable. AC3 (fake echo round-trip): worker_test.go:115 TestEchoRoundTrip re-execs test binary via TestHelperProcess:31 pattern (no python on PATH). All pass. No further code work or PR needed.
