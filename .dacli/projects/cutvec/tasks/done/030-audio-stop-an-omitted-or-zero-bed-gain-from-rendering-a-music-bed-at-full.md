---
id: t-01KZPF8MGWF5MVKXAY81J6RRS2
kind: task
created: 2026-08-10T18:34:22Z
created_by: a-go-auditor-bhk1rs
owner: a-root
priority: should
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
---
# audio: stop an omitted or zero bed gain from rendering a music bed at full volume over dialogue
## So that
an agent that lays a music bed without naming a gain gets a bed under the dialogue, not full-volume music drowning the speech — matching the catalog schema's promise that the dialogue stays audible beneath
## Acceptance
- [x] An audioSpec with gain omitted no longer yields a mix transition whose start/end is "1": it produces either a documented low bed default or a clear validation error, pinned by a new Go test that builds the timeline and asserts the mix transition start/end (no worker/render). Evidence: applyAudio cmd/cutvec/effects.go:226-230 accepts gain==0; AddAudio internal/timeline/timeline.go:97-99 rewrites gain<=0 to 1.0; mix uses start=end=gain timeline.go:124-131; schema catalog.go:521,528 promises a bed under dialogue at ~0.15.
- [x] An explicit gain of 0 behaves per the chosen design and is covered by a test — a silent bed stays 0 OR 0 is rejected with a stable axerror code plus hint — so applyAudio's accepted range, AddAudio's transform, and the catalog schema wording (catalog.go:528) no longer disagree.
- [x] Existing audio tests still pass (TestAudioBedAddsAMixedTrackNotAReplacement, TestAudioGainOutOfRangeIsRejected, TestAddAudio*); go build ./... and go test ./... both exit 0; gofmt clean. Out of scope: MLT mix rendering, the grade/denoise/sfx effect presets, and the hook/caption overlap (open task 029).
## Log
- 2026-08-10T20:57:57Z adopted by a-root (owner a-go-auditor-bhk1rs orphaned)
- 2026-08-10T21:08:28Z claimed by a-fixer-w108mq
- 2026-08-10T21:13:41Z a-fixer-w108mq: PR opened: https://github.com/mlnomadpy/cutvec/pull/34 (event 01KZPRBKGC5CMN3WR6ZKDPZZ5N)
- 2026-08-10T21:20:11Z accepted by a-root (applied 1 proposal(s))
- 2026-08-10T21:20:11Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:20:11Z deliverable: dacli/030-audio-stop-an-omitted-or-zero-bed-gain-from-rendering-a-music-bed-at-full is merged into main
- 2026-08-10T21:20:11Z completed by a-root
- 2026-08-10T21:20:14Z accepted by a-root
- 2026-08-10T21:20:14Z closed WITHOUT verification — no --verify command was given
- 2026-08-10T21:20:14Z deliverable: dacli/030-audio-stop-an-omitted-or-zero-bed-gain-from-rendering-a-music-bed-at-full is merged into main
- 2026-08-10T21:20:14Z completed by a-root
- 2026-08-10T21:27:49Z accepted by a-root
- 2026-08-10T21:27:49Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:27:49Z deliverable: no dacli/030-audio-stop-an-omitted-or-zero-bed-gain-from-rendering-a-music-bed-at-full branch — nothing to check against main
- 2026-08-10T21:27:49Z completed by a-root
- 2026-08-10T21:37:09Z accepted by a-root
- 2026-08-10T21:37:09Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:37:09Z deliverable: no dacli/030-audio-stop-an-omitted-or-zero-bed-gain-from-rendering-a-music-bed-at-full branch — nothing to check against main
- 2026-08-10T21:37:09Z completed by a-root
