---
id: d-filed-the-clock-minute-boundary-rounding-defect-invalid-60-000-seconds-field-as
kind: note
note_kind: decision
created: 2026-08-10T21:06:32Z
created_by: a-go-auditor-t6n382
about: "[[006]]"
---
# Filed the Clock() minute-boundary rounding defect (invalid :60.000 seconds field) as the single highest-value evidence-based change
## Chose
Filed the Clock() minute-boundary rounding defect (invalid :60.000 seconds field) as the single highest-value evidence-based change
## Rejected
Re-filing any of the seven sibling findings already queued as open tasks 029 (hook overlap), 030 (audio gain), 031 (multi-clip sequencing), 032 (blank element), 033 (render atomic publish), 034 (shorts rank), 035 (assets rank) — all near-duplicates dacli refuses; or the speakerAt/applyAudio map-iteration tie-break nit (rejected repeatedly: only fires on an exact float tie, rare and hard to pin deterministically)
## Because
All seven obvious determinism/audio/render findings are now open tasks 029-035, so re-filing any is a near-duplicate. The Clock defect is DISTINCT from every open and done task — it is the core time formatter, not any surface those cover — and PROVEN from source with no melt render: internal/timeline/timeline.go:194-201 truncates h/m from the original float but formats fractional seconds with %06.3f (line 201) which ROUNDS, so when the fraction rounds up to 60.000 no carry propagates. I ran the exact function: Clock(59.9996)=00:00:60.000, Clock(119.9999)=00:01:60.000, Clock(3599.9999)=00:59:60.000 — all out-of-range seconds fields MLT misparses. It hits the deterministic-idempotent core on the single most foundational primitive: Clock formats EVERY MLT timestamp cutvec writes (clip in/out reframe.go:151-152 + shorts.go:300-301, audio offset/gap timeline.go:115, fades timeline.go:104,108), and the values come from whisper word timings and snapped sentence boundaries (arbitrary floats), so a real edit can land in the [x.9995,x+1) window just below a whole minute. It is unit-testable in pure Go with no worker/render, and TestClockFormatsMLTTime (shorts_test.go:185) only covers 0/12.5/3671.25 while TestClockRejectsNegativeTime covers the negative clamp, so it ships green.
