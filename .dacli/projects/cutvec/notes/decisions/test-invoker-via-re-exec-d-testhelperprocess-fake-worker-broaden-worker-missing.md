---
id: d-test-invoker-via-re-exec-d-testhelperprocess-fake-worker-broaden-worker-missing
kind: note
note_kind: decision
created: 2026-08-05T23:11:07Z
created_by: a-fixer-978dgv
about: "[[009]]"
---
# Test invoker via re-exec'd TestHelperProcess fake worker; broaden worker.missing detection to absolute-path ENOENT/EPERM
## Chose
Test invoker via re-exec'd TestHelperProcess fake worker; broaden worker.missing detection to absolute-path ENOENT/EPERM
## Rejected
Ship a real python echo script requiring python3 on PATH in go test; leave worker.missing keyed only on *exec.Error
## Because
The stdlib helper-process pattern re-execs the test binary as the subprocess, exercising real exec+stdin+stdout+exit-code with zero external toolchain so go test ./... stays green in CI where python is absent. Go only wraps *exec.Error for PATH lookups, so an absolute missing path returned worker.crashed (retryable); adding errors.Is(os.ErrNotExist/os.ErrPermission) maps a genuinely-missing worker to the non-retryable worker.missing it deserves.
