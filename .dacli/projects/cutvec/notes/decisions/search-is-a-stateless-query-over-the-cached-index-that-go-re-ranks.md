---
id: d-search-is-a-stateless-query-over-the-cached-index-that-go-re-ranks
kind: note
note_kind: decision
created: 2026-08-06T08:38:40Z
created_by: a-fixer-fs1gss
about: "[[014]]"
---
# search is a stateless query over the cached index that Go re-ranks deterministically (score desc, then start/end/text tiebreak); no result caching, requires an existing index
## Chose
search is a stateless query over the cached index that Go re-ranks deterministically (score desc, then start/end/text tiebreak); no result caching, requires an existing index
## Rejected
Cache search results keyed on query+source hash like transcribe/index/render, and trust the worker's returned order
## Because
AC2 requires determinism, not caching: search is a read query whose durable input (the .lance table) is already content-hash cached by index. Go sorts the worker's candidates into a total order (score desc, then start,end,text) so a fixed query+index yields byte-identical ranked output regardless of worker iteration order; this is unit-testable via project.RankCandidates. Adding a result cache would need a second cache key (query+source) with no AC benefit and a footgun if the index changed under a stale key.
