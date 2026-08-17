---
id: d-centralise-env-fallback-in-internal-envconfig-doctor-probes-both-david-and
kind: note
note_kind: decision
created: 2026-08-17T14:59:04Z
created_by: a-r862mrr8mk
about: "[[047]]"
---
# centralise env fallback in internal/envconfig; doctor probes both DAVID_* and CUTVEC_* via Probe seam
## Chose
centralise env fallback in internal/envconfig; doctor probes both DAVID_* and CUTVEC_* via Probe seam
## Rejected
inline os.Getenv calls per-command or a global map lookup
## Because
acceptance criteria require centralised lookup that does not pollute JSON stdout; doctor uses an injected Probe so env fallback must happen inside the probe closure, not by calling envconfig.Lookup which hits real os.Getenv
