---
id: f-timeline-clock-emits-malformed-hh-mm-60-000-timecodes-at-minute-boundaries
kind: note
note_kind: finding
created: 2026-08-10T18:23:46Z
created_by: a-go-auditor-tf6j64
about: "[[006]]"
severity: moderate
---
# timeline.Clock emits malformed HH:MM:60.000 timecodes at minute boundaries
internal/timeline/timeline.go:194-202 Clock formats the seconds field with fmt.Sprintf("%02d:%02d:%06.3f", h, m, s) where s = seconds - h*3600 - m*60. %06.3f ROUNDS to 3 decimals, so any time within ~0.0005s below a whole-minute boundary rounds the seconds field up to 60.000 instead of carrying into the minute. Empirically verified with the exact function: Clock(59.9997)="00:00:60.000", Clock(119.9996)="00:01:60.000", Clock(3599.9998)="00:59:60.000" — all malformed MLT clocks (seconds must be <60). Clock(60.0)="00:01:00.000" correctly carries, so the bug is specifically the sub-boundary rounding band. srtTime (shorts.go:587-596) is NOT affected because it derives seconds via integer truncation (int(seconds)%60) and ms separately. Reachable: Clock produces every producer In/Out (reframe.go:151-152, shorts.go:300-301) and every blank-entry length (timeline.go:104,108,115) in the MLT XML the render worker feeds melt. shorts feeds RAW unrounded start/end (padding-adjusted floats) straight into Clock, so a clip boundary landing near a whole minute emits an invalid timecode into the product artifact. Latent+green: TestClockFormatsMLTTime (shorts_test.go:185-198) only checks 0, 12.5, 3671.25 — none near a minute boundary — so the defect passes CI. Fully unit-testable in Go without a melt render: assert Clock's seconds field parses to <60 across a sweep including x.9997-style values, and that the minute carries. Distinct from open tasks 006 and 029 (029 is the SRT hook/caption cue overlap in buildSRT; this is the MLT clock formatter). Fix: carry the rounding into the minute/hour (e.g. round to whole milliseconds first, then decompose h/m/s from the integer-ms), matching srtTime's truncation-safe decomposition.
