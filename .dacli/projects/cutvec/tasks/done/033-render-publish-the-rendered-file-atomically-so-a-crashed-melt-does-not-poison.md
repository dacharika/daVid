---
id: t-01KZPH0PCBN521KBNE7E48QQ4M
kind: task
created: 2026-08-10T19:04:59Z
created_by: a-go-auditor-k6rpnh
owner: a-root
priority: should
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
---
# render: publish the rendered file atomically so a crashed melt does not poison the cache
## So that
a render that dies mid-encode does not leave a truncated mp4 that the next resumed run returns as a completed cache hit, keeping render deterministic/idempotent/resumable (renderCmd main.go:1357,1364 passes the final cache path proj.RenderPath to the worker; on error main.go:1367-1369 leaves the partial file; HasRender project.go:581-584 is a bare os.Stat; contrast transcribe main.go:590-597 which writes the cache only after the worker fully succeeds)
## Acceptance
- [x] render publishes atomically: the worker writes to a temp path (e.g. out+'.tmp' or a temp file in CacheDir) and the code os.Rename()s it onto proj.RenderPath(hash,quality) only after the worker returns success; a worker failure leaves NO file at the cache path (or the partial output is removed on error)
- [x] a new test drives a fake render worker that writes partial bytes to the output path then exits non-zero (extend the fake worker at main_test.go:114-128 with a 'render-partial' mode), asserts the first render errors, asserts proj.HasRender(hash,quality) is false afterward, and asserts a second render with a WORKING worker re-renders (Cached:false) and writes the complete output
- [x] existing render tests still pass (TestRenderProducesOutput, TestRenderCacheHitIdempotent, TestRenderProxyAndFinalQuality, TestRenderFailureRules) and the worker.protocol 'did not write the output' check (main.go:1372-1375) still holds on the atomic path
- [x] go build ./... and go test ./... both exit 0; changed Go files are gofmt-clean
## Log
- 2026-08-10T20:57:57Z adopted by a-root (owner a-go-auditor-k6rpnh orphaned)
- 2026-08-10T21:37:13Z claimed by a-fixer-jcyhnt
- 2026-08-10T21:40:20Z a-fixer-jcyhnt: PR opened: https://github.com/mlnomadpy/cutvec/pull/37 (event 01KZPSWJCHXZ0C04N2V2350731)
- 2026-08-10T21:46:49Z accepted by a-root (applied 1 proposal(s))
- 2026-08-10T21:46:49Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:46:49Z deliverable: dacli/033-render-publish-the-rendered-file-atomically-so-a-crashed-melt-does-not-poison is merged into main
- 2026-08-10T21:46:49Z completed by a-root
- 2026-08-10T21:46:54Z accepted by a-root
- 2026-08-10T21:46:54Z closed WITHOUT verification — no --verify command was given
- 2026-08-10T21:46:54Z deliverable: dacli/033-render-publish-the-rendered-file-atomically-so-a-crashed-melt-does-not-poison is merged into main
- 2026-08-10T21:46:54Z completed by a-root
- 2026-08-10T21:57:23Z accepted by a-root
- 2026-08-10T21:57:23Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:57:23Z deliverable: no dacli/033-render-publish-the-rendered-file-atomically-so-a-crashed-melt-does-not-poison branch — nothing to check against main
- 2026-08-10T21:57:23Z completed by a-root
