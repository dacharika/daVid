---
id: d-probe-dimensions-via-an-injected-project-prober-seam-nil-prober-no-probing
kind: note
note_kind: decision
created: 2026-08-10T16:40:31Z
created_by: a-fixer-hx5gwf
about: "[[022]]"
---
# Probe dimensions via an injected project.Prober seam; nil prober = no probing
## Chose
Probe dimensions via an injected project.Prober seam; nil prober = no probing
## Rejected
Make the ffmpeg worker mandatory in Ingest (worker.missing when unset), or shell out to ffprobe from Go
## Because
The recorded d-ingest-builds-proxy-audio-probe decision already sanctions Ingest(path, seam) with a nil seam preserving content-hash-only recording and keeping internal/project free of a worker import. A graceful nil-prober keeps all existing ingest tests/fixtures green (they configure no worker) while a new fake-ffmpeg-worker test proves width/height/duration/fps are recorded and status --json reports them; probe runs only on a cache miss so a re-ingest stays a pure cache hit.
