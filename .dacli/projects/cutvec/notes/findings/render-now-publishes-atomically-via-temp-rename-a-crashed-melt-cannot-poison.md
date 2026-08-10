---
id: f-render-now-publishes-atomically-via-temp-rename-a-crashed-melt-cannot-poison
kind: note
note_kind: finding
created: 2026-08-10T21:39:51Z
created_by: a-fixer-jcyhnt
about: "[[033]]"
severity: major
---
# render now publishes atomically via temp+rename; a crashed melt cannot poison the cache
renderCmd (cmd/cutvec/main.go:~1367) now renders to out+'.tmp', removes any stale tmp first, and os.Rename()s onto proj.RenderPath(hash,quality) ONLY after the worker returns success AND the worker.protocol 'did the worker write the output' os.Stat check passes on the tmp path. On any failure (worker error, missing output, or rename error) the tmp is os.Remove()d, so a worker that dies mid-encode leaves NO file at the cache path — HasRender stays false and the next resumed run re-renders. New test TestRenderPartialWriteDoesNotPoisonCache (cmd/cutvec/main_test.go) drives a new 'render-partial' fake-worker mode that writes truncated bytes then exits non-zero; it asserts the first render errors, HasRender(hash,'proxy')==false, the .tmp is gone, and a second render with the working worker is Cached:false and writes the complete output. Verified the test FAILS on old main.go (HasRender=true) and PASSES with the fix. All existing render tests (Produces/CacheHit/ProxyAndFinal/FailureRules) still pass. go build ./... and go test ./... exit 0; gofmt-clean.
