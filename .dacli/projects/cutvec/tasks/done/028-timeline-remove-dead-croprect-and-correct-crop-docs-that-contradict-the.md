---
id: t-01KZPCZG4A965M0QMA7ENDFSAX
kind: task
created: 2026-08-10T17:54:26Z
created_by: a-go-auditor-1mtdcs
owner: a-root
priority: should
estimate: "{optimistic: 1, probable: 1, pessimistic: 2}"
---
# timeline: remove dead CropRect and correct crop docs that contradict the verified use_profile finding
## So that
the crop-filter helpers in internal/timeline stop carrying a black-rendering dead function and a doc comment that flatly claims the use_profile crop form is a no-op — a claim task 027 REFUTED by A/B on the real melt render path, and the exact 'wrong belief' 027 warned would get working PunchIn code deleted
## Acceptance
- [x] Remove the unused CropRect function (timeline.go:214-218): grep confirms zero callers across cmd/ and internal/, and it builds a qtcrop filter that renders black on the render worker's melt (memory cutvec-melt-crop-fill). If a caller is found, instead keep it and document its black-render/unused status.
- [x] Reword CropWindow's doc comment (timeline.go:226-228) so it no longer asserts the use_profile edge form renders black or is a no-op — task 027 verified crop+use_profile=1+explicit-edges crops-and-fills and PunchIn depends on it in production; note the real hazard is combining center=1 WITH explicit edges (which pads to black), per the d-keep-punchins-use-profile decision.
- [x] go build ./... and go test ./... both exit 0; gofmt, go vet, and staticcheck are clean
## Log
- 2026-08-10T18:10:53Z claimed by a-fixer-b774vg
- 2026-08-10T18:19:21Z adopted by a-root (owner a-go-auditor-1mtdcs orphaned)
- 2026-08-10T18:19:21Z accepted by a-root (applied 1 proposal(s))
- 2026-08-10T18:19:21Z closed WITHOUT verification — no --verify command was given
- 2026-08-10T18:19:21Z deliverable: dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the is merged into trunk
- 2026-08-10T18:19:21Z completed by a-root
- 2026-08-10T18:19:21Z a-fixer-b774vg: PR opened: https://github.com/mlnomadpy/cutvec/pull/30 (event 01KZPE0P78DJM02BH9NAD9FCHX)
- 2026-08-10T18:25:59Z accepted by a-root
- 2026-08-10T18:25:59Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:25:59Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T18:25:59Z completed by a-root
- 2026-08-10T18:36:09Z accepted by a-root
- 2026-08-10T18:36:09Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:36:09Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T18:36:09Z completed by a-root
- 2026-08-10T18:42:39Z accepted by a-root
- 2026-08-10T18:42:39Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:42:39Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T18:42:39Z completed by a-root
- 2026-08-10T18:49:09Z accepted by a-root
- 2026-08-10T18:49:09Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:49:09Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T18:49:09Z completed by a-root
- 2026-08-10T18:55:20Z accepted by a-root
- 2026-08-10T18:55:20Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:55:20Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T18:55:20Z completed by a-root
- 2026-08-10T19:01:49Z accepted by a-root
- 2026-08-10T19:01:49Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:01:49Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T19:01:49Z completed by a-root
- 2026-08-10T19:07:08Z accepted by a-root
- 2026-08-10T19:07:08Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:07:08Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T19:07:08Z completed by a-root
- 2026-08-10T19:13:47Z accepted by a-root
- 2026-08-10T19:13:47Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:13:47Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T19:13:47Z completed by a-root
- 2026-08-10T19:20:33Z accepted by a-root
- 2026-08-10T19:20:33Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:20:33Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T19:20:33Z completed by a-root
- 2026-08-10T19:25:49Z accepted by a-root
- 2026-08-10T19:25:49Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:25:49Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T19:25:49Z completed by a-root
- 2026-08-10T19:32:21Z accepted by a-root
- 2026-08-10T19:32:21Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:32:21Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T19:32:21Z completed by a-root
- 2026-08-10T19:35:56Z accepted by a-root
- 2026-08-10T19:35:56Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:35:56Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T19:35:56Z completed by a-root
- 2026-08-10T19:42:26Z accepted by a-root
- 2026-08-10T19:42:26Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:42:26Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T19:42:26Z completed by a-root
- 2026-08-10T19:48:55Z accepted by a-root
- 2026-08-10T19:48:55Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:48:55Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against trunk
- 2026-08-10T19:48:55Z completed by a-root
- 2026-08-10T21:08:24Z accepted by a-root
- 2026-08-10T21:08:24Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:08:24Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against main
- 2026-08-10T21:08:24Z completed by a-root
- 2026-08-10T21:20:10Z accepted by a-root
- 2026-08-10T21:20:10Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:20:10Z deliverable: no dacli/028-timeline-remove-dead-croprect-and-correct-crop-docs-that-contradict-the branch — nothing to check against main
- 2026-08-10T21:20:10Z completed by a-root
