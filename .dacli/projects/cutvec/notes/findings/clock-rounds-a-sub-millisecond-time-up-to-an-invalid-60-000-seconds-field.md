---
id: f-clock-rounds-a-sub-millisecond-time-up-to-an-invalid-60-000-seconds-field
kind: note
note_kind: finding
created: 2026-08-10T21:06:11Z
created_by: a-go-auditor-t6n382
about: "[[006]]"
severity: major
---
# Clock() rounds a sub-millisecond time up to an invalid :60.000 seconds field instead of carrying into the next minute
internal/timeline/timeline.go:194-201: Clock decomposes h/m from the ORIGINAL float by truncation (h:=int(seconds)/3600, m:=(int(seconds)%3600)/60) but formats the fractional seconds with %06.3f (line 201), which ROUNDS to 3 decimals. When the fractional part rounds up to 60.000, no carry propagates into minutes/hours. VERIFIED by running the exact function: Clock(59.9996)="00:00:60.000", Clock(119.9999)="00:01:60.000", Clock(3599.9999)="00:59:60.000" — every one an out-of-range seconds field (MLT mlt_time expects seconds<60). The failing window is [x.9995, x+1) just below any whole minute. IMPACT: Clock formats EVERY timestamp cutvec writes to the MLT doc — clip in/out points (cmd/cutvec/reframe.go:151-152, cmd/cutvec/shorts.go:300-301), audio start offset / playlist gap length (timeline.go:115), and fade in/out durations (timeline.go:104,108). These values come from snapped sentence boundaries and whisper word timings (arbitrary floats), so a real edit can land in the window and emit a malformed clock melt misparses. NO render needed to prove it — it is pure string formatting. LATENT/GREEN: TestClockFormatsMLTTime (cmd/cutvec/shorts_test.go:185-198) covers only 0/12.5/3671.25 and TestClockRejectsNegativeTime (internal/timeline/timeline_test.go:57-62) covers the negative clamp; neither asserts a near-minute-boundary value, so the bug ships green. FIX: round to whole milliseconds BEFORE decomposing, e.g. ms:=int64(math.Round(seconds*1000)); h:=ms/3600000; m:=(ms/60000)%60; s:=(ms%60000) then format, so the carry propagates. DISTINCT from every open task (029 hook overlap, 030 audio gain, 031 sequencing, 032 blank element, 033 render atomic, 034 shorts rank, 035 assets rank) and every done task — this is the core time formatter, not any of those surfaces.
