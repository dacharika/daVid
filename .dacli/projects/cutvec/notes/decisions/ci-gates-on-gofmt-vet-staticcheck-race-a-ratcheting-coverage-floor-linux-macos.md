---
id: d-ci-gates-on-gofmt-vet-staticcheck-race-a-ratcheting-coverage-floor-linux-macos
kind: note
note_kind: decision
created: 2026-08-10T15:05:54Z
created_by: a-root
---
# CI gates on gofmt, vet, staticcheck, -race, a ratcheting coverage floor, Linux+macOS, plus python lint and a worker-protocol contract job
## Chose
CI gates on gofmt, vet, staticcheck, -race, a ratcheting coverage floor, Linux+macOS, plus python lint and a worker-protocol contract job
## Rejected
build+test only
## Because
the contract job checks a promise no Go test can reach, and the lint gate immediately found three silent-truncation zip() bugs.
