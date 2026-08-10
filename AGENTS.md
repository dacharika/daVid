# Working in this repository

cutvec's primary user is an AI agent, and so is most of its contributor base.
This file is the brief: what the project values, what has already been decided,
and the failure modes that have actually cost time here.

## What this is

An agent-native video editor. A Go core owns orchestration, caching and the
agent-facing contract; Python workers do the ML and media work behind a
versioned JSON protocol. The editable artifact is an **MLT XML timeline** — a
declarative document an agent can inspect, mutate, validate and re-render —
never an opaque ffmpeg command string.

```
ingest → transcribe → speakers → index → search → timeline → render
                                            └→ recipes: shorts, reframe-edit
```

## The rules that are not negotiable

**One catalog backs both surfaces.** `internal/catalog` defines every tool with
JSON Schemas. The CLI dispatches from it and the MCP server serves it, so adding
a tool in one place adds it to both. Never hand-write an MCP tool definition.

**Handles, not blobs.** Tools return paths and small summaries. An agent that
wants the content asks for it. Returning a transcript inline would cost a caller
more tokens than the whole rest of the call.

**Every failure is structured.** `{code, message, hint, retryable}`. The hint has
to say what to do next — "run `cutvec index` first", not "index missing". An
error a caller cannot act on is a dead end.

**Deterministic and idempotent.** Steps are keyed by content hash; re-running is
a cache hit. Identical requests must produce byte-identical output — that is why
effect properties are emitted in sorted order, and why `Date`-like nondeterminism
never enters a timeline.

**Local by default.** Embeddings, models and inference run on the machine. No API
key, no network at query time. A tool an agent depends on should not fail because
someone's credentials expired.

## Verification: the part that actually matters

Tests passing is not evidence that a feature works. This project has shipped,
and caught, all of the following — each of which passed its own tests:

- A vertical crop that produced technically-correct 1080x1920 clips framed on the
  wrong person, because the crop bias sign was inverted.
- Shorts framed full-body rather than head-and-shoulders: right format, unusable
  output.
- An MCP frame reader that spun forever on a malformed frame — a hang, not a
  crash, so no example-based test caught it.
- A review that confidently reported punch-ins as a no-op, with file:line
  citations, and was **wrong**. Acting on it would have deleted working code.

So: **verify against the real path, and look at the artifact.** Render the frame
and read it. Measure the audio. Diff the XML. When a change affects what a viewer
sees or hears, a screenshot or a measurement belongs in the PR — not a claim.

Treat a confident report as a hypothesis until you have checked it yourself. That
applies to reports from humans, from other agents, and to your own.

## Layout

| path | what lives there |
|---|---|
| `cmd/cutvec` | CLI dispatch, one file per command group, plus the MCP server |
| `internal/catalog` | the tool catalog and JSON Schemas — the source of truth |
| `internal/timeline` | the MLT XML model: profiles, producers, filters, transitions |
| `internal/project` | project state, content-hash cache, manifest |
| `internal/worker` | the Go side of the worker protocol |
| `internal/axerror` | the structured error shape |
| `workers/` | Python ML workers, one per concern, plus `smoke_test.sh` |

## Before you open a PR

```bash
gofmt -l .            # must print nothing
go vet ./...
go test ./...         # and -race if you touched anything concurrent
ruff check workers/ && ruff format --check workers/
```

CI additionally runs staticcheck, a coverage floor, a Linux+macOS matrix, and a
**worker contract job** that invokes each Python worker with no dependencies
installed and requires a well-formed JSON error with a hint. That last one checks
a promise no Go test can reach.

`workers/smoke_test.sh <media>` runs the whole pipeline against real media and
asserts on substance — words transcribed, segments indexed, a video actually
written. A stubbed implementation passes the unit tests and fails this.

## Things already decided

Recorded as decision notes under `.dacli/projects/cutvec/notes/decisions/`, and
worth reading before proposing an alternative — they exist so the same debate is
not had twice. The short version:

- Go core + Python workers over subprocess JSON, not cgo and not a service.
- MLT XML as the artifact, not generated ffmpeg strings.
- LanceDB with local embeddings, not a hosted vector service.
- Presets are named lists of real MLT filters, so anything a preset does can be
  written longhand. `cutvec filters` discovers the several hundred MLT offers at
  runtime rather than hardcoding a chosen few.
