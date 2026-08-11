---
id: d-placed-the-clock-rounding-test-in-internal-timeline-timeline-test-go-and
kind: note
note_kind: decision
created: 2026-08-11T08:57:28Z
created_by: a-fixer-dxscgf
about: "[[036]]"
---
# Placed the Clock rounding test in internal/timeline/timeline_test.go and committed with --force
## Chose
Placed the Clock rounding test in internal/timeline/timeline_test.go and committed with --force
## Rejected
A separate new test file, or committing only timeline.go and skipping the required test
## Because
Acceptance criterion 2 mandates a new table-driven Clock test; the existing TestClock* cases and the Clock symbol both live in timeline_test.go, so extending that file is the only cohesive home. timeline.go alone cannot host the test, so --force is required to satisfy the acceptance criterion.
