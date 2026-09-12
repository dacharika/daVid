#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

stale=$(grep -RInE \
  --exclude-dir=.git \
  --exclude-dir=.dacli \
  --exclude='check_branding.sh' \
  'cmd/cutvec|build/cutvec|github\.com/mlnomadpy/cutvec|`cutvec (describe|init|ingest|status|transcribe|index|search|speakers|shorts|assets|filters|reframe-edit|timeline|render|doctor|mcp)' \
  . || true)

if [[ -n "$stale" ]]; then
  printf '%s\n' "stale canonical branding found:" "$stale" >&2
  exit 1
fi

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT
export GOCACHE="${GOCACHE:-$tmp_dir/go-cache}"
export GOMODCACHE="${GOMODCACHE:-$tmp_dir/go-mod-cache}"
go build -o "$tmp_dir/daVid" ./cmd/daVid

for command in describe schema; do
  args=("$command")
  [[ "$command" == schema ]] && args+=(render)
  if "$tmp_dir/daVid" "${args[@]}" | grep -qi 'cutvec'; then
    printf 'agent-visible output for `%s` contains the legacy name\n' "${args[*]}" >&2
    exit 1
  fi
done

printf 'canonical daVid branding verified\n'
