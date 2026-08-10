---
id: d-filed-the-shorts-raw-worker-order-missing-rankcandidates-re-rank-determinism
kind: note
note_kind: decision
created: 2026-08-10T19:23:31Z
created_by: a-go-auditor-c30b9e
about: "[[006]]"
---
# Filed the shorts raw-worker-order (missing RankCandidates re-rank) determinism defect as the single highest-value evidence-based change
## Chose
Filed the shorts raw-worker-order (missing RankCandidates re-rank) determinism defect as the single highest-value evidence-based change
## Rejected
Re-filing any of the five sibling findings already queued as open tasks 029 (hook/caption overlap), 030 (audio bed gain), 031 (multi-clip sequencing), 032 (blank element), 033 (render atomic publish); re-filing the refuted reframe PunchIn no-op; or the applyAudio/speakerAt map-iteration determinism nit (rare, needs a tie, hard to pin deterministically)
## Because
All five obvious findings from the brief are now open tasks 029-033, so re-filing any is a near-duplicate dacli refuses. This defect is distinct from every open (029-033) and done task (014 fixed search's OWN ranking; 019 shipped the shorts recipe; 023 the framing) and is provable from source with no melt render: searchCmd re-ranks via project.RankCandidates (main.go:985, comment 982-984 calling it the pinned determinism) but shortsCmd takes searchFromResult candidates in raw worker order (shorts.go:194) and caps [:req.Count] (shorts.go:202-204), building/numbering shorts by that order (shorts.go:213,223,252) and never calling RankCandidates. It hits the headline 'deterministic idempotent resumable steps' promise on the shorts recipe: two runs over one index can yield different clips when the worker's order is unstable or returns more than the limit. It is unit-testable in Go by driving shortsCmd --plan with the EXISTING out-of-order fake search worker (main_test.go:81-92, which returns 0.2 before 0.9 to prove search re-ranks) and asserting Shorts[0] is the 0.9 candidate; no shorts test asserts ordering today, so it ships green.
