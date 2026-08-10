---
id: d-a-summary-job-named-go-build-and-test-carries-the-repository-ruleset-s-required
kind: note
note_kind: decision
created: 2026-08-10T15:05:54Z
created_by: a-root
---
# A summary job named 'Go build and test' carries the repository ruleset's required check
## Chose
A summary job named 'Go build and test' carries the repository ruleset's required check
## Rejected
renaming the ruleset rule
## Because
splitting CI into per-concern jobs removed that check name and made every PR unmergeable while showing green; the rule is policy, the workflow should bend.
