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

First slice landed. The layout builds and tests green; `cutvec describe` returns
the catalog and `cutvec timeline` carries a request from the CLI entry point
through `catalog` → `timeline` → `project` (storage) and back as a JSON handle —
the full data-flow path above, minus the Python worker that later slices add.
Each remaining bounded concern is a real package that later slices flesh out.
