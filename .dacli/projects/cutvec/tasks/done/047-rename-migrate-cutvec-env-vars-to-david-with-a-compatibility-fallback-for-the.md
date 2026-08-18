---
id: t-01M06VEJEMX585NE7ZMB44STEG
kind: task
created: 2026-08-17T03:15:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 2, probable: 4, pessimistic: 6}"
depends_on: "[046]"
---
# rename: migrate CUTVEC_* env vars to DAVID_* with a compatibility fallback for the deprecation window
## So that
agents and operators configure the tool with DAVID_* variables while existing CUTVEC_* setups continue to work through the documented deprecation window
## Acceptance
- [x] canonical env vars are DAVID_FFMPEG_WORKER, DAVID_WHISPER_WORKER, DAVID_INDEX_WORKER, DAVID_SEARCH_WORKER, DAVID_MEDIAPIPE_WORKER, DAVID_MELT_WORKER, DAVID_WHISPER_MODEL, DAVID_FACE_LANDMARKER, DAVID_EMBED_MODEL, DAVID_PROGRESS_INTERVAL
- [x] when only DAVID_* is set it is used; when only CUTVEC_* is set it is used with a deprecation warning on stderr; when both are set DAVID_* wins and a structured warning is emitted; JSON stdout is not polluted
- [x] env lookup is centralised (not scattered per-command); doctor.go and its test fixtures reflect DAVID_* as canonical
- [x] Python workers (embedder.py, mediapipe_worker.py, whisper_worker.py) read DAVID_* first, fall back to CUTVEC_* with a warning
- [x] go build ./... and go test ./... -count=1 both exit 0; grep for raw CUTVEC_ in Go sources shows only compatibility fallback sites
## Log
- 2026-08-17T12:34:28Z claimed by a-r862mrr8mk
- 2026-08-18T12:29:35Z a-k4wj4rkn62: PR opened: https://github.com/mlnomadpy/daVid/pull/69 (event 01M08BMQFJWYDWRK5H5S685VY4)
- 2026-08-18T12:29:35Z status done proposed by a-k4wj4rkn62, applied (event 01M08BMZF3K29KR7X07CAWHD7T)
- 2026-08-18T12:29:38Z accepted by a-root
- 2026-08-18T12:29:38Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-18T12:29:38Z completed by a-root
