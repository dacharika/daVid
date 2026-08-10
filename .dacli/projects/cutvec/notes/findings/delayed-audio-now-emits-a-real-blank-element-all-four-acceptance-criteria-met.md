---
id: f-delayed-audio-now-emits-a-real-blank-element-all-four-acceptance-criteria-met
kind: note
note_kind: finding
created: 2026-08-10T21:30:59Z
created_by: a-fixer-0pvg7s
about: "[[032]]"
severity: major
---
# Delayed audio now emits a real <blank> element; all four acceptance criteria met
Root cause: Entry marshaled under one xml:"entry" field tag, so a delayed bed's gap became <entry length=...> which MLT ignores (plays the sound from t=0). Fix (internal/timeline/timeline.go): Playlist now carries custom MarshalXML (emits <blank length=.../> for Entry.Blank, else <entry producer=.../>, order preserved) and UnmarshalXML (routes both <blank> and <entry> children back into Entries). AC1: AddAudio start>0 -> playlist marshals <blank length="HH:MM:SS.mmm"/> (verified in test XML). AC2: Parse round-trips <blank> back to Entry.Blank; Sequence/AddAudio still emit <entry producer=...> for real producers. AC3: effects_test.go:227 TestAudioStartOffsetBecomesALeadingBlank now asserts the marshaled XML contains <blank length="00:00:12.500"> and round-trips; new internal/timeline TestBlankMarshalsAndRoundTrips covers it at the package level. AC4: go build ./... and go test ./... both exit 0; gofmt clean. Commit ff27ff9.
