---
id: d-keep-punchin-s-use-profile-crop-form-in-reframe-edit
kind: note
note_kind: decision
created: 2026-08-10T17:42:21Z
created_by: a-root
---
# Keep PunchIn's use_profile crop form in reframe-edit
## Chose
Keep PunchIn's use_profile crop form in reframe-edit
## Rejected
task 027's plan to strip use_profile and rewrite the passing test to match the source-pixel form
## Because
Isolated A/B on the real render path (melt <tl> -consumer avformat, same source, same instant, only the crop edges differing 0px vs 528/297px) shows the use_profile form visibly crops and fills. 027 generalised from a genuine finding — that combining center=1 WITH explicit edges yields black bars — to the false conclusion that use_profile edges never crop. Acting on it would have removed working code and rewritten a passing test to enshrine the wrong belief.
