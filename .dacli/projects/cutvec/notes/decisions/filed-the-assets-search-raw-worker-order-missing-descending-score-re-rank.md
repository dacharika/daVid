---
id: d-filed-the-assets-search-raw-worker-order-missing-descending-score-re-rank
kind: note
note_kind: decision
created: 2026-08-10T19:33:29Z
created_by: a-go-auditor-dpd5dj
about: "[[006]]"
---
# Filed the assets-search raw-worker-order (missing descending-Score re-rank) determinism defect as the single highest-value evidence-based change
## Chose
Filed the assets-search raw-worker-order (missing descending-Score re-rank) determinism defect as the single highest-value evidence-based change
## Rejected
Re-filing any of the six sibling findings already queued as open tasks 029 (hook/caption overlap), 030 (audio bed gain), 031 (multi-clip sequencing), 032 (blank element), 033 (render atomic publish), 034 (shorts moment re-rank); or the applyAudio map-iteration determinism nit (rare, needs a tie, hard to pin deterministically)
## Because
All six obvious findings surfaced in the brief are now open tasks 029-034, so re-filing any is a near-duplicate dacli refuses (exit 3). The assets-search defect is distinct from every open (029-034) and done task (014 fixed transcript search's OWN ranking; 034 is the shorts/moment surface over project.Candidate), and provable from source with no melt/lancedb: searchCmd re-ranks via project.RankCandidates (main.go:985) but assetsSearchCmd appends hits in raw worker order (assets.go:248-262), never sorting by Score nor capping to req.Limit in Go. It hits the headline 'vector search reproduce a full podcast edit' promise on the library/SFX retrieval surface: two runs over one index can yield differently-ordered results when the worker order is unstable. It is unit-testable in Go by driving assetsSearchCmd with a fake search worker returning assets out of Score order (0.2 before 0.9) and asserting out.Assets[0].Score is 0.9; assets_test.go asserts nothing about ordering today, so it ships green.
