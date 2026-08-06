---
id: f-index-builds-a-content-hash-cached-local-lancedb-index-of-transcript-segments
kind: note
note_kind: finding
created: 2026-08-06T08:24:48Z
created_by: a-fixer-e29tn6
about: "[[013]]"
severity: major
---
# index builds a content-hash-cached local LanceDB index of transcript segments via an embed worker; no network/API key
indexCmd (cmd/cutvec/main.go): parseProjectArgs -> proj.Load -> resolveSource -> transcript.missing precondition (index is built over transcript segments) -> HasIndex cache-hit short-circuit -> resolveIndexWorker (request.worker or CUTVEC_INDEX_WORKER) -> worker.Invoke(op=index, params{transcript,table}) -> os.Stat verifies the .lance table was written (worker.protocol otherwise) -> indexFromResult -> project.WriteIndex. AC1 (cutvec index <project> --json builds a local LanceDB index of transcript segments): metadata cached at cache/index-<hash>.json, vectors in sibling cache/index-<hash>.lance written by the worker; result {index,table,source,segments,dim,model,cached}. Proven by TestIndexBuildsLanceDBIndex (cmd/cutvec/main_test.go) via a re-exec'd fake embed worker (TestHelperProcess WORKER_MODE=index, no python/lancedb on PATH). AC2 (no network/API key): embeddings produced locally by the worker; TestIndexCacheHitIdempotent points the 2nd run at /nonexistent yet gets cached:true running NO worker. Unit-level TestIndexCacheRoundTrip (internal/project/project_test.go). project.go gains Index type + IndexPath/IndexTablePath/HasIndex/WriteIndex/ReadIndex, keyed 'index-'+sha256; status graph gains per-source index:<id> artifact; catalog gains the 'index' tool. Failure rules pinned by TestIndexFailureRules (input.invalid/cli.unknown_flag/project.uninitialized/input.empty) + TestIndexRequiresTranscript (transcript.missing) + TestIndexWorkerErrorMapsThrough (worker.missing). Uncached go build ./... exit 0; go test -count=1 ./... ALL ok; gofmt clean. Live e2e with a python3 embed worker confirmed cached:false then cached:true, .lance+.json in cache/, and the index artifact in status.
