# cutvec

Agent-native video-editing CLI **and** MCP server. cutvec reproduces a full
podcast edit — proxy, transcribe, active-speaker reframe, vertical shorts with
hook and karaoke captions, color grade, denoise, SFX — as **MLT XML timelines**
an agent can mutate, plus **vector search** over the transcript. It is built for
AI agents, not humans: JSON in, JSON out; self-describing schemas; deterministic,
idempotent, resumable steps; and a `--plan` dry-run on every mutating operation.

## Quick start

From a clean checkout — no dependencies beyond the Go toolchain:

```sh
go build ./...        # exits 0
go test ./...         # exits 0
```

Build the binary and list the tool catalog:

```sh
go build -o build/cutvec ./cmd/cutvec
./build/cutvec describe    # emits the tool catalog + JSON Schemas as JSON
```

`describe` is the entry point every agent starts from: one self-describing
catalog backs both the CLI and the MCP server, so the two surfaces never drift.

## Project state: init, ingest, status

A project is a directory holding a `project.json` manifest plus a content-hash
`cache/`. Three tools model its state so every step is resumable and an agent can
read the whole graph in one call:

```sh
echo '{"project":"proj","name":"podcast"}' | ./build/cutvec init
# → {"project":"proj","manifest":"proj/project.json","name":"podcast","created":true}

echo '{"project":"proj","source":"raw.mp4"}' | ./build/cutvec ingest
# → {"source":{"id":"8b1faca47fef","path":"raw.mp4","hash":"8b1f…",
#              "probe":{"width":3840,"height":2160,"duration":20.03,"fps":59.94},
#              "proxy":"proj/cache/proxy-8b1f….mp4","audio":"proj/cache/audio-8b1f….wav"}}
echo '{"project":"proj","source":"raw.mp4"}' | ./build/cutvec ingest   # re-run
# → {"source":{…},"cached":true}   ← content-hash cache hit; nothing re-recorded

echo '{"project":"proj"}' | ./build/cutvec status --json
# → {"project":"proj","name":"podcast","sources":[…],"artifacts":[{"name":"timeline","status":"pending"}]}
```

`init` is idempotent (re-running returns `created:false`, never clobbering).
`ingest` keys each source on the sha256 of its bytes: re-ingesting an unchanged
file is a **cache hit** (`cached:true`) that records nothing — the step is
deterministic, idempotent, and resumable. On the miss it also measures the
source and builds its derived media through the ffmpeg worker: a small
fast-seeking **proxy**, and the **audio track** on its own. Both are handles
keyed on the content hash, so the cache hit returns them without rebuilding — on
a 379 MB 4K clip that is 6.7s the first time and 0.16s every time after. With no
ffmpeg worker configured the source is still recorded, just without probe or
handles. `status` returns the state graph — the
sources ingested and each artifact marked `done`/`pending` — so an agent decides
what to run next from one JSON read. Every mutating tool takes `--plan`.

Failure rules: empty stdin returns `input.empty`; a missing `project`/`source`
returns `input.invalid`; ingesting or statusing a directory with no manifest
returns `project.uninitialized`; a missing source file returns `source.missing` —
each the structured `{code, hint, retryable}` shape on stderr with a non-zero exit.

## Transcribe: word-level transcript, cached

`transcribe` turns an ingested source into a **word-level transcript** — each
token with a `start`/`end` in seconds — so an agent can search the audio and
build karaoke captions. It runs the whisper worker (a subprocess speaking the
versioned JSON contract; point at it with `"worker"` or the
`CUTVEC_WHISPER_WORKER` env var) and caches the result keyed by the source's
content hash, so a re-run is a **cache hit** that invokes no worker:

```sh
./build/cutvec transcribe proj --json < /dev/null
# → {"transcript":"proj/cache/transcribe-<hash>.json","source":"<hash>","words":812,"language":"en","cached":false}
./build/cutvec transcribe proj --json < /dev/null   # re-run
# → {"transcript":"proj/cache/transcribe-<hash>.json",…,"cached":true}   (no worker ran)
```

The project may be a positional argument (`transcribe proj`) or `"project"` in a
JSON request; `"source"` (a path, id, or hash) selects which ingested source when
the project has more than one. The result is a **handle** to the transcript file,
not the transcript blob. `--plan` reports the handle and cache status without
running the worker. Failure rules: a project with no ingested sources returns
`input.empty`; an unknown source returns `source.missing`; no configured worker
returns `worker.missing`; a worker that never started, crashes, or speaks the
wrong contract version maps to `worker.missing`/`worker.crashed`/`worker.version`.

## Search: semantic query to ranked timecodes

`search` answers a natural-language query with **ranked, timecoded candidates** —
each `{start, end, text, score}`, most-relevant first — so an agent can ask for a
moment by meaning and get back where it is. It searches the source's vector index
(built by `index`, so a source must be indexed first, else `index.missing`),
running a local search worker (`"worker"` in the request or the
`CUTVEC_SEARCH_WORKER` env var) that embeds the query and returns scored hits. The
core then re-ranks those hits into a **deterministic** total order — descending
score, then `start`/`end`/`text` as stable tiebreaks — so a fixed query+index
always yields the identical ranking, whatever order the worker reported them in:

```sh
./build/cutvec search proj "the funniest moment" --json < /dev/null
# → {"query":"the funniest moment","source":"<hash>","results":[
#      {"start":132.0,"end":137.5,"text":"…","score":0.88},
#      {"start":54.2,"end":58.0,"text":"…","score":0.71}],"count":2,"model":"…"}
```

The project and query may be positional arguments (`search proj "<query>"`) or
`"project"`/`"query"` in a JSON request; `"source"` (a path, id, or hash) selects
which indexed source when the project has more than one, and `"limit"` caps how
many ranked candidates come back. Unlike the build steps, search caches nothing —
it is a read run on demand. `--plan` echoes the query and source without running
the worker. Failure rules: an empty query returns `input.empty`; an un-indexed
source returns `index.missing`; no configured worker returns `worker.missing`.

## First feature slice: build a timeline

The `timeline` tool is the first edit operation carried end to end. It reads a
JSON clip list on stdin, builds the MLT XML timeline — the artifact an agent
mutates — writes it into the project directory, and returns a **handle** (a
path), not the XML blob:

```sh
echo '{"project":"proj","clips":[{"id":"cam1","resource":"raw.mp4"},{"resource":"broll.mp4"}]}' \
  | ./build/cutvec timeline
# → {"timeline":"proj/timeline.mlt","producers":2}
```

Add `--plan` for a dry-run that reports what it would write and touches nothing
(`{"planned":true}`). Failure and empty input have written rules: empty stdin or
an empty `clips` list returns `input.empty`; a missing `project`, an empty
`resource`, or malformed JSON returns `input.invalid` — every failure is the
structured `{code, hint, retryable}` shape on stderr with a non-zero exit.

## Render: timeline to video, cached per quality

`render` turns a project's built MLT timeline into an output video with the melt
worker (a subprocess speaking the versioned JSON contract; point at it with
`"worker"` in the request or the `CUTVEC_MELT_WORKER` env var). The result is
keyed on the timeline's content hash **and** the quality tier, so proxy and final
renders coexist and a re-run at the same tier is a **cache hit** that invokes no
worker. Choose the tier with `--proxy` (fast draft, the default) or `--final`
(full quality):

```sh
./build/cutvec render proj --json < /dev/null
# → {"output":"proj/cache/render-<hash>-proxy.mp4","timeline":"proj/timeline.mlt","source":"<hash>","quality":"proxy","cached":false}
./build/cutvec render proj --final --json < /dev/null
# → {"output":"proj/cache/render-<hash>-final.mp4",…,"quality":"final","cached":false}
./build/cutvec render proj --json < /dev/null   # re-run at proxy
# → {"output":"proj/cache/render-<hash>-proxy.mp4",…,"cached":true}   (no worker ran)
```

The project may be a positional argument (`render proj`) or `"project"` in a JSON
request; the returned `output` is a **handle** to the rendered file, not the video
blob. `--plan` reports the handle and cache status without running the worker.
Failure rules: choosing both `--proxy` and `--final`, an unknown quality, or a
missing/too-many project returns `input.invalid`; an uninitialised directory
returns `project.uninitialized`; a project with no built timeline returns
`timeline.missing`; no configured worker returns `worker.missing`; and a worker
that crashes or does not write the output maps to `worker.crashed`/`worker.protocol`.

## What actually reads the proxy

A cached artifact nobody reads is just disk. Two steps read the proxy, and the
numbers are why — both measured on a 20s 4K clip:

| step | reads | effect |
|---|---|---|
| `speakers` | the proxy | detection **9.40s → 4.15s**, attribution **96.3% → 100%** |
| `shorts` / `reframe-edit` at `quality: proxy` | the proxy | draft render **11.66s → 4.31s** |
| the same at `quality: final` | the master | unchanged — a delivery render must keep the resolution |
| `transcribe` | the master | unchanged, deliberately (see below) |

Detection gets *better* on the proxy, not just faster: downscaling hands the
detector a frame nearer the size it expects instead of a 4K one it has to fit.
Seat positions from both agree to within 0.002 normalised (~6px at 4K).

Substituting the proxy is only safe where the maths is resolution-independent.
Seats are normalised and `reframe-edit`'s punch-in works in profile space, so
both travel unchanged. `shorts` crops in **source pixels**, so its resource and
its dimensions are chosen together by one function — pairing the proxy with the
master's dimensions renders a perfectly valid video of the wrong thing (the
uncropped wide shot, letterboxed, no error). A proxy that is missing or was
recorded without dimensions falls back to the master.

`transcribe` still decodes the master, on purpose: reusing the cached audio was
measured at **0.885s** saved on a 23 GB source, because ffmpeg reads only the
audio stream and never decodes video. Not worth the coupling.

## Speakers: who is talking, when

`speakers` runs the mediapipe worker over an ingested source and returns a track
of **active-speaker turns** against a fixed two-person layout — the input the
reframe and shorts recipes cut on. Like the other analysis steps it is keyed on
the source's content hash, so a re-run is a cache hit that invokes no worker:

```sh
./build/cutvec speakers proj --json < /dev/null
# → {"speakers":"proj/cache/speakers-<hash>.json","source":"<hash>","turns":184,
#    "layout":"two-person","cached":false}
```

The returned `speakers` is a handle to the cached turns, not the turn list. Point
the worker with `"worker"` or `CUTVEC_MEDIAPIPE_WORKER`; `"source"` selects which
ingested source when the project holds more than one.

## Reframe: one locked-off wide shot, cut like two cameras

`reframe-edit` is a recipe, not a primitive: it takes a single static wide shot
and produces an edit that looks multi-camera — punching in on whoever is
speaking, falling back to the wide two-shot when nobody is — then renders it. It
needs a source with a speaker track (run `speakers` first).

```sh
./build/cutvec reframe-edit proj --json < /dev/null
# → {"output":"proj/cache/render-<hash>-final.mp4","timeline":"proj/reframe.mlt",
#    "duration":1243.0,"shots":[{"start":0,"end":8.4,"shot":"wide"},
#                               {"start":8.4,"end":21.7,"shot":"left"},…]}
```

`shots` is the **gapless** shot list the edit cuts through, so the plan is
auditable before the pixels exist. `keep` sets how much frame width a single
holds (smaller punches tighter), `minShot` sets the shortest shot so the edit
does not ping-pong on brief interjections, and `leftCentre`/`rightCentre`
(0.28/0.72 by default) say where the seats are. `--plan` writes the timeline and
returns the shot list without rendering.

## Shorts: query to vertical captioned clips

`shorts` is the other recipe, and it chains the whole stack: it searches the
index for moments matching a query, frames head-and-shoulders on whoever is
speaking with a crop window computed from the real source dimensions and the
detected seat, burns word-timed captions, and renders each clip vertical. It
needs an indexed, transcribed source with a speaker track.

```sh
./build/cutvec shorts proj "the funniest moment" --json < /dev/null
# → {"query":"the funniest moment","shorts":[
#      {"rank":1,"score":0.88,"start":132.0,"end":137.5,
#       "captions":"proj/cache/shorts/short-1.srt",
#       "output":"proj/cache/shorts/short-1.mp4"},…]}
```

Moments are ranked into the same deterministic order `search` uses, so shorts and
search agree on the top moments and two runs over one index cannot yield
different clips. `count` sets how many to cut (3 by default), `padding` adds
headroom either side of a matched moment, `hook` puts a caption card over the
opening seconds, and `quality` picks `proxy` or `final`. Under `--plan` the
timelines and captions are written but nothing renders.

## Filters: discover what this machine's MLT can do

Guessing an MLT service name is the fastest way to a failed render. `filters`
searches the installed registry — several hundred entries, including the whole
avfilter set — and expands a named filter's parameters, so an agent composes
`timeline` effects from what is actually present:

```sh
echo '{"query":"denoise","limit":3}' | ./build/cutvec filters
# → {"count":3,"filters":[
#      {"name":"avfilter.atadenoise","description":"Apply an Adaptive Temporal Averaging Denoiser."},
#      {"name":"avfilter.owdenoise","description":"Denoise using wavelets."},
#      {"name":"avfilter.vaguedenoiser","description":"Apply a Wavelet based Denoiser."}],
#    "presets":["denoise","denoise-strong","grade","loudness","warm"]}

echo '{"name":"avfilter.curves"}' | ./build/cutvec filters
# → {"count":1,"filters":[{"name":"avfilter.curves","description":"Adjust components curves.",
#      "params":[{"name":"av.preset","type":"string","description":"select a color curves preset"},…]}]}
```

`presets` are cutvec's own named chains — `grade`, `denoise`, `denoise-strong`,
`loudness`, `warm` — usable directly as a `timeline` effect without naming a
service. Parameters are only expanded for a focused lookup, so a broad search
stays small enough to read.

## Assets: a searchable library of sounds and b-roll

`assets` is the retrieval half of the vector-search surface, pointed at a media
library rather than a transcript. `assets index` walks a directory and embeds
each file; `assets search` returns ranked refs to place on a timeline:

```sh
echo '{"project":"proj"}' | ./build/cutvec assets index ~/sfx
# → {"assets":128,"table":"proj/cache/assets.lance","dim":384,"model":"…"}

echo '{"project":"proj"}' | ./build/cutvec assets search "deep impact hit"
# → {"query":"deep impact hit","assets":[
#      {"ref":"sfx/boom.wav","text":"boom","score":0.91},…],"model":"…"}
```

A file with no annotation is described by its own path; a sidecar `.txt` next to
it overrides that, which is how a library owner fixes a filename that does not
describe its contents. Hits come back in a deterministic total order — descending
score, then `ref`/`text` as stable tiebreaks — and `limit` (10 by default) caps
them.

## Timeline: inspect and edit an existing edit

Beyond building one, `timeline` reads a composed edit back and mutates it in
place, so an agent can adjust a single filter without re-running search and
detection to rebuild everything:

```sh
echo '{"project":"proj"}' | ./build/cutvec timeline inspect
# → the structure: each producer and the filters attached to it

echo '{"project":"proj","add":[{"preset":"grade","producer":"cam1"}]}' \
  | ./build/cutvec timeline edit
```

`add` attaches effects (a named preset, or one MLT service with its properties);
omitting `producer` applies to every clip. Both subcommands take `--plan`.

## MCP: the same catalog over JSON-RPC

`mcp serve` exposes every tool above to an MCP client over stdio. It is the same
catalog the CLI dispatches from, so the two surfaces cannot drift:

```sh
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}' \
  | ./build/cutvec mcp serve
# → {"jsonrpc":"2.0","id":1,"result":{"capabilities":{"tools":{"listChanged":false}},
#    "protocolVersion":"2025-06-18","serverInfo":{"name":"cutvec","version":"0"}}}
```

`tools/list` returns each tool with its input schema; `tools/call` runs one
through the CLI's own dispatch. A tool failure is **not** a protocol failure — it
comes back as `isError` content carrying the structured `{code, hint, retryable}`
error, which is what lets an agent read the hint and retry. Progress rides the
server's stderr, never the JSON-RPC stream on stdout.

## Progress: watch a long step advance

The long steps — `transcribe`, `speakers`, and `render` — write newline-delimited
JSON progress events to **stderr** while their worker runs, so a caller can tell a
slow run from a hung one. **stdout is untouched**: it still carries exactly one JSON
result, so the machine-readable contract is unchanged and the progress channel is
safe to ignore. Each event is one object per line, opening with `start`, beating
`heartbeat` every couple of seconds (override with `CUTVEC_PROGRESS_INTERVAL`, e.g.
`5s`), and closing with `done` (or `error`):

```sh
./build/cutvec render proj --json > result.json 2> progress.jsonl < /dev/null
# progress.jsonl:
# {"kind":"progress","step":"render","phase":"start","seq":0,"elapsed_s":0}
# {"kind":"progress","step":"render","phase":"heartbeat","seq":1,"elapsed_s":2.01}
# {"kind":"progress","step":"render","phase":"done","seq":2,"elapsed_s":3.4}
# result.json: exactly one JSON result, as always
```

A cache hit or `--plan` runs no worker, so it returns immediately and emits nothing.

## Doctor: preflight the whole environment in one call

`doctor` probes every dependency the pipeline needs — external binaries
(`ffmpeg`, `ffprobe`, `whisper-cli`, `melt`, `python3`), the six python worker
executables (via their `CUTVEC_*_WORKER` env vars), the ML python packages
(`lancedb`, `mediapipe`, `numpy`, `opencv-python`, and a `fastembed`/
`sentence-transformers` embedding backend), and the model files (the whisper
ggml model and the mediapipe face landmarker) — and reports each as `ok` or
`missing` with an actionable install hint. It runs no worker and touches no
project, so it is safe to call as a gate before a pipeline spends minutes
failing halfway on something that was never installed.

```sh
./build/cutvec doctor --json
# → {"ok":false,"checks":[{"name":"ffmpeg","kind":"binary","status":"ok",…},
#    {"name":"lancedb","kind":"package","status":"missing","hint":"pip install lancedb"},…],
#    "missing":2,"requiredMissing":2,"missingNames":["lancedb","mediapipe"]}
```

The report is always written to stdout. When a **required** dependency is
missing the command additionally exits non-zero with the structured
`doctor.preflight_failed` error on stderr, so a caller can branch on the exit
code before starting real work; when everything is present it exits `0` with
`"ok":true`.

## Layout

```
cmd/cutvec/         entry point — the CLI, and `cutvec mcp serve` over JSON-RPC
internal/catalog/   the one self-describing tool catalog (backs CLI + MCP)
internal/axerror/   structured errors {code, hint, retryable}
internal/project/   explicit project-dir state: content-hash cache, resumable steps
internal/worker/    versioned JSON contract to the Python ML workers (subprocesses)
internal/timeline/  in-memory MLT XML timeline the agent edits, rendered via melt
internal/doctor/    environment preflight — every binary, worker, package, model
internal/progress/  JSONL progress events on stderr around the long worker steps

```

One package per bounded concern, with no import cycles between concerns.

## Data flow

A single tool call travels one path from entry point to storage and back:

```
agent
  │  JSON request (a tool from `describe`)
  ▼
cmd/cutvec ──▶ internal/catalog        (validate against the tool's JSON Schema)
  │
  ▼
tool handler ─▶ internal/project       (resolve handles to paths; check hash cache)
  │                                    hit ─▶ return cached result
  │ miss
  ▼
internal/worker ──▶ Python worker      (subprocess: whisper / mediapipe / embeddings)
  │  JSON response                     internal/timeline ◀─ mutate MLT XML
  ▼
internal/project (write result + content-hash entry) ──▶ JSON result to agent
```

Handles, not blobs, cross every boundary; a repeated call with unchanged inputs
hits the content-hash cache instead of re-running the worker.

## Architecture

- **Go core, Python workers.** The Go binary is the CLI + MCP surface; all ML
  (transcription, active-speaker detection, embeddings, MLT tooling) runs in
  Python workers reached over a versioned JSON subprocess contract.
- **MLT XML is the edit artifact.** Declarative, inspectable, diffable — an agent
  mutates a structured timeline and renders it with `melt`, never an opaque
  ffmpeg command string.
- **Structured everything.** Every failure is `{code, hint, retryable}`; every
  tool is self-described by JSON Schema; every mutating step supports `--plan`.

## Status

Builds and tests green from a clean checkout. `cutvec describe` returns the
catalog of **15 tools** — `describe`, `init`, `ingest`, `status`, `transcribe`,
`index`, `search`, `speakers`, `shorts`, `assets`, `filters`, `reframe-edit`,
`timeline`, `render`, `doctor` — and each carries a request from the CLI entry
point through `catalog` → its handler → `project` (storage) and back as a JSON
handle. Every capability the opening paragraph claims is implemented: active-
speaker reframe, vertical shorts with hook and karaoke captions, the
grade/denoise/loudness/warm presets, asset retrieval for SFX and b-roll, and the
`mcp serve` surface.

The steps that cross the `worker` boundary to a Python subprocess are stubbed in
tests by a re-exec'd fake worker, so `go test ./...` needs no Python, whisper,
lancedb, or melt on `PATH`. CI runs the Go suite on Linux and macOS with `-race`
and a coverage floor, lints and compiles the workers, and checks every worker
still answers the JSON contract with no ML dependencies installed.

Running the real pipeline does need those dependencies. `cutvec doctor` is the
one call that reports which are missing and how to install each.
