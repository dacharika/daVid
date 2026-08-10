---
id: d-sequence-clips-in-timelinebuildcmd-when-len-producers-2-before-applyaudio
kind: note
note_kind: decision
created: 2026-08-10T21:22:26Z
created_by: a-fixer-tg912m
about: "[[031]]"
---
# Sequence clips in timelineBuildCmd when len(Producers)>=2, before applyAudio
## Chose
Sequence clips in timelineBuildCmd when len(Producers)>=2, before applyAudio
## Rejected
Always sequence (including single clip), or sequence only inside a new no-audio branch
## Because
Sequencing before applyAudio with the same 'main' id means AddAudio finds the tractor already built and its nil-guard no longer fires, so the audio path (mix tractor) is byte-identical for 2+ clips; a lone clip keeps needing no track structure (melt plays one producer as is), matching the acceptance which scopes to 2+; AddAudio's nil-tractor guard is kept as a safety net for the audio-into-a-bare-timeline path.
