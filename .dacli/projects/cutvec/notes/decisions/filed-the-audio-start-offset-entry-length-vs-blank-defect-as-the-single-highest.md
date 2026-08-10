---
id: d-filed-the-audio-start-offset-entry-length-vs-blank-defect-as-the-single-highest
kind: note
note_kind: decision
created: 2026-08-10T18:53:05Z
created_by: a-go-auditor-p6gy2n
about: "[[006]]"
---
# Filed the audio start-offset <entry length> vs <blank> defect as the single highest-value evidence-based change
## Chose
Filed the audio start-offset <entry length> vs <blank> defect as the single highest-value evidence-based change
## Rejected
Re-filing the multi-clip sequencing defect (already open as 031), the bed gain-0 default (already open as 030), or the hook/caption SRT overlap (already open as 029); re-filing the refuted reframe PunchIn no-op (refuted by verified d-keep-punchin); or the applyAudio map-iteration determinism nit (needs 2+ distinct producer targets on audio effects, rare and hard to pin deterministically)
## Because
The blank/start defect is structural and PROVEN from source with no melt render: AddAudio's start offset (timeline.go:113-116) appends Entry{Blank: Clock(start)} whose only tag is xml:"length,attr" on the Entry struct (timeline.go:32-35), so it marshals as <entry length="00:00:12.500"> — I built and marshaled exactly this and confirmed there is no <blank> element. MLT's playlist gap is the distinct <blank> element; an <entry> with a length attr and no producer is not a valid blank, so the delay is dropped and a delayed sting/bed plays from t=0, breaking the documented 'a sting can land on a moment' capability (timeline.go:85-87). It is unit-testable in Go by asserting the marshaled XML, and the existing TestAudioStartOffsetBecomesALeadingBlank (effects_test.go:222-241) asserts only the Go model field bed.Entries[0].Blank and never the XML, so the defect ships green. It is distinct from every open task (029 hook overlap, 030 bed gain, 031 sequencing) and every done task, and from the audio-gain finding because it is the playlist-gap ELEMENT SHAPE, not the mix-transition gain value.
