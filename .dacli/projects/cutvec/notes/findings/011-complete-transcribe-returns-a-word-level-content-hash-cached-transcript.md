---
id: f-011-complete-transcribe-returns-a-word-level-content-hash-cached-transcript
kind: note
note_kind: finding
created: 2026-08-05T23:48:54Z
created_by: a-fixer-sv7wx8
about: "[[011]]"
severity: major
---
# 011 complete: transcribe returns a word-level, content-hash-cached transcript handle; both ACs pass green
Uncached 'go build ./...' exit 0 and 'go test -count=1 ./...' ALL ok (cmd/cutvec, axerror, catalog, project, worker; timeline no test files); gofmt clean.

AC1 (handle + word-level start/end): 'cutvec transcribe <project> --json' returns {transcript(handle path),source(hash),words,language,cached} via transcribeCmd (cmd/cutvec/main.go:365) -> resolveSource -> whisper worker.Invoke(op=transcribe) -> transcriptFromResult -> project.WriteTranscript. Transcript file at proj/cache/transcribe-<hash>.json holds []Word{word,start,end} (internal/project/project.go Word/Transcript types). Proven by TestTranscribeWordLevel (cmd/cutvec/main_test.go), which reads the handle and asserts words[0]={hello 0 0.5}, words[1] start 0.5/end 1.0, via a re-exec'd fake whisper worker (TestHelperProcess, WORKER_MODE=transcribe) — no python/whisper on PATH.

AC2 (cached + idempotent): transcript is content-addressed by the source sha256 in cache/transcribe-<hash>.json (project.TranscriptPath/HasTranscript/Read/WriteTranscript). A re-run is a cache hit that runs NO worker. Proven by TestTranscribeCacheHitIdempotent (second run points at /nonexistent worker yet returns cached:true with an identical handle — the worker is never touched) and the unit-level TestTranscriptCacheRoundTrip (internal/project/project_test.go).

Also: catalog gains the 'transcribe' tool (describe --json lists 6 tools; schema transcribe emits a draft-2020-12 doc); status graph now includes per-source transcript:<id> artifacts marked done/pending; failure rules pinned by TestTranscribeFailureRules (input.invalid/cli.unknown_flag/project.uninitialized/input.empty) + TestTranscribeWorkerErrorMapsThrough (worker.missing). Cannot self-check acceptance boxes (owner-only); owner a-root to check n=1,n=2.
