---
id: f-timeline-inspect-edit-implemented-and-green-both-acs-verified
kind: note
note_kind: finding
created: 2026-08-10T18:05:37Z
created_by: a-fixer-0mj66x
about: "[[026]]"
severity: major
---
# timeline inspect/edit implemented and green: both ACs verified
AC1: cmd/cutvec/timeline_edit.go adds 'timeline inspect' (returns {producers:[{id,resource,in,out,filters:[{service,index,properties}]}],valid,problems}) and 'timeline edit' (remove:[{producer,service}] + add:[effectSpec]) dispatched from timelineCmd (cmd/cutvec/main.go) on args[0]; the historic bare 'timeline' build is now timelineBuildCmd and still works (TestTimelineBuildStillWorks). timeline.Parse (internal/timeline/timeline.go) is Marshal's inverse via xml.Unmarshal; Filter.Service(), Timeline.Producer(id), Producer.RemoveFilterService(service) are the mutation primitives. edit reuses resolveEffects/applyEffects so producer-existence and preset/service handling match build. AC2: edit re-validates (tl.Validate) BEFORE writing and refuses on problems (timeline.invalid) without touching the file; TestTimelineEditThenRenders drives the edited timeline through render's fake melt worker end-to-end (not cached, output written). MCP: callTool special-cases 'timeline' op=inspect|edit (mcp.go); catalog 'timeline' tool documents op/remove/add/inspect outputs. Failure rules pinned (TestTimelineEditFailureRules: input.invalid/input.empty/effect.unknown_producer/filter.not_found/cli.unknown_flag; a failed edit leaves the file intact). Gates: go build ./... ok; gofmt clean; go vet clean; staticcheck (go run honnef.co) exit 0; go test -race -count=1 ./... ALL ok. Live e2e with the built binary: inspect showed the crop, edit removed it + added the 3-filter grade preset (removed:1 added:3 valid:true), re-inspect confirmed, filter.not_found returned on a bogus removal.
