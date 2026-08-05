---
id: f-009-complete-all-3-acs-pass-uncached-code-merged-to-main-via-pr-5
kind: note
note_kind: finding
created: 2026-08-05T23:20:37Z
created_by: a-fixer-fn7bzg
about: "[[009]]"
severity: major
---
# 009 complete: all 3 ACs pass uncached, code merged to main via PR #5
Independently re-verified on branch dacli/009 in isolated worktree. 'git diff origin/main -- cmd/ internal/' is EMPTY (code landed on main via merged PR #5). Ran UNCACHED 'go build ./...' (exit 0) and 'go test -count=1 ./...' — ALL packages ok (cmd/cutvec, axerror, catalog, project, worker).
AC1 (JSON stdin/stdout invoker): internal/worker/worker.go:78-81 exec.CommandContext + cmd.Stdin=bytes.NewReader(reqBytes), stdout/stderr captured; worker.go:116 decodeResponse(stdout.Bytes()) into Response.
AC2 (version field + axerror {code,hint,retryable}): Request.Version worker.go:22, Response.Version:32, ContractVersion=1 stamped every request:71; failures all return *axerror.Error (worker.missing:100, worker.timeout:90, worker.crashed:112, worker.protocol:135/140, worker.version:121); ResponseError.toAxError:148 preserves code+hint+retryable. Pinned by TestRequestVersionStamped:146, TestWorkerReportedError:157, TestVersionMismatch:223, TestProtocolErrors:212.
AC3 (fake echo round-trip): worker_test.go TestEchoRoundTrip:115 re-execs the test binary via TestHelperProcess:31 pattern (no python on PATH needed). PASS.
No further code work, commit, or PR needed. Owner (a-root) may check boxes 1-3 and mark 009 done.
