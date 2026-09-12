#!/bin/zsh
# End-to-end proof that the workers do real work: init -> ingest -> transcribe ->
# index -> search -> speakers against real media, through the real CLI.
#
# It asserts on the *substance*, not just the exit code: a transcript with words,
# an index with a non-zero dimension, a search hit, a speaker turn. A stubbed
# worker fails here even though it would compile and pass a unit test.
#
#   ./workers/smoke_test.sh <media-file> [python-with-ml-packages]
set -u
MEDIA=${1:?usage: smoke_test.sh <media-file> [python]}
PY=${2:-python3}
ROOT=${0:a:h:h}
W=$ROOT/workers
PROJ=$(mktemp -d)/proj
CLI="go run $ROOT/cmd/daVid"
fails=0

step() { print -n "  $1 … " }
ok()   { print "ok — $1" }
bad()  { print "FAIL — $1"; fails=$((fails+1)) }
req()  { print -n "{\"project\":\"$PROJ\",\"worker\":\"$PY\",\"workerArgs\":[\"$1\"]${2:-}}" }

print "daVid worker smoke test"
print "  media:   $MEDIA"
print "  python:  $PY"
print "  project: $PROJ"

step init
print "{\"project\":\"$PROJ\"}" | eval $CLI init >/dev/null 2>&1 \
  && ok "project created" || bad "init"

step ingest
width=$(print "{\"project\":\"$PROJ\",\"source\":\"$MEDIA\",\"worker\":\"$PY\",\"workerArgs\":[\"$W/ffmpeg_worker.py\"]}" \
  | eval $CLI ingest 2>/dev/null \
  | grep -oE '"width": *[0-9]+' | grep -oE '[0-9]+')
[[ -n ${width:-} && $width -gt 0 ]] && ok "source ingested, probed ${width}px wide" \
  || bad "ingest did not probe dimensions"

step transcribe
words=$(req "$W/whisper_worker.py" | eval $CLI transcribe $PROJ --json 2>/dev/null \
  | grep -oE '"words": *[0-9]+' | grep -oE '[0-9]+')
[[ -n ${words:-} && $words -gt 0 ]] && ok "$words words" || bad "no words transcribed"

step index
idx=$(req "$W/index_worker.py" | eval $CLI index $PROJ --json 2>/dev/null)
segs=$(print -r -- $idx | grep -oE '"segments": *[0-9]+' | grep -oE '[0-9]+')
dim=$(print -r -- $idx | grep -oE '"dim": *[0-9]+' | grep -oE '[0-9]+')
[[ -n ${segs:-} && $segs -gt 0 && -n ${dim:-} && $dim -gt 0 ]] \
  && ok "$segs segments, dim $dim" || bad "index built nothing"

step search
hits=$(req "$W/search_worker.py" ",\"query\":\"what is this about\",\"limit\":3" \
  | eval $CLI search $PROJ --json 2>/dev/null | grep -c '"score"')
[[ ${hits:-0} -gt 0 ]] && ok "$hits candidates" || bad "search returned nothing"

step speakers
turns=$(req "$W/mediapipe_worker.py" | eval $CLI speakers $PROJ --json 2>/dev/null \
  | grep -oE '"turns": *[0-9]+' | grep -oE '[0-9]+')
[[ -n ${turns:-} && $turns -gt 0 ]] && ok "$turns turns" || bad "no speaker turns"

step render
print "{\"project\":\"$PROJ\",\"clips\":[{\"resource\":\"$MEDIA\",\"id\":\"clip1\"}]}" \
  | eval $CLI timeline >/dev/null 2>&1
out=$(req "$W/melt_worker.py" ",\"quality\":\"proxy\"" \
  | eval $CLI render $PROJ --json 2>/dev/null \
  | grep -oE '"output": *"[^"]+"' | sed 's/.*"output": *"//;s/"$//')
if [[ -n ${out:-} && -s ${out:-/nonexistent} ]]; then
  dur=$(ffprobe -v error -show_entries format=duration -of default=nk=1:nw=1 "$out" 2>/dev/null)
  ok "wrote a ${dur:-?}s video"
else
  bad "render produced no video"
fi

step "cache is idempotent"
cached=$(print "{\"project\":\"$PROJ\",\"worker\":\"/nonexistent\"}" \
  | eval $CLI transcribe $PROJ --json 2>/dev/null | grep -oE '"cached": *true')
[[ -n ${cached:-} ]] && ok "re-run hit the cache without starting a worker" \
  || bad "cache did not hit"

print ""
if [[ $fails -eq 0 ]]; then print "all steps did real work"; else print "$fails step(s) failed"; fi
exit $fails
