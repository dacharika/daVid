---
id: d-placed-the-assets-search-rank-unit-test-in-main-test-go-extending
kind: note
note_kind: decision
created: 2026-08-10T22:21:06Z
created_by: a-fixer-dkm65b
about: "[[035]]"
---
# Placed the assets-search rank unit test in main_test.go, extending TestHelperProcess with a search-assets fake-worker mode
## Chose
Placed the assets-search rank unit test in main_test.go, extending TestHelperProcess with a search-assets fake-worker mode
## Rejected
A self-contained fake worker in assets_test.go with its own helper-process re-exec
## Because
The shared subprocess harness (TestHelperProcess, indexWorkerEnv, runJSON, run) already lives in main_test.go and every worker fixture routes through it; a second parallel helper-process entry point would duplicate the re-exec plumbing. Committed main_test.go with --force since the acceptance criterion mandates a new test and assets.go alone cannot host a subprocess fixture.
