---
id: f-009-firsthand-verified-green-all-3-acs-pass-code-merged-to-main-via-pr-5
kind: note
note_kind: finding
created: 2026-08-05T23:23:35Z
created_by: a-fixer-y12v6s
about: "[[009]]"
severity: major
---
# 009 firsthand-verified green: all 3 ACs pass, code merged to main via PR #5
Branch dacli/009 in isolated worktree; 'git diff origin/main -- cmd/ internal/' is EMPTY (landed via merged PR #5). Ran UNCACHED 'go build ./...' (exit 0, BUILD_OK) and 'go test -count=1 ./...' — ALL packages ok (cmd/cutvec, axerror, catalog, project, worker; timeline no test files). AC1 (JSON stdin/stdout invoker): internal/worker/worker.go:78-79 exec.CommandContext + cmd.Stdin=bytes.NewReader(reqBytes), stdout captured; worker.go:116 decodeResponse(stdout.Bytes()). AC2 (version + axerror {code,hint,retryable}): Request.Version worker.go:22, Response.Version:32, ContractVersion=1 stamped every request:71; failures return *axerror.Error (worker.missing:100, worker.timeout:90, worker.crashed:112, worker.protocol:135/140, worker.version:121); ResponseError.toAxError:148 preserves code+hint+retryable. AC3 (fake echo round-trip): worker_test.go:115 TestEchoRoundTrip via TestHelperProcess:31 re-exec pattern (no python on PATH). No further code work or PR needed.
