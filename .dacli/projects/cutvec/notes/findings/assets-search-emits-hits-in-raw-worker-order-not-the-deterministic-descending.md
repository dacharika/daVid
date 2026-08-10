---
id: f-assets-search-emits-hits-in-raw-worker-order-not-the-deterministic-descending
kind: note
note_kind: finding
created: 2026-08-10T19:33:19Z
created_by: a-go-auditor-dpd5dj
about: "[[006]]"
severity: major
---
# assets search emits hits in raw worker order, not the deterministic descending-Score order search enforces
assetsSearchCmd (cmd/cutvec/assets.go:248-262) loops 'for _, raw := range list' over resp.Result["assets"] and appends each assetHit to out.Assets IN WORKER ORDER: no project.RankCandidates, no sort by Score, no Go-side cap to req.Limit (limit is only passed to the worker at assets.go:238). searchCmd DOES enforce the order at cmd/cutvec/main.go:985 'ranked := project.RankCandidates(hits)' (RankCandidates at internal/project/project.go:447-464 sorts descending Score with stable tiebreaks). So 'assets search' — the library/SFX/b-roll retrieval half of the headline 'vector search reproduce a full podcast edit' — trusts worker iteration order: if the worker returns hits unsorted or with tied scores in nondeterministic order, two identical asset searches over one index return differently-ordered results, breaking the 'deterministic idempotent resumable steps' promise. LATENT/GREEN: assets_test.go covers only describeAsset and parseOnePositional; no test asserts assets ordering. UNIT-TESTABLE with no melt/lancedb: drive assetsSearchCmd with a fake search worker returning assets out of Score order (0.2 before 0.9) and assert out.Assets[0].Score is the highest. FIX: sort out.Assets by descending Score with stable tiebreaks (Ref, Text) and cap to req.Limit in Go, mirroring searchCmd. Note assetHit has Ref/Text/Score (no Start/End), so it needs a local sort mirroring RankCandidates rather than reusing it verbatim. DISTINCT from open 034 (shorts/moment surface via project.Candidate) and done 014 (transcript search's own ranking); this is the assets library surface.
