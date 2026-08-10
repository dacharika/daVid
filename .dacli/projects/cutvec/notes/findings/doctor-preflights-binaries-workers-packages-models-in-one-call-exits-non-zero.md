---
id: f-doctor-preflights-binaries-workers-packages-models-in-one-call-exits-non-zero
kind: note
note_kind: finding
created: 2026-08-10T17:30:02Z
created_by: a-fixer-xrt529
about: "[[024]]"
severity: major
---
# doctor preflights binaries+workers+packages+models in one call, exits non-zero on required-missing, runs no worker
New: internal/doctor (doctor.go) is a Go-owned checklist evaluated through an injected Probe seam; doctorCmd + osProbe in cmd/cutvec/main.go (case 'doctor' in run()) wire it to the CLI and MCP (via callTool->run). Checklist: 5 binaries (python3, ffmpeg, ffprobe, whisper-cli, melt via exec.LookPath), 6 workers (CUTVEC_{FFMPEG,WHISPER,INDEX,SEARCH,MEDIAPIPE,MELT}_WORKER env set AND file exists), 5 packages (lancedb, mediapipe, numpy, cv2, fastembed|sentence_transformers via python3 importlib.util.find_spec), 2 models (CUTVEC_WHISPER_MODEL/CUTVEC_FACE_LANDMARKER or well-known home paths). AC1: each Check carries {name,kind,required,status:ok|missing,detail,hint}; every missing check has an actionable hint mirroring the worker's own install hint. Pinned by internal/doctor/doctor_test.go (TestRunReportsEveryKind, TestMissing*, TestEmbeddingBackendSatisfiedByEither, TestModelFoundAtWellKnownPath) and cmd/cutvec/main_test.go TestDoctorReportsEveryKind/TestDoctorInCatalog. AC2: report always emitted to stdout; when a REQUIRED check is missing doctorCmd returns axerror doctor.preflight_failed so run()->os.Exit(1); doctor invokes no worker and reads no project (pure inspection). Pinned by TestDoctorFailsWhenRequiredMissing (clears worker env, asserts exit-error + report still on stdout + requiredMissing>=6) and TestDoctorRejectsUnknownFlag (cli.unknown_flag). catalog gains the 'doctor' tool (describe now lists it). Gates: go build ./... ok; gofmt clean; go vet clean; staticcheck ./internal/doctor/... ./cmd/cutvec/... exit 0; go test -race -count=1 ./... ALL ok. Live e2e: built binary, 'cutvec doctor --json' on this host emitted the full report (binaries+models ok, unset workers+missing lancedb/mediapipe missing with hints) and exited 1 with doctor.preflight_failed on stderr.
