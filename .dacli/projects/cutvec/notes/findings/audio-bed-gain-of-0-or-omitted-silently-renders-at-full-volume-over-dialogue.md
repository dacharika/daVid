---
id: f-audio-bed-gain-of-0-or-omitted-silently-renders-at-full-volume-over-dialogue
kind: note
note_kind: finding
created: 2026-08-10T18:33:36Z
created_by: a-go-auditor-bhk1rs
about: "[[006]]"
severity: major
---
# Audio bed gain of 0 (or omitted) silently renders at full volume over dialogue
applyAudio (cmd/cutvec/effects.go:226-230) validates only spec.Gain<0 || spec.Gain>1, so a gain of exactly 0 — including the JSON zero value when 'gain' is omitted — is accepted as valid. But AddAudio (internal/timeline/timeline.go:97-99) then does 'if gain <= 0 { gain = 1 }', rewriting that 0 to full scale 1.0. The mix transition sets start=end=gain (timeline.go:124-131), so the bed is mixed at sum=1 at FULL volume over the dialogue. This directly contradicts the catalog schema, which documents audio as 'sounds laid under the edit ... the dialogue stays audible beneath them' (internal/catalog/catalog.go:521) and gain as 'level from 0 to 1; around 0.15 sits a music bed under dialogue without fighting it' (catalog.go:528). Concrete failure: applyAudio(tl, []audioSpec{{Resource:"/music.wav"}}) (no gain) or {{Resource, Gain:0}} yields a mix transition with start=end="1", a music bed blasting over speech — the opposite of a bed. Also a request to mute a bed to 0 is silently turned into full volume. Latent/green: no test covers gain=0 or omitted gain; TestAudioBedAddsAMixedTrackNotAReplacement (effects_test.go:197,216) uses 0.15 and asserts start/end=="0.15"; TestAudioGainOutOfRangeIsRejected (effects_test.go:275) only checks gain>1 and missing-resource. Provable in Go by building the timeline and asserting the mix transition's start/end. Distinct from open task 029 (hook/caption SRT overlap) and done task 020 (grade/denoise/sfx effect presets, which is picture/filter chains, not the audio-bed gain default).
