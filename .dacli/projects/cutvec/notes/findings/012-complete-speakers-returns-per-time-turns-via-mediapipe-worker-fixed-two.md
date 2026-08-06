---
id: f-012-complete-speakers-returns-per-time-turns-via-mediapipe-worker-fixed-two
kind: note
note_kind: finding
created: 2026-08-06T08:08:43Z
created_by: a-fixer-72cctq
about: "[[012]]"
severity: major
---
# 012 complete: speakers returns per-time turns via mediapipe worker, fixed two-person layout, content-hash cached
AC1 (cutvec speakers <project> --json returns per-time speaker labels/turns): speakersCmd (cmd/cutvec/main.go) -> resolveSource -> mediapipe worker.Invoke(op=speakers, params{source,layout}) -> speakersFromResult -> project.WriteSpeakers. Returns {speakers(handle path),source(hash),layout,turns,cached}. Track file at proj/cache/speakers-<hash>.json holds Speakers{Source,Layout,[]Turn{speaker,start,end}} (internal/project/project.go). Proven by TestSpeakersTurns (cmd/cutvec/main_test.go): asserts turns[0]={left 0 1.5}, turns[1]={right 1.5 3.0} via a re-exec'd fake mediapipe worker (TestHelperProcess WORKER_MODE=speakers, no python/mediapipe on PATH). AC2 (fixed two-person layout): layout is the project.TwoPersonLayout constant set by the core (not a request field); the worker echoes back the layout it received and TestSpeakersTurns asserts both the result and recorded track carry 'two-person'. Cache/idempotency: content-addressed by source sha256 at project.SpeakersPath; HasSpeakers short-circuits a re-run to cached:true running NO worker (TestSpeakersCacheHitIdempotent points 2nd run at /nonexistent yet gets cached:true). status graph gains per-source speakers:<id> artifact; catalog gains the 'speakers' tool. Failure rules pinned by TestSpeakersFailureRules (input.invalid/cli.unknown_flag/project.uninitialized/input.empty) + TestSpeakersWorkerErrorMapsThrough (worker.missing). Unit-level TestSpeakersCacheRoundTrip (internal/project/project_test.go). Uncached go build ./... exit 0; go test -count=1 ./... ALL ok; gofmt clean. Live e2e with a shell worker confirmed cached:false then cached:true, layout 'two-person' round-tripping to the worker, and the speakers artifact done in status.
