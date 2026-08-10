---
id: d-playlist-carries-custom-marshalxml-unmarshalxml-to-emit-blank-vs-entry-per-child
kind: note
note_kind: decision
created: 2026-08-10T21:30:43Z
created_by: a-fixer-0pvg7s
about: "[[032]]"
---
# Playlist carries custom MarshalXML/UnmarshalXML to emit <blank> vs <entry> per child
## Chose
Playlist carries custom MarshalXML/UnmarshalXML to emit <blank> vs <entry> per child
## Rejected
Keep Entry with xml:"entry" struct tags and both length/producer attrs on one <entry>
## Because
Go xml marshals a slice field under one element name from the field tag, so a blank Entry became <entry length=...> which MLT does not treat as a gap. A custom Playlist encoder chooses <blank length=.../> vs <entry producer=.../> per child and preserves their order; a matching decoder routes both back into Entries so Parse round-trips a delayed bed's leading gap.
