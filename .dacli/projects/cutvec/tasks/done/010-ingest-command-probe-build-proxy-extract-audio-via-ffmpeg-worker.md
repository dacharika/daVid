---
id: t-01KZ9WMYQCA4X9CGWMZSHSRBZ0
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# ingest command: probe + build proxy + extract audio via ffmpeg worker
## So that
downstream steps run on a fast proxy and clean audio, cached by content hash
## Acceptance
- [ ] cutvec ingest <video> --json returns handles for proxy and audio plus probe metadata
      NOT MET ON MAIN — unchecked 2026-08-11 by a-root. `cutvec schema ingest` reports
      output fields ['cached','planned','source'] with source ['hash','id','path','probe']:
      no proxy, no audio. project.Source has no such fields and ffmpeg_worker.py
      implements a probe op only. Carried forward as task 039.
- [x] artifacts are cached and re-ingest is a no-op hit
      (this half is real: content-hash caching works and re-ingest returns cached:true)
## Log
- 2026-08-05T23:41:01Z claimed by a-fixer-andad9
- 2026-08-06T00:14:52Z a-fixer-andad9: PR opened: https://github.com/mlnomadpy/cutvec/pull/7 (event 01KZA5D8GEDN5QG3Z4NG9FJ04Y)
- 2026-08-06T00:14:56Z accepted by a-root
- 2026-08-06T00:14:56Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-06T00:14:56Z completed by a-root
- 2026-08-06T01:29:39Z status done proposed by a-fixer-andad9, applied (event 01KZA5DE1NY6KR2H4XADKVDQHB)
- 2026-08-11 a-root: first acceptance box unchecked — it was never true on main. The
  accept above fired 4 seconds after the PR was opened (00:14:52 -> 00:14:56), i.e. on
  PR-opened rather than PR-merged, and the `go build && go test` that "verified" it ran
  against the agent's own branch, not trunk. PR #7 then sat open for six days and is now
  closed as superseded: it models Source.Probe as map[string]any, while task 022 gave main
  a typed *Probe{Width,Height,Duration,Fps} that shorts/reframe-edit read for the crop
  window, so merging it would break the 023 framing fix. Remaining work re-filed as 039,
  scoped additively against current main. Filed dacli#443 for the accept-on-propose gap.
