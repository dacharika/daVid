---
id: t-01KZPGBT24B21JAJ7654APQETK
kind: task
created: 2026-08-10T18:53:35Z
created_by: a-go-auditor-p6gy2n
owner: a-root
priority: should
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
---
# audio: emit a real <blank> element for a delayed sting or bed instead of an <entry length> MLT ignores
## So that
a delayed sting or music bed actually lands on its intended moment instead of silently playing from t=0
## Acceptance
- [ ] A timeline built with an audio start offset (AddAudio start>0) marshals a real MLT <blank length="..."/> element inside the audio track playlist, not an <entry length="..."> that MLT does not treat as a gap
- [ ] timeline.Parse round-trips a <blank> element back into the model, and Sequence/AddAudio still emit correct <entry producer="..."> for real producers
- [ ] A test asserts the marshaled XML contains <blank ...> for a delayed bed (replacing/augmenting effects_test.go:222 TestAudioStartOffsetBecomesALeadingBlank, which today asserts only the Go model field and never the XML)
- [ ] go build ./... and go test ./... both exit 0
## Log
- 2026-08-10T20:57:57Z adopted by a-root (owner a-go-auditor-p6gy2n orphaned)
