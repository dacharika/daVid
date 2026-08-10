---
id: t-01KZPCZG4A965M0QMA7ENDFSAX
kind: task
created: 2026-08-10T17:54:26Z
created_by: a-go-auditor-1mtdcs
owner: a-go-auditor-1mtdcs
priority: should
estimate: "{optimistic: 1, probable: 1, pessimistic: 2}"
---
# timeline: remove dead CropRect and correct crop docs that contradict the verified use_profile finding
## So that
the crop-filter helpers in internal/timeline stop carrying a black-rendering dead function and a doc comment that flatly claims the use_profile crop form is a no-op — a claim task 027 REFUTED by A/B on the real melt render path, and the exact 'wrong belief' 027 warned would get working PunchIn code deleted
## Acceptance
- [ ] Remove the unused CropRect function (timeline.go:214-218): grep confirms zero callers across cmd/ and internal/, and it builds a qtcrop filter that renders black on the render worker's melt (memory cutvec-melt-crop-fill). If a caller is found, instead keep it and document its black-render/unused status.
- [ ] Reword CropWindow's doc comment (timeline.go:226-228) so it no longer asserts the use_profile edge form renders black or is a no-op — task 027 verified crop+use_profile=1+explicit-edges crops-and-fills and PunchIn depends on it in production; note the real hazard is combining center=1 WITH explicit edges (which pads to black), per the d-keep-punchins-use-profile decision.
- [ ] go build ./... and go test ./... both exit 0; gofmt, go vet, and staticcheck are clean
## Log
