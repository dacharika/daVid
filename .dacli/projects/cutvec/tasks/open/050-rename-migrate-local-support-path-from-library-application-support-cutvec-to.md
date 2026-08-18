---
id: t-01M0ADSTRQ87HYKWGME05TPPZ3
kind: task
created: 2026-08-18T12:33:37Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 2, probable: 3, pessimistic: 5}"
depends_on: "[049]"
---
# rename: migrate local support path from Library/Application Support/cutvec to daVid with legacy fallback
## So that
new installs find models under the canonical daVid path while existing cutvec model caches keep working through the deprecation window
## Acceptance
- [ ] mediapipe_worker.py and internal/doctor/doctor.go look in ~/Library/Application Support/daVid/models/ first
- [ ] when only the legacy ~/Library/Application Support/cutvec/models/ directory exists it is used as fallback
- [ ] daVid doctor --json emits a migration recommendation when only the legacy path exists
- [ ] doctor_test.go covers new-path lookup, legacy fallback, and missing-model cases
- [ ] go build ./... and go test ./... both exit 0; ruff checks pass
## Log
