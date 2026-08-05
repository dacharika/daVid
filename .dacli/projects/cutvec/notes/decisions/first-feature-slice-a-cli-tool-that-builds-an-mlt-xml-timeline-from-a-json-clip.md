---
id: d-first-feature-slice-a-cli-tool-that-builds-an-mlt-xml-timeline-from-a-json-clip
kind: note
note_kind: decision
created: 2026-08-05T14:41:18Z
created_by: a-fixer-zqmn1k
about: "[[004]]"
---
# First feature slice = a  CLI tool that builds an MLT XML timeline from a JSON clip list and writes it into the project dir, returning a path handle
## Chose
First feature slice = a  CLI tool that builds an MLT XML timeline from a JSON clip list and writes it into the project dir, returning a path handle
## Rejected
Checking the box with the already-working  command, or implementing the pipeline's literal first step  (needs ffmpeg/Python workers)
## Because
describe was already carried end-to-end by tasks 001/002, so it proves nothing new; proxy/transcribe need the Python worker + ffmpeg stack which is out of scope and unavailable. Building an MLT timeline is the core agent-editable artifact, is pure-Go/dependency-free, and exercises a real path entry point->catalog->timeline->project(storage)->JSON handle back, matching the README data-flow diagram.
