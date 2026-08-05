---
id: f-pr-first-not-possible-repo-has-no-git-remote-work-is-committed-on-branch-dacli
kind: note
note_kind: finding
created: 2026-08-05T14:35:45Z
created_by: a-fixer-br37c8
about: "[[002]]"
severity: minor
---
# PR-first not possible: repo has no git remote; work is committed on branch dacli/002 for owner to accept/integrate locally
git remote -v is empty; 'dacli push' fails with 'origin does not appear to be a git repository' and 'dacli pr' fails with 'no git remotes found'. This matches the run rule reported for task 001 (PR-first off; owner runs 'dacli accept' then integrates into main). Task 002 is committed as fd7db57 on branch dacli/002-set-up-the-test-harness with both acceptance criteria satisfied (see finding test-harness-wired). Owner action: dacli accept 002 + integrate.
