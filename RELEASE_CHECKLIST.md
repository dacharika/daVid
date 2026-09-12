# Release checklist

1. Confirm `git status --short` is empty and the tag points at reviewed `main`.
2. Run `gofmt -l .`, `go vet ./...`, `go test ./...`, and `go test -race ./...`.
3. Run `ruff check workers/`, `ruff format --check workers/`, and
   `python3 -m compileall -q workers/`.
4. Run `./scripts/render_proof.sh` to verify the actual FFmpeg/MLT render and
   atomic-publish path against generated video and audio.
5. Run `./workers/smoke_test.sh <owned-test-media> <python-with-ml-packages>` and
   retain the output plus ffprobe evidence for the rendered video.
6. Confirm `daVid describe`, `daVid schema render`, and MCP initialization all
   report the canonical daVid identity.
7. Push an annotated `v*` tag. The release workflow builds deterministic
   archives for Linux and macOS on amd64 and arm64, bundles the version-matched
   workers, generates checksums, and publishes the GitHub release.
8. Review the generated archives and checksums before announcing the release.
