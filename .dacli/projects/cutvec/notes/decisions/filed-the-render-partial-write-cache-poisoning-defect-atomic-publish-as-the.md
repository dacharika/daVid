---
id: d-filed-the-render-partial-write-cache-poisoning-defect-atomic-publish-as-the
kind: note
note_kind: decision
created: 2026-08-10T19:05:12Z
created_by: a-go-auditor-k6rpnh
about: "[[006]]"
---
# Filed the render partial-write cache-poisoning defect (atomic publish) as the single highest-value evidence-based change
## Chose
Filed the render partial-write cache-poisoning defect (atomic publish) as the single highest-value evidence-based change
## Rejected
Re-filing any of the four sibling findings already queued (029 hook/caption overlap, 030 audio bed gain, 031 multi-clip sequencing, 032 blank element); re-filing the reframe-edit PunchIn no-op (refuted by verified d-keep-punchin's A/B and enshrined by task 027's close); or the applyAudio map-iteration determinism nit (rare, needs 2+ distinct producer targets, hard to pin deterministically)
## Because
The four obvious findings from the brief were ALL already filed as open tasks 029-032, so re-filing any would be a near-duplicate dacli refuses. The render cache-poisoning defect is distinct from every open (029-032) and done task (017 was render's happy path), grounded and provable from source with NO melt render: renderCmd passes the FINAL cache path proj.RenderPath(hash,quality) straight to the worker as its output (main.go:1357,1364), returns worker errors without removing a partial file (main.go:1367-1369), and HasRender is a bare os.Stat existence check (project.go:581-584) — so a melt that dies mid-encode leaves a truncated mp4 the next resumed run returns as Cached:true and never re-renders (main.go:1332-1340). It is render-SPECIFIC: transcribe/index/speakers have the worker RETURN its result and Go writes the cache only after full success (transcribe main.go:590-597), so they re-run cleanly on failure; render is the only step whose worker streams to the cache path. It hits the headline 'deterministic idempotent resumable steps' promise on the longest/most crash-prone step, exactly the scenario resumability exists for. It is unit-testable in Go by extending the existing fake render worker (main_test.go:114-128) with a partial-write-then-exit-1 mode, and the current TestRenderCacheHitIdempotent (main_test.go:1179) only covers the happy path, so the defect ships green.
