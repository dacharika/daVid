---
id: t-01KZPBSQ25A5938GVM1A94AD7V
kind: task
created: 2026-08-10T17:33:47Z
created_by: a-go-auditor-vsb4em
owner: a-root
priority: should
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
---
# reframe-edit: fix punch-ins that render as a no-op on the melt build
## So that
reframe-edit actually reframes: today every left/right single renders as the untouched wide two-shot because PunchIn (timeline.go:267-288) uses the crop use_profile edge form that task 023 VERIFIED is a no-op on the render worker's melt (melt <tl> -consumer avformat); shorts was migrated to source-pixel CropWindow but reframe-edit was left behind, so the recipe's headline capability is silently broken (see finding reframe-edit-s-punchin-crop-is-a-no-op...)
## Acceptance
- [x] reframe-edit's per-shot crop uses source-pixel left/right/top/bottom edges with NO use_profile, matching the CropWindow form 023 verified crops-and-fills; source dims come from the 022 probe via sourceDims (effects.go:196)
- [x] A test asserts the reframe punch-in filter carries no use_profile property (mirroring CropWindow's timeline_test.go:205-207); the existing TestPunchInKeepsTheRequestedFraction assertion of use_profile==1 (timeline_test.go:179) is updated, not left enshrining the broken form
- [x] Verified against the real render path on a synthetic source that a left/right single visibly crops-and-fills (subject reframed) rather than rendering the untouched wide frame — evidence from an extracted frame, as done in 023
- [x] Dead CropRect (qtcrop, renders black per the note, uncalled in production) is removed or its black-rendering/unused status is documented
- [x] go build ./... and go test ./... both exit 0; gofmt, go vet, and staticcheck are clean
## Log

- 2026-08-10T17:43:21Z adopted by a-root (owner a-go-auditor-vsb4em orphaned)
- 2026-08-10T17:43:21Z accepted by a-root (applied 1 proposal(s))
- 2026-08-10T17:43:21Z closed WITHOUT verification — no --verify command was given
- 2026-08-10T17:43:21Z deliverable: no dacli/027-reframe-edit-fix-punch-ins-that-render-as-a-no-op-on-the-melt-build branch — nothing to check against trunk
- 2026-08-10T17:43:21Z completed by a-root
### Closed as refuted — the premise does not hold

Isolated A/B on the real render path (`melt <tl> -consumer avformat`), same
source and same instant, differing only in the crop edges:

| crop | result |
|---|---|
| left/right/top/bottom = 0 | the untouched wide two-shot |
| left/right = 528, top/bottom = 297, `use_profile=1` | visibly cropped and filled |

The `use_profile` edge form crops correctly, so the singles are not rendering as
the wide frame. A full `reframe-edit` render confirms it end to end: the wide
shot shows both people, the right single fills with one, the left single with
the other.

The underlying observation was real but over-generalised. Combining `center=1`
*with* explicit edges does misbehave — it pads to black bars rather than
tightening — and that is what the earlier experiment hit. It does not follow
that `use_profile` edges never crop.

Acting on the acceptance criteria would have removed working code and rewritten
a passing test (`TestPunchInKeepsTheRequestedFraction`) to enshrine the wrong
belief. Kept as-is; see the decision note "Keep PunchIn's use_profile crop form
in reframe-edit".

One item here is still worth doing separately: `CropRect` (qtcrop) is unused in
production and reportedly renders black. That is filed as its own task rather
than carried by a refuted one.
