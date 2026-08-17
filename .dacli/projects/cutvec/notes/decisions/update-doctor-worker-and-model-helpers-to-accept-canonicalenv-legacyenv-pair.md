---
id: d-update-doctor-worker-and-model-helpers-to-accept-canonicalenv-legacyenv-pair
kind: note
note_kind: decision
created: 2026-08-17T17:16:12Z
created_by: a-k4wj4rkn62
about: "[[047]]"
---
# update doctor worker() and model() helpers to accept canonicalEnv+legacyEnv pair; probe both via Probe seam
## Chose
update doctor worker() and model() helpers to accept canonicalEnv+legacyEnv pair; probe both via Probe seam
## Rejected
call envconfig.Lookup from inside the probe closure
## Because
envconfig.Lookup calls os.Getenv directly; the Probe seam exists precisely so doctor checks are testable without real env; the fallback logic must live inside the probe using p.Getenv for both keys
