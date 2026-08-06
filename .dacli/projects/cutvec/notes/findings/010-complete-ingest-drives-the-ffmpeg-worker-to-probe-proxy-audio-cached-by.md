---
id: f-010-complete-ingest-drives-the-ffmpeg-worker-to-probe-proxy-audio-cached-by
kind: note
note_kind: finding
created: 2026-08-05T23:50:59Z
created_by: a-fixer-andad9
about: "[[010]]"
severity: major
---
# 010 complete: ingest drives the ffmpeg worker to probe+proxy+audio, cached by content hash; both ACs pass live + in go test
cmd/cutvec/main.go ingestCmd now passes an ffmpegArtifactor{ctx} into project.Ingest(path, Artifactor). project.Ingest (internal/project/project.go) calls Artifactor.BuildArtifacts exactly once on a content-hash cache MISS and skips it on a HIT. The artifactor lazily resolves worker.Worker from CUTVEC_FFMPEG_WORKER/_ARGS, invokes worker op=ingest with {source,proxy,audio} paths under cache/, verifies both files were written, and returns Artifacts{Proxy,Audio,Probe}; project.Source gains proxy/audio/probe (json omitempty) recorded in project.json + cache/<hash>.json. AC1 (main_test.go TestIngestBuildsProxyAudioProbe): 'ingest --json' returns source.proxy, source.audio, and source.probe handles that resolve to real files the worker wrote, via a re-exec'd fake worker (TestFFmpegHelperProcess, no ffmpeg/python needed). AC2 (TestIngestCacheHitIdempotent + project_test.go TestIngestRunsArtifactorOncePerHash): re-ingesting identical bytes returns cached:true with the SAME handles and the artifactor/worker is NOT re-run; manifest holds exactly 1 source. Failure rules: no worker configured -> non-retryable worker.missing (TestIngestWithoutWorkerIsMissing); mapSourceErr passes structured worker axerrors through unchanged. Uncached 'go build ./...' exit 0 and 'go test -count=1 ./...' all packages ok. Live e2e with a python3 worker stand-in confirmed cached:false then cached:true and proxy/audio/json in cache/.
