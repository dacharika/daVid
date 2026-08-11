---
id: f-clock-now-rounds-to-whole-ms-before-decomposing-boundary-times-carry-into-the
kind: note
note_kind: finding
created: 2026-08-11T08:57:48Z
created_by: a-fixer-dxscgf
about: "[[036]]"
severity: major
---
# Clock now rounds to whole ms before decomposing; boundary times carry into the next minute/hour instead of emitting :60.000
Fix at internal/timeline/timeline.go:294 Clock(): replaced the truncating 'int(seconds)' decomposition (which let %06.3f round the leftover seconds up to an invalid 60.000) with ms:=int64(seconds*1000+0.5) then h/m/s from integer ms and %02d.%03d formatting. New table-driven TestClockRoundsAndCarries (internal/timeline/timeline_test.go) asserts Clock(59.9996)=='00:01:00.000', Clock(119.9999)=='00:02:00.000', Clock(3599.9999)=='01:00:00.000', plus the existing 0/12.5/3671.25 and negative-clamp cases. VERIFIED: the test FAILS on old code (Clock(59.9996)=='00:00:60.000', Clock(119.9999)=='00:01:60.000', Clock(3599.9999)=='00:59:60.000') and PASSES after. go build ./... and go test ./... both exit 0; gofmt clean. All 3 acceptance criteria met (owner must check boxes 1-3). Committed 0a5b1e3.
