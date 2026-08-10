<!--
Every reviewer here is short on context, and several are agents. Say what
changed and how you know it works; the diff already says how.
-->

## What changed and why

<!-- The behaviour, not the file list. If this fixes something, say what was
wrong and what caused it. -->

## How it was verified

<!--
Required. "Tests pass" is not verification — this project has shipped clips
framed on the wrong person, a hang no test caught, and a review that was
confidently wrong, all with a green suite.

Show the evidence:
  - visual change  -> an extracted frame, or a before/after
  - audio change   -> a measurement (levels, spectrum), not "sounds right"
  - timeline change-> the XML, or what melt did with it
  - protocol change-> the actual request/response
-->

## Anything not done, or done differently than asked

<!-- Scope you left out, a decision that diverged, or a finding that turned out
to be wrong. Say it here rather than leaving it to be discovered. -->

---

- [ ] `gofmt -l .` prints nothing, `go vet ./...` and `go test ./...` are clean
- [ ] Python touched? `ruff check workers/` and `ruff format --check workers/` pass
- [ ] New tool or changed IO? `internal/catalog` updated, so the CLI and MCP stay in step
- [ ] New failure mode? It returns `{code, message, hint, retryable}` with a hint that says what to do
