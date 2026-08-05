---
id: f-unanchored-gitignore-binary-pattern-can-swallow-the-cmd-entry-point
kind: note
note_kind: finding
created: 2026-08-05T14:27:26Z
created_by: a-7p3edw
about: "[[001]]"
severity: minor
---
# Unanchored gitignore binary pattern can swallow the cmd entry point
An unanchored 'cutvec' line in .gitignore (.gitignore:4) matched the cmd/cutvec/ source directory, silently excluding cmd/cutvec/main.go from git add. Fixed by anchoring to '/cutvec' (root-only). Lesson for later tasks: name any binary-ignore pattern with a leading slash so it never collides with a same-named source dir.
