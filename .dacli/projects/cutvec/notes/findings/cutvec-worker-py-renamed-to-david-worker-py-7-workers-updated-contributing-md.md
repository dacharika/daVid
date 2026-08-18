---
id: f-cutvec-worker-py-renamed-to-david-worker-py-7-workers-updated-contributing-md
kind: note
note_kind: finding
created: 2026-08-18T12:37:11Z
created_by: a-ctky0m2cwb
about: "[[049]]"
severity: minor
---
# cutvec_worker.py renamed to david_worker.py; 7 workers updated; CONTRIBUTING.md updated; all checks pass
workers/cutvec_worker.py deleted, workers/david_worker.py created with updated docstring (cutvec->daVid). Imports updated in: melt_worker.py, mediapipe_worker.py, index_worker.py, embedder.py, search_worker.py, whisper_worker.py, ffmpeg_worker.py. CONTRIBUTING.md:66 updated to reference david_worker.py. ruff check/format exit 0, python3 -m compileall exit 0, go build ./... exit 0, go test ./... exit 0.
