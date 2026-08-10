---
id: t-01KZPSKV2F7V17ZTNWATEHZ1K9
kind: task
created: 2026-08-10T21:35:15Z
created_by: a-go-auditor-p03sgd
owner: a-root
priority: should
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
---
# timeline build: validate the composed timeline before writing so a duplicate producer id fails cleanly instead of silently persisting a timeline melt cannot render
## So that
a bad build (e.g. two clips sharing an explicit id) fails at the build step with a clean timeline.invalid naming the fault, instead of writing a broken MLT that returns success and only breaks later as an opaque melt error at render — matching every other timeline-producing path (reframe.go:157, shorts.go:242, timeline_edit.go:231) which all Validate before writing
## Acceptance
- [ ] timelineBuildCmd (cmd/cutvec/main.go:199-286) calls tl.Validate() after the timeline is fully composed (effects + sequence + audio) and BEFORE tl.Marshal()/os.WriteFile
- [ ] when Validate() reports problems the command returns an axerror with code timeline.invalid naming the problems and writes NOTHING to disk (mirror reframe.go:157-161 / shorts.go:242)
- [ ] a new cmd/cutvec/main_test.go case drives timelineBuildCmd with two clips carrying the same explicit id and asserts code timeline.invalid; it fails on current main (returns success today) and passes after the fix
- [ ] go build ./... and go test ./... both exit 0; gofmt clean
## Log
- 2026-08-10T21:37:11Z adopted by a-root (owner a-go-auditor-p03sgd orphaned)
