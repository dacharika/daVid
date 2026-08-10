---
id: f-ingest-probes-records-source-width-height-duration-fps-via-ffmpeg-worker-re
kind: note
note_kind: finding
created: 2026-08-10T16:47:38Z
created_by: a-fixer-hx5gwf
about: "[[022]]"
severity: major
---
# ingest probes & records source width/height/duration/fps via ffmpeg worker; re-ingest stays a cache hit
project.Source gains Probe *Probe{Width,Height,Duration,Fps} (internal/project/project.go). Ingest signature is now Ingest(path, project.Prober): on a cache MISS it calls Prober.Probe(path) once and stores the probe on the source before recording it; on a cache HIT it returns the cached source (probe included) running no prober. A nil Prober skips probing (source still recorded, Probe nil) — keeps internal/project free of a worker import per [[d-probe-dimensions-via-an-injected-project-prober-seam-nil-prober-no-probing]]. cmd/cutvec/main.go adds ffmpegProber (worker op=probe), resolveProber (request.worker or CUTVEC_FFMPEG_WORKER; nil when neither set), probeFromResult, and worker/workerArgs on ingestRequest. status --json reports the probe automatically (Source is embedded in the StateGraph). catalog ingest schema documents the new input/output. New: workers/ffmpeg_worker.py (op=probe, ffprobe -> width/height/duration/fps). AC1 proven by TestIngestProbesDimensions (cmd, fake WORKER_MODE=probe helper) + TestIngestProbesOnceAndCaches (project unit); AC2 by the same cache-hit assertions (re-ingest with worker=/nonexistent returns cached:true, same probe, no worker run). Live e2e: real ffprobe on a generated 1280x720 3s 30fps clip returned those exact dims through the CLI; re-ingest cached:true. Gates: gofmt/vet/staticcheck clean, go test -race all ok, ruff check+format clean, compileall ok, coverage 68.2% > 62% floor.
