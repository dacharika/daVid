---
id: f-all-three-acceptance-criteria-met-ci-green-gated-on-mlnomadpy-cutvec
kind: note
note_kind: finding
created: 2026-08-05T14:45:23Z
created_by: a-fixer-qm59fz
about: "[[003]]"
severity: major
---
# All three acceptance criteria met: CI green + gated on mlnomadpy/cutvec
AC1 (ci.yml committed on default branch, runs both commands): origin/main:.github/workflows/ci.yml runs 'go build ./...' (line 33) and 'go test ./...' (line 35); only change vs seed is go-version-file:'go.mod' pinning (deterministic toolchain), committed 2c3d204 and merged via PR #1 (merge 71c9aa4). AC2 (green once on default, linked): green runs on main — 31016580325 (initial), 31016665594 (merge of PR #1, pinned workflow, 27s success): https://github.com/mlnomadpy/cutvec/actions/runs/31016665594 . AC3 (PR shows CI check, red blocks merge): PR #1 https://github.com/mlnomadpy/cutvec/pull/1 shows the 'Go build and test' check in its status list; active repository ruleset id 20463625 (current_user_can_bypass=never) requires that check + a PR before merging main, so a red build blocks the merge (sibling PR #2 from task 004 also passed through this gate). Remote: mlnomadpy/cutvec (private); no git remote existed before this task.
