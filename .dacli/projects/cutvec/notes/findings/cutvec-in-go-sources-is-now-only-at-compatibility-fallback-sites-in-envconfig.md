---
id: f-cutvec-in-go-sources-is-now-only-at-compatibility-fallback-sites-in-envconfig
kind: note
note_kind: finding
created: 2026-08-17T17:16:04Z
created_by: a-k4wj4rkn62
about: "[[047]]"
severity: minor
---
# CUTVEC_ in Go sources is now only at compatibility fallback sites in envconfig.Lookup and workerSpec.legacyEnv fields
grep CUTVEC_ over *.go shows only envconfig/envconfig.go, doctor.go (legacyEnv params), main.go (legacyEnv fields in workerSpec literals), ingest.go (envconfig.Lookup call), and workers.go (comment). No raw os.Getenv(CUTVEC_*) anywhere.
