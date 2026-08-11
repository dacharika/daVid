---
id: t-01KZR44FA61CV423GCHS33EWRZ
kind: task
created: 2026-08-11T09:58:20Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# ingest: build the proxy and audio handles task 010 was closed against but never landed
## So that
the pipeline runs its expensive steps on a small proxy and an extracted audio track instead of the 4K original, and `ingest`'s documented output matches what it actually returns

## Context — a closed task whose work is not on main
Task 010 sits in `done/` with this acceptance box **checked**:

- [x] `cutvec ingest <video> --json` returns handles for proxy and audio plus probe metadata

Main does not do this. Verified against the built binary:

    $ cutvec schema ingest
    ingest OUTPUT fields: ['cached', 'planned', 'source']
    source subfields:     ['hash', 'id', 'path', 'probe']

No `proxy`, no `audio`. `internal/project/project.go` has no such fields, and
`workers/ffmpeg_worker.py` implements a probe op only — there is no proxy or
audio-extraction op anywhere in the worker.

The work exists only in PR #7, open and unmerged since 2026-08-05 and now
**closed as superseded**: that branch models `Source.Probe` as `map[string]any`,
while task 022 gave main a typed `*Probe{Width,Height,Duration,Fps}` that
`shorts` and `reframe-edit` read to compute the crop window (the fix for 023's
full-body framing). Merging #7 would replace the typed struct with an untyped map
and break both recipes; a test merge conflicts in all five files it touches.

So what remains is **additive on current main** and does not need #7's rework.

## Acceptance
- [x] `workers/ffmpeg_worker.py` gains two ops alongside the existing probe — one building a proxy (small, fast-seeking, same duration) and one extracting the audio track — each answering the versioned JSON contract and returning a structured error, not a traceback, when ffmpeg is absent (the `TestWorkerContract` gate in CI must still pass)
- [x] `project.Source` gains `Proxy` and `Audio` string handles, `omitempty`, alongside the existing typed `Probe` — which is NOT changed to a map
- [x] `ingest` builds both on a content-hash cache **miss** only, and a re-ingest of unchanged bytes stays a pure cache hit that runs no worker (the existing `cached:true` behaviour is unchanged); a missing ffmpeg worker skips them the way probing already skips, rather than failing the ingest
- [x] the catalog's `ingest` output schema documents both handles, so `cutvec schema ingest` and the MCP `tools/list` stop under-reporting the real output
- [x] a Go test drives ingest with the re-exec'd fake worker and asserts both handles come back and that a second ingest returns them from cache with no worker invocation; `go build ./...` and `go test ./... -count=1` exit 0 and every changed file is gofmt-clean

## Notes
Scope deliberately excludes making any downstream step *use* the proxy —
transcribe/index/speakers keep reading what they read today. Switching them over
is a separate change with its own quality trade-offs, and bundling it here would
hide a behaviour change inside an additive one.

## Log
- 2026-08-11 filed by a-root after finding task 010's checked acceptance box unmet on main; PRs #7 (stale Probe rework) closed as superseded with the reasoning recorded there
- 2026-08-11T11:16:38Z accepted by a-root
- 2026-08-11T11:16:38Z verified by `go build ./... && go test ./... -count=1` (exit 0)
- 2026-08-11T11:16:38Z deliverable: no dacli/039-ingest-build-the-proxy-and-audio-handles-task-010-was-closed-against-but-never branch — nothing to check against main
- 2026-08-11T11:16:38Z completed by a-root
