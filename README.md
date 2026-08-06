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
# → {"source":{"id":"8b1faca47fef","path":"raw.mp4","hash":"8b1f…"},"cached":false}
echo '{"project":"proj","source":"raw.mp4"}' | ./build/cutvec ingest   # re-run
# → {"source":{…},"cached":true}   ← content-hash cache hit; nothing re-recorded

echo '{"project":"proj"}' | ./build/cutvec status --json
# → {"project":"proj","name":"podcast","sources":[…],"artifacts":[{"name":"timeline","status":"pending"}]}
```

`init` is idempotent (re-running returns `created:false`, never clobbering).
`ingest` keys each source on the sha256 of its bytes: re-ingesting an unchanged
file is a **cache hit** (`cached:true`) that records nothing — the step is
deterministic, idempotent, and resumable. `status` returns the state graph — the
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

## Layout

```
cmd/cutvec/         entry point — CLI today, `cutvec mcp` serves the same catalog
internal/catalog/   the one self-describing tool catalog (backs CLI + MCP)
internal/axerror/   structured errors {code, hint, retryable}
internal/project/   explicit project-dir state: content-hash cache, resumable steps
internal/worker/    versioned JSON contract to the Python ML workers (subprocesses)
internal/timeline/  in-memory MLT XML timeline the agent edits, rendered via melt
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

The layout builds and tests green from a clean checkout. `cutvec describe`
returns the catalog of seven tools — `describe`, `init`, `ingest`, `status`,
`transcribe`, `timeline`, `render` — and each carries a request from the CLI
entry point through `catalog` → its handler → `project` (storage) and back as a
JSON handle. `transcribe` and `render` complete the full data-flow path above,
crossing the `worker` boundary to a subprocess (stubbed in tests by a re-exec'd
fake worker, so `go test ./...` needs no Python, whisper, or melt on `PATH`).
Later slices flesh out the remaining edits (active-speaker reframe, vertical
shorts, captions, grade, denoise, SFX) and the `cutvec mcp` server surface.
