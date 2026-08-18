---
id: f-all-5-acs-verified-david-path-primary-cutvec-legacy-fallback-migration
kind: note
note_kind: finding
created: 2026-08-18T12:44:43Z
created_by: a-hb2p9mgjxq
about: "[[050]]"
severity: major
---
# all 5 ACs verified: daVid path primary, cutvec legacy fallback, migration recommendation, new tests, go/ruff clean
AC1: internal/doctor/doctor.go faceLandmarkerSpec() probes daVid path first (Library/Application Support/daVid/models/face_landmarker.task). AC2: legacy cutvec path (Library/Application Support/cutvec/models/face_landmarker.task) used as fallback when daVid path missing. AC3: advisory func emits migration recommendation when only cutvec path exists — Report.Recommendations field added to JSON output. AC4: doctor_test.go adds TestFaceLandmarkerFoundAtNewDaVidPath, TestFaceLandmarkerLegacyFallback, TestFaceLandmarkerMissingBothPaths, TestMigrationRecommendationEmittedWhenOnlyLegacyExists, TestNoMigrationRecommendationWhenNewPathExists. AC5: go build ./... and go test ./... both exit 0; ruff check workers/ passes. workers/mediapipe_worker.py MODEL_CANDIDATES updated with daVid path first, cutvec last.
