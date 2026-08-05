---
id: f-first-feature-slice-implemented-cutvec-timeline-carries-a-json-request-end-to
kind: note
note_kind: finding
created: 2026-08-05T14:43:38Z
created_by: a-fixer-zqmn1k
about: "[[004]]"
severity: major
---
# First feature slice implemented: cutvec timeline carries a JSON request end-to-end to a written MLT document + handle
New CLI tool 'timeline' (cmd/cutvec/main.go:timelineCmd) reads a JSON clip list on stdin, builds an MLT XML timeline via internal/timeline, writes it to <project>/timeline.mlt via internal/project.TimelinePath (project.go), and returns a JSON handle {timeline,producers} on stdout — the README data-flow path minus the Python worker (out of scope). Registered in internal/catalog/catalog.go with input/output JSON Schemas. Supports --plan dry-run (writes nothing, returns planned:true). Failure/empty rules: empty stdin or empty clips -> input.empty; missing project / empty resource / malformed JSON -> input.invalid (structured axerror {code,hint,retryable}, exit 1). Verified with the real binary end-to-end. Tests in cmd/cutvec/main_test.go: TestTimelineSlice (asserts the handle resolves to a real MLT doc containing the clips) fails cleanly when the write path breaks (demonstrated by pointing WriteFile at a wrong path -> '--- FAIL: TestTimelineSlice ... was not written'); TestTimelinePlanWritesNothing, TestTimelineFailureRules. go build ./... and go test ./... both exit 0; gofmt + go vet clean.
