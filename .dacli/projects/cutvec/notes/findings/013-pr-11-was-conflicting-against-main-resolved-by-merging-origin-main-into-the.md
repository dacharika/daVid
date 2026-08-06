---
id: f-013-pr-11-was-conflicting-against-main-resolved-by-merging-origin-main-into-the
kind: note
note_kind: finding
created: 2026-08-06T08:33:59Z
created_by: a-fixer-9brkjz
about: "[[013]]"
severity: major
---
# 013 PR #11 was CONFLICTING against main; resolved by merging origin/main into the branch
PR #11 (branch dacli/013...) was mergeable=CONFLICTING/DIRTY because the branch (commit 7e46791) was cut before 012/render landed on main. Merged origin/main into the branch (merge commit f6b2334, parents 7e46791 + 143f9b9) and hand-resolved 4 source conflicts where 012(speakers)+render and 013(index) added parallel command surfaces: (1) internal/project/project.go — kept both Index type/methods and Speakers+Render types/methods; Status() now appends transcript+index+speakers per source plus render:proxy/final tiers. (2) internal/catalog/catalog.go — split the collapsed entry into two full tool entries (index, speakers). (3) cmd/cutvec/main.go — kept indexCmd (013) AND speakersCmd+renderCmd (012/render); note 013 refactored resolveSource to 3-arg (m,want,verb), so the grafted speakers call was updated to resolveSource(m,req.Source,"speakers"). (4) cmd/cutvec/main_test.go — TestHelperProcess switch now has all three worker cases (index, speakers, render). Uncached go build ./... exit 0; go test -count=1 ./... ALL ok; gofmt clean; describe lists all 9 commands incl index+speakers+render. Pushed -> PR #11 now mergeable=MERGEABLE; auto-merge already queued; CI 'Go build and test' in progress.
