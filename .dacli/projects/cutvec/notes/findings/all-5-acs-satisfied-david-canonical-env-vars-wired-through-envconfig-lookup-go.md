---
id: f-all-5-acs-satisfied-david-canonical-env-vars-wired-through-envconfig-lookup-go
kind: note
note_kind: finding
created: 2026-08-17T17:16:32Z
created_by: a-k4wj4rkn62
about: "[[047]]"
severity: major
---
# all 5 ACs satisfied: DAVID_* canonical env vars wired through envconfig.Lookup (Go) and DAVID_*-first fallback (Python); doctor probes both via Probe seam; go build ./... and go test ./... -count=1 both exit 0; no raw CUTVEC_ in Go sources outside compatibility fallback sites
AC1: DAVID_FFMPEG_WORKER/WHISPER_WORKER/INDEX_WORKER/SEARCH_WORKER/MEDIAPIPE_WORKER/MELT_WORKER/WHISPER_MODEL/FACE_LANDMARKER/EMBED_MODEL/PROGRESS_INTERVAL all defined as canonical. AC2: envconfig.Lookup implements DAVID-wins / CUTVEC-fallback-with-warn / both-set-DAVID-wins logic (envconfig/envconfig.go). AC3: envconfig.Lookup centralises all Go lookups; doctor.go worker()/model() helpers accept canonicalEnv+legacyEnv, probe both via p.Getenv (internal/doctor/doctor.go); doctor_test.go fixtures use DAVID_* keys. AC4: workers/embedder.py, whisper_worker.py, mediapipe_worker.py read DAVID_* first, fall back to CUTVEC_* with structured stderr warning. AC5: go build ./... exit 0, go test ./... -count=1 all ok; grep CUTVEC_ *.go shows only fallback sites.
