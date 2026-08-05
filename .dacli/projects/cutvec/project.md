---
id: p-cutvec
kind: project
created: 2026-08-05T13:24:56Z
created_by: a-root
status: active
stage: definition
github_repo: mlnomadpy/cutvec
---
# cutvec
## Goal
Agent-native video-editing CLI plus MCP server: MLT XML timelines and vector search reproduce a full podcast edit (proxy, transcribe, active-speaker reframe, vertical shorts with hook and karaoke captions, color grade, denoise, SFX) designed for AI agents not humans, with JSON IO, self-describing schemas, deterministic idempotent resumable steps, and plan/dry-run.
## Constraints
Stack: Go. Build with `go build ./...`; test with `go test ./...`. A task in this project is done only when both exit 0.
## Out of scope
Everything past the first working release of cutvec: additional platforms, paid tiers, and localisation stay outside this project until that release ships.
A planner agent narrows this line to the real boundary once discovery has run.
## Success criteria
- `go build ./...` and `go test ./...` both exit 0 from a clean checkout.
- The first feature slice runs from its user-visible entry point through to a returned result.
- A reader who has never seen this repository reaches a green test run from the README alone.
## Spec
**What cutvec is:** Agent-native video-editing CLI plus MCP server: MLT XML timelines and vector search reproduce a full podcast edit (proxy, transcribe, active-speaker reframe, vertical shorts with hook and karaoke captions, color grade, denoise, SFX) designed for AI agents not humans, with JSON IO, self-describing schemas, deterministic idempotent resumable steps, and plan/dry-run.

**Primary user:** one sentence naming who runs it and the outcome they came for. A planner agent writes this line from discovery, before the first implementation task is claimed.

**User-visible surface:** every entry point a person touches — commands, screens, or endpoints. Name each one and the result it returns.

**Stated behaviour on failure and on empty input:** one written rule per case. A rule nobody wrote down arrives later as a defect report.

**Scope boundary:** the Out of scope section of this document. Every line there is a boundary this spec inherits.

**Open questions a planner answers before build:** data model, persistence, authentication, deployment target. Each answer belongs in `dacli note add decision` with the rejected alternative recorded alongside it.
## Architecture
**Stack:** Go — scaffold with `go mod init <module-path>`, build with `go build ./...`, test with `go test ./...`.

**Starting layout:** cmd/ for entry points, internal/ for packages the module owns, one package per bounded concern.

**Module boundaries:** one directory per bounded concern, with no import cycles between concerns. A planner agent names the concerns from the spec's user-visible surface and records each boundary as a decision note.

**Data flow:** the single path a request takes from the entry point to storage and back. Record it as one diagram in the README.

**Dependency rule:** a new third-party dependency needs a decision note naming what it replaces and what was rejected.
