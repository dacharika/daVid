---
id: f-shorts-selects-its-moments-in-raw-worker-order-not-search-s-deterministic
kind: note
note_kind: finding
created: 2026-08-10T19:23:20Z
created_by: a-go-auditor-c30b9e
about: "[[006]]"
severity: major
---
# shorts selects its moments in raw worker order, not search's deterministic RankCandidates re-rank
shortsCmd never applies the deterministic re-rank that searchCmd relies on, so the shorts recipe picks and numbers its moments in the search worker's iteration order rather than descending-score order.

EVIDENCE (no melt render needed):
- searchCmd re-ranks for determinism: cmd/cutvec/main.go:985 'ranked := project.RankCandidates(hits)', with the comment at main.go:982-984 stating this is 'the determinism the acceptance criterion pins' (task 014).
- shortsCmd does NOT re-rank: cmd/cutvec/shorts.go:194 'candidates, _, err := searchFromResult(resp.Result)', then shorts.go:202-204 caps with 'candidates = candidates[:req.Count]' and shorts.go:213 'for i, c := range candidates' builds one short per candidate IN WORKER ORDER, stamping short.Rank = i+1 (shorts.go:252) and output/caption filenames fmt.Sprintf("%02d-%s", i+1, who) (shorts.go:223).
- searchFromResult returns candidates verbatim in worker order (cmd/cutvec/main.go:1046-1062: plain json.Unmarshal, no sort).
- RankCandidates (internal/project/project.go:447-464) is the stable descending-score total order search uses; shorts never calls it (grep RankCandidates: only main.go + project.go).

IMPACT: 'search' and 'shorts' disagree on which moments are top and in what order. The rank numbers and the 01-left.mp4/02-right.mp4 filenames do not reflect score order. If the worker returns candidates unsorted, with tied scores in nondeterministic order, or returns more than the requested limit, two shorts runs over the same index yield different clips - breaking the headline 'deterministic idempotent resumable steps' promise on the shorts recipe. reframe-edit is unaffected (it cuts on the speaker track, not search).

LATENT/GREEN: the fake search worker deliberately returns candidates OUT of ranked order - {score:0.2} before {score:0.9} (cmd/cutvec/main_test.go:81-92) - precisely to prove search re-ranks; yet no shorts test asserts moment ordering (shorts_test.go covers only helpers: speakerAt/buildSRT/buildShortTimeline/snapToSentence; there is no shorts end-to-end ordering test). So the defect ships green.

UNIT-TESTABLE with no melt: drive shortsCmd --plan with the existing out-of-order fake search worker and assert out.Shorts[0] is the highest-scoring candidate (0.9), not the worker's first (0.2).

FIX: apply project.RankCandidates to candidates before the [:req.Count] cap in shortsCmd, mirroring searchCmd's one-liner - reusing the same primitive rather than trusting worker order.

DISTINCT from open 029 (hook/caption overlap), 030 (audio gain), 031 (multi-clip sequencing), 032 (blank element), 033 (render atomic publish); and from done 014 (search's own ranking), 019 (shorts recipe), 023 (head-and-shoulders framing).
