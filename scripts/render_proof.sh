#!/usr/bin/env bash
set -euo pipefail

for tool in ffmpeg ffprobe melt python3 jq; do
  command -v "$tool" >/dev/null || {
    printf 'missing required proof tool: %s\n' "$tool" >&2
    exit 1
  }
done

repo=$(cd "$(dirname "$0")/.." && pwd)
proof=$(mktemp -d)
trap 'rm -rf "$proof"' EXIT
export GOCACHE="${GOCACHE:-$proof/go-cache}"
export GOMODCACHE="${GOMODCACHE:-$proof/go-mod-cache}"

ffmpeg -hide_banner -loglevel error -y \
  -f lavfi -i testsrc2=size=640x360:rate=24 \
  -f lavfi -i sine=frequency=440:sample_rate=48000 \
  -t 3 -c:v libx264 -pix_fmt yuv420p -c:a aac "$proof/source.mp4"

go build -o "$proof/daVid" "$repo/cmd/daVid"
printf '{"project":"%s/project","name":"render-proof"}\n' "$proof" |
  "$proof/daVid" init >/dev/null
printf '{"project":"%s/project","source":"%s/source.mp4","worker":"python3","workerArgs":["%s/workers/ffmpeg_worker.py"]}\n' \
  "$proof" "$proof" "$repo" | "$proof/daVid" ingest >/dev/null
printf '{"project":"%s/project","clips":[{"resource":"%s/source.mp4","id":"clip1"}]}\n' \
  "$proof" "$proof" | "$proof/daVid" timeline >/dev/null
printf '{"project":"%s/project","worker":"python3","workerArgs":["%s/workers/melt_worker.py"],"quality":"proxy"}\n' \
  "$proof" "$repo" | "$proof/daVid" render "$proof/project" --json >"$proof/result.json"
output=$(jq -r .output "$proof/result.json")
[[ -s "$output" ]] || { printf 'render produced no output\n' >&2; exit 1; }
[[ $(find "$proof/project" -name '*.tmp' -type f | wc -l | tr -d ' ') == 0 ]] || {
  printf 'render left temporary files behind\n' >&2
  exit 1
}

ffprobe -v error \
  -show_entries stream=codec_type,codec_name \
  -show_entries format=duration,size \
  -of json "$output"
