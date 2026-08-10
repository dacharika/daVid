---
id: f-audio-start-offset-marshals-as-entry-length-not-blank-so-a-delayed-sting-bed
kind: note
note_kind: finding
created: 2026-08-10T18:52:54Z
created_by: a-go-auditor-p6gy2n
about: "[[006]]"
severity: major
---
# Audio start offset marshals as <entry length> not <blank>, so a delayed sting/bed silently plays from t=0
internal/timeline/timeline.go models a playlist gap with Entry.Blank (timeline.go:32-35: Blank string `xml:"length,attr,omitempty"` on the Entry struct, which marshals as <entry>). AddAudio's start offset appends exactly such a gap (timeline.go:113-116: pl.Entries = append(pl.Entries, Entry{Blank: Clock(start)})). PROVEN in Go with no render: marshaling a timeline built via AddAudio("sting","/s.wav",0.5,12.5,0,0) emits\n  <playlist id="sting_track">\n    <entry length="00:00:12.500"></entry>\n    <entry producer="sting"></entry>\n  </playlist>\nMLT's empty-span element in a playlist is <blank length="..."/>, a distinct child element from <entry>; an <entry> with a length attr and NO producer is not a valid blank (mlt_xml parses on_start_blank vs on_start_entry separately). So the delay is dropped and the sting plays from t=0 — the exact opposite of the documented capability 'start offsets the sound into the timeline, so a sting can land on a moment rather than at the top' (timeline.go:85-87). Latent/green: TestAudioStartOffsetBecomesALeadingBlank (cmd/cutvec/effects_test.go:222-241) asserts only the Go model field bed.Entries[0].Blank=="00:00:12.500", never the marshaled XML, so the malformed output ships green. Fix: give Timeline a distinct Blank element type (xml:"blank") for gaps rather than overloading Entry, and assert the <blank> element in the round-trip test. Distinct from open 030 (bed gain) and 031 (multi-clip sequencing) and done 020 (picture-filter presets).
