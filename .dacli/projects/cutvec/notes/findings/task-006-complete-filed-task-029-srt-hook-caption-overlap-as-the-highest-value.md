---
id: f-task-006-complete-filed-task-029-srt-hook-caption-overlap-as-the-highest-value
kind: note
note_kind: finding
created: 2026-08-10T18:09:07Z
created_by: a-go-auditor-zd1s8t
about: "[[006]]"
severity: minor
---
# Task 006 complete: filed task 029 (SRT hook/caption overlap) as the highest-value evidence-based change; implemented nothing
AC1 satisfied: filed new task 029-shorts-stop-the-hook-cue-from-overlapping-the-first-caption-cue-in-buildsrt (t-01KZPDSSRZMHR2CJNA1DSP6CNV), grounded in an observed defect — buildSRT (cmd/cutvec/shorts.go:510-526) emits a fixed [0,2.5s] hook cue that overlaps the first independently-timed caption cue whenever the first word starts before 2.5s; the existing TestBuildSRTLeadsWithTheHook constructs this exact overlap yet never checks it. Backlog checked first (open: 006/026/028; active: none) — distinct from 026 (timeline edit) and 028 (CropRect removal), and deliberately NOT the contested reframe-edit PunchIn no-op (contradicts the VERIFIED d-keep-punchin A/B). AC2 satisfied: no source changed in this task — only dacli notes and the new task were written; go build ./... and go test ./... remain green (verified at start). No git branch/commit was needed since filing is via dacli, not a code change.
