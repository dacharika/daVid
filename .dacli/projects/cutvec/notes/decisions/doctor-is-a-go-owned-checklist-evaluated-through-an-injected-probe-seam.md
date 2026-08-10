---
id: d-doctor-is-a-go-owned-checklist-evaluated-through-an-injected-probe-seam
kind: note
note_kind: decision
created: 2026-08-10T17:29:25Z
created_by: a-fixer-xrt529
about: "[[024]]"
---
# doctor is a Go-owned checklist evaluated through an injected Probe seam (LookPath/FileExists/Getenv/Home/HasModule), python packages tested via find_spec
## Chose
doctor is a Go-owned checklist evaluated through an injected Probe seam (LookPath/FileExists/Getenv/Home/HasModule), python packages tested via find_spec
## Rejected
Add a doctor/check op to each Python worker and invoke every worker subprocess to self-report
## Because
The authoritative list of what the pipeline needs (6 workers, 5 binaries, 5 packages, 2 models) belongs in the Go core next to the tool catalog and is fully unit-testable with a fake Probe (no ffmpeg/melt/python/model on the test host); a per-worker doctor op would require python to even run doctor and couple the report to worker startup. HasModule shells python3 -c importlib.util.find_spec so heavy packages (mediapipe) are located, not imported. doctor runs no worker and touches no project (never starts real work); it emits the full report to stdout always and returns doctor.preflight_failed (exit 1) only when a REQUIRED check is missing.
