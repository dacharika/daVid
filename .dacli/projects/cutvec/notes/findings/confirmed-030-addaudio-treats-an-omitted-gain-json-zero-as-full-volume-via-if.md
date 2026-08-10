---
id: f-confirmed-030-addaudio-treats-an-omitted-gain-json-zero-as-full-volume-via-if
kind: note
note_kind: finding
created: 2026-08-10T18:48:38Z
created_by: a-root
origin: internal/timeline/timeline.go:97
---
# CONFIRMED 030: AddAudio treats an omitted gain (JSON zero) as full volume via 'if gain <= 0 { gain = 1 }', so a caller who omits gain gets a bed drowning the dialogue
