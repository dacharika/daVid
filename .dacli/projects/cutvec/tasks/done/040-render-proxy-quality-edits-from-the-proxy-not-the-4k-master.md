---
id: t-01KZRJ2JW6WCDGK8HRX49H7Z3R
kind: task
created: 2026-08-11T12:40:00Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# render proxy-quality edits from the proxy, not the 4K master
## So that
a draft render — the loop an agent iterates an edit in — stops decoding 4K frames it is about to throw away at 540p

## Measured before proposing
Transcoding the same 20s to 540p, on this machine:

    from the 4K master   5.491s
    from the 540p proxy  0.895s     6.1x

`shorts` and `reframe-edit` both take `quality: proxy|final`, and `proxy` is
explicitly the fast-draft tier — but both still point melt at the master. The
proxy ingest already builds is read by `speakers` (see the seat-detection change)
and by nothing else.

## The hazard this must not walk into
Seat positions are normalised (`cx`/`cy` in 0..1), which is why swapping the
detector's input to the proxy was safe. **Crop windows are not.** `CropWindow`
takes a source-pixel rectangle against `srcW`/`srcH`, and `sourceDims`
(cmd/cutvec/effects.go:196) reads `Source.Probe` — the *master's* dimensions.

Pointing melt at the proxy while still computing insets against 3840x2160 would
crop a 960x540 frame using 4K coordinates: the output would render successfully
and be framed on nothing. That is the same class as the `center_bias` sign
inversion and the `center=1` full-body bug, both of which looked fine until
someone rendered a frame and looked.

So the proxy's own dimensions have to be recorded and used, not derived.
Deriving them (`540` and `even(masterW * 540/masterH)`) is right for 3840x2160
but bakes in the worker's current default height and its rounding rule.

## Acceptance
- [x] the ffmpeg worker's `proxy` op reports the dimensions it actually produced, alongside the existing `{output, bytes}`
- [x] `project.Source` records them, so a cache-hit re-ingest returns them without re-running the worker; the master's `Probe` is unchanged and still typed
- [x] `shorts` and `reframe-edit` at `quality: proxy` point melt at `Source.Proxy` AND compute crops against the recorded proxy dimensions; at `quality: final` both are unchanged and read the master
- [x] a missing or unrecorded proxy falls back to the master rather than failing — the proxy is an optimisation, not a precondition — and a test pins that direction too
- [x] a Go test asserts the resource and the dims travel together: rendering proxy-quality must never pair the proxy resource with master dimensions, since that combination renders green and frames nothing
- [x] verified by rendering a proxy-quality reframe on real 4K footage and LOOKING at both cuts, not by exit code
- [x] `go build ./...` and `go test ./... -count=1` exit 0; gofmt, vet, ruff clean

## Notes
Deliberately out of scope: rendering `final` from the proxy (it would destroy the
output), and using the proxy for anything in `transcribe`. Reusing the cached
audio there was measured and rejected — it saves 0.885s on a 23 GB source,
because ffmpeg reads only the audio stream and never decodes video.

## Log
- 2026-08-11 filed by a-root with the measurement above, after wiring `speakers` to the proxy showed the proxy was otherwise dead weight
- 2026-08-11T14:13:50Z accepted by a-root
- 2026-08-11T14:13:50Z verified by `go build ./... && go test ./... -count=1` (exit 0)
- 2026-08-11T14:13:50Z deliverable: no dacli/040-render-proxy-quality-edits-from-the-proxy-not-the-4k-master branch — nothing to check against main
- 2026-08-11T14:13:50Z completed by a-root
