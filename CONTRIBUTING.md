# Contributing

Read [AGENTS.md](AGENTS.md) first — it covers the architecture, the rules that
are not negotiable, and the failure modes this project has actually hit. This
file covers the mechanics.

## Getting set up

```bash
brew install ffmpeg whisper-cpp mlt        # media, transcription, rendering
pip install lancedb fastembed mediapipe opencv-python numpy
go build ./...
```

Then check the environment before running anything:

```bash
daVid doctor --json
```

That reports every worker, binary, model and package with a hint for whatever is
missing. It exists because failures used to surface several minutes into a
pipeline instead of before it started.

## The loop

```bash
gofmt -l .                                   # must print nothing
go vet ./... && go test ./...
ruff check workers/ && ruff format --check workers/
./workers/smoke_test.sh <some-media-file>    # the real pipeline, real media
```

The smoke test is the one that matters most. It asserts on substance — words
transcribed, segments indexed, a video actually written with a duration — so a
stubbed implementation passes the unit tests and fails here.

## What a good change looks like

**Verify against the real path.** Render the frame and look at it. Measure the
audio. Read the XML. Every genuine bug found in this project came from doing
that, and several came *after* a green test suite: a clip framed on the wrong
person, shorts framed full-body, a hang no example test caught.

**Say how you know.** The PR template asks for evidence, and it means it — an
extracted frame for anything visual, a measurement for anything audible. "Tests
pass" describes the suite, not the change.

**Treat confident reports as hypotheses.** A review here once reported a feature
as broken, with file:line citations, and was wrong; acting on it would have
deleted working code and rewritten a passing test to match the error. Check
before you act, including on your own findings.

**Keep the two surfaces in step.** A new tool or a changed request shape means
updating `internal/catalog`. Both the CLI and the MCP server read from it, so
that is the only edit needed — and skipping it means the MCP surface silently
lies about what the tool takes.

**Make failures actionable.** Return `{code, message, hint, retryable}` where the
hint says what to do next. `"index missing"` wastes a round trip; `"run daVid
index on this source first"` does not.

## Adding a worker

Workers are subprocesses speaking one JSON Request in, one JSON Response out —
see `workers/README.md` for the contract, and `workers/david_worker.py` for the
plumbing that handles it. Two things are load-bearing:

- **stdout carries only the Response.** Logging goes to stderr. A stray print is
  a protocol violation.
- **Always answer.** Even with nothing installed and nothing to work with, a
  worker returns a structured error rather than dying with a traceback. CI has a
  job that checks exactly this, because no Go test can.

## Commits and PRs

Explain why in the message, not what — the diff already has the what. If a
change fixes something, say what was wrong and what caused it; if an approach was
tried and abandoned, that is worth a line, since it stops the next person
retrying it.

CI runs gofmt, vet, staticcheck, the suite under `-race`, a coverage floor, a
Linux and macOS matrix, Python lint and format, and the worker contract job. The
coverage floor ratchets: raise it when coverage rises, never lower it to go green.
