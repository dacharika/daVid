---
id: f-render-transcribe-speakers-emit-jsonl-progress-on-stderr-stdout-still-one-json
kind: note
note_kind: finding
created: 2026-08-10T17:48:14Z
created_by: a-fixer-3z558s
about: "[[025]]"
severity: major
---
# render/transcribe/speakers emit JSONL progress on stderr; stdout still one JSON result
New internal/progress (progress.go): Reporter emits one JSON object per line {kind:progress,step,phase,seq,elapsed_s,detail} via progress.While(w,step,interval,fn) which brackets fn with a start event, a ticker-driven heartbeat every interval (default 2s, override CUTVEC_PROGRESS_INTERVAL), and a done/error close. cmd/cutvec/main.go: transcribeCmd/speakersCmd/renderCmd gained a stderr io.Writer param; each wraps its single w.Invoke in progress.While(stderr,step,progressInterval(),...). run() gained an optional variadic stderr (resolves to io.Discard when absent) so all 41 existing 3-arg run() test callers still compile; main() passes os.Stderr, mcp.go callTool passes os.Stderr (progress rides the MCP server's log channel, never the JSON-RPC stdout). AC1 pinned by TestTranscribeEmitsProgress/TestSpeakersEmitsProgress/TestRenderEmitsProgress (assertStderrProgress: kind=progress, step match, start..done, strictly increasing seq). AC2 pinned by assertOneJSONResult (stdout decodes to exactly one JSON value, io.EOF after) + TestCacheHitEmitsNoProgress (cache hit runs no worker, emits nothing). Unit tests in internal/progress/progress_test.go cover framing, clock-seam elapsed, heartbeat-while-running, error close, nil-writer no-op. Gates: gofmt clean, go vet clean, staticcheck clean, go test -race ./... all ok. Live e2e: built binary, render with a 1.2s sleeping shell worker + CUTVEC_PROGRESS_INTERVAL=300ms wrote start+4 heartbeats+done to stderr while stdout held exactly one JSON render result.
