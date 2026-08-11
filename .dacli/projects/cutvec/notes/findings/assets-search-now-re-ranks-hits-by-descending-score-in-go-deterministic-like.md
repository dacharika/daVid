---
id: f-assets-search-now-re-ranks-hits-by-descending-score-in-go-deterministic-like
kind: note
note_kind: finding
created: 2026-08-10T22:21:19Z
created_by: a-fixer-dkm65b
about: "[[035]]"
severity: major
---
# assets search now re-ranks hits by descending Score in Go, deterministic like transcript search
assetsSearchCmd (cmd/cutvec/assets.go:264-283) now sorts out.Assets with sort.SliceStable by descending Score, tiebreaking Ref then Text, and caps to req.Limit before emitJSON — mirroring searchCmd's project.RankCandidates (main.go:995-998). New tests in cmd/cutvec/main_test.go: TestAssetsSearchRanksHitsByScore drives assetsSearchCmd via a new 'search-assets' fake-worker mode in TestHelperProcess that returns hits out of Score order (whoosh 0.2 before boom 0.9) and asserts Assets[0] is the 0.9 boom; TestAssetsSearchLimitCaps asserts limit=1 keeps the top hit. Verified the rank test FAILS before the fix (top=whoosh 0.2) and PASSES after. go build ./... and go test ./... both exit 0.
