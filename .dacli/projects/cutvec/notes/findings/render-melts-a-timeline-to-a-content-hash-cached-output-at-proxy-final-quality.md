---
id: f-render-melts-a-timeline-to-a-content-hash-cached-output-at-proxy-final-quality
kind: note
note_kind: finding
created: 2026-08-06T07:25:58Z
created_by: a-fixer-0kc19a
about: "[[017]]"
severity: major
---
# render melts a timeline to a content-hash-cached output at proxy/final quality; both ACs pass
renderCmd (cmd/cutvec/main.go) — dispatched at run() case 'render'. AC1 (cutvec render <timeline> --json returns the output handle): TestRenderProducesOutput drives 'render <project> --json' and asserts a returned output handle that resolves to a real video the fake melt worker wrote via a re-exec'd TestHelperProcess (WORKER_MODE=render, no melt/ffmpeg on PATH). AC2 (--proxy and --final quality): parseRenderArgs reads the mutually-exclusive --proxy/--final flags (default proxy via resolveQuality); TestRenderProxyAndFinalQuality proves each tier writes a DISTINCT output. Cache/idempotency: output keyed on the built timeline's sha256 + quality at project.RenderPath = cache/render-<hash>-<quality>.mp4 (internal/project/project.go); HasRender short-circuits a re-run to cached:true running NO worker (TestRenderCacheHitIdempotent points the 2nd run at /nonexistent yet gets cached:true). melt worker is a configurable subprocess (request.worker or CUTVEC_MELT_WORKER via resolveMeltWorker), op=render params {timeline,output,quality}; Go verifies the worker wrote the file (worker.protocol otherwise). Failure rules pinned by TestRenderFailureRules: input.invalid (no/too-many project, both quality flags, unknown quality, malformed json), cli.unknown_flag, project.uninitialized, timeline.missing (no built timeline), worker.missing. status graph gains render:proxy/render:final artifacts (TestRenderStatusArtifacts). catalog gains the 'render' tool (describe now lists 7). Uncached 'go build ./...' exit 0; 'go test -count=1 ./...' ALL ok; gofmt clean. Live e2e with a shell melt worker confirmed cached:false then cached:true, distinct proxy/final files in cache/, and timeline.missing on an un-built project.
