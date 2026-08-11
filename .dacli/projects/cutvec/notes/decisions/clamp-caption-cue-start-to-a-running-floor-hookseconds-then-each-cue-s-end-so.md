---
id: d-clamp-caption-cue-start-to-a-running-floor-hookseconds-then-each-cue-s-end-so
kind: note
note_kind: decision
created: 2026-08-11T09:08:14Z
created_by: a-fixer-aj37ya
about: "[[029]]"
---
# Clamp caption cue start to a running floor (hookSeconds, then each cue's end) so buildSRT emits strictly non-overlapping cues
## Chose
Clamp caption cue start to a running floor (hookSeconds, then each cue's end) so buildSRT emits strictly non-overlapping cues
## Rejected
Shifting only the first caption's start past 2.5s, or dropping words that fall under the hook
## Because
A running floor generalizes non-overlap to every consecutive cue pair (the AC), clears the hook for the opening card without discarding caption text, and a minCaptionShow bump keeps a pushed cue readable instead of a zero-length flash; shifting only cue #2 leaves later word-span overlaps unhandled.
