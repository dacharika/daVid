---
id: d-test-the-invoker-via-a-re-exec-d-helper-process-fake-worker-os-exec
kind: note
note_kind: decision
created: 2026-08-05T21:38:45Z
created_by: a-fixer-4qganh
about: "[[009]]"
---
# Test the invoker via a re-exec'd helper-process fake worker (os/exec TestHelperProcess pattern), not a real Python script
## Chose
Test the invoker via a re-exec'd helper-process fake worker (os/exec TestHelperProcess pattern), not a real Python script
## Rejected
Shipping a real Python echo script and requiring python3 on PATH in the go test
## Because
The acceptance criterion is a fake echo worker round-tripping through real stdin/stdout in a go test; the stdlib helper-process pattern re-execs the test binary as the subprocess, exercising exec+stdin+stdout+exit-code paths for real with zero external toolchain, so go test ./... stays green in CI where python may be absent (workers are out of scope until task 010).
