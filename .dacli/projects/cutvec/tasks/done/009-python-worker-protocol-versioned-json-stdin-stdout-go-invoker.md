---
id: t-01KZ9WMYPK5WD7QHZSJ3HA5WR7
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# python worker protocol: versioned JSON stdin/stdout + Go invoker
## So that
Go orchestrates deterministic Python ML workers over a stable contract
## Acceptance
- [x] internal/worker invokes a python worker passing a JSON request on stdin and parsing a JSON response on stdout
- [x] protocol carries a version field and errors map to axerror {code,hint,retryable}
- [x] a fake echo worker round-trips in a go test
## Log
- 2026-08-05T21:37:09Z claimed by a-fixer-4qganh
- 2026-08-05T23:12:47Z a-fixer-978dgv: PR opened: https://github.com/mlnomadpy/cutvec/pull/5 (event 01KZA34V5KY2WYJRFM9F71W57Y)
- 2026-08-05T23:27:50Z accepted by a-root
- 2026-08-05T23:27:50Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-05T23:27:50Z completed by a-root
- 2026-08-06T00:14:52Z status done proposed by a-fixer-978dgv, applied (event 01KZA351RJNQW54SH7RS8YSW2B)
- 2026-08-06T00:14:52Z status done proposed by a-fixer-1prtkf, applied (event 01KZA38KYFGWYZ5M7DTRRB263V)
- 2026-08-06T00:14:52Z status done proposed by a-fixer-sqgwhc, applied (event 01KZA3ATKWZK7KRZ752HZJK8DR)
- 2026-08-06T00:14:52Z status done proposed by a-fixer-bd7bmd, applied (event 01KZA3GYHTF3Q717AER9YDA28X)
- 2026-08-06T00:14:52Z status done proposed by a-fixer-yx2zzj, applied (event 01KZA3K024JP9R2MSVHCV2TJMB)
- 2026-08-06T00:14:52Z status done proposed by a-fixer-fn7bzg, applied (event 01KZA3NA80XEYBEX07BT91ZC6N)
- 2026-08-06T00:14:56Z accepted by a-root
- 2026-08-06T00:14:56Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-06T00:14:56Z completed by a-root
- 2026-08-10T16:55:00Z accepted by a-root
- 2026-08-10T16:55:00Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T16:55:00Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T16:55:00Z completed by a-root
- 2026-08-10T17:20:45Z accepted by a-root
- 2026-08-10T17:20:45Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:20:45Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T17:20:45Z completed by a-root
- 2026-08-10T17:35:37Z accepted by a-root
- 2026-08-10T17:35:37Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:35:37Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T17:35:37Z completed by a-root
- 2026-08-10T17:56:23Z accepted by a-root
- 2026-08-10T17:56:23Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:56:23Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T17:56:23Z completed by a-root
- 2026-08-10T18:10:47Z accepted by a-root
- 2026-08-10T18:10:47Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:10:47Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T18:10:47Z completed by a-root
- 2026-08-10T18:19:14Z accepted by a-root
- 2026-08-10T18:19:14Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:19:14Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T18:19:14Z completed by a-root
- 2026-08-10T18:25:56Z accepted by a-root
- 2026-08-10T18:25:56Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:25:56Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T18:25:56Z completed by a-root
- 2026-08-10T18:36:06Z accepted by a-root
- 2026-08-10T18:36:06Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:36:06Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T18:36:06Z completed by a-root
- 2026-08-10T18:42:35Z accepted by a-root
- 2026-08-10T18:42:35Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:42:35Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T18:42:35Z completed by a-root
- 2026-08-10T18:49:05Z accepted by a-root
- 2026-08-10T18:49:05Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:49:05Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T18:49:05Z completed by a-root
- 2026-08-10T18:55:16Z accepted by a-root
- 2026-08-10T18:55:16Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:55:16Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T18:55:16Z completed by a-root
- 2026-08-10T19:01:45Z accepted by a-root
- 2026-08-10T19:01:45Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:01:45Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T19:01:45Z completed by a-root
- 2026-08-10T19:07:05Z accepted by a-root
- 2026-08-10T19:07:05Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:07:05Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T19:07:05Z completed by a-root
- 2026-08-10T19:13:42Z accepted by a-root
- 2026-08-10T19:13:42Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:13:42Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T19:13:42Z completed by a-root
- 2026-08-10T19:20:29Z accepted by a-root
- 2026-08-10T19:20:29Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:20:29Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T19:20:29Z completed by a-root
- 2026-08-10T19:25:45Z accepted by a-root
- 2026-08-10T19:25:45Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:25:45Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T19:25:45Z completed by a-root
- 2026-08-10T19:32:17Z accepted by a-root
- 2026-08-10T19:32:17Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:32:17Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T19:32:17Z completed by a-root
- 2026-08-10T19:35:52Z accepted by a-root
- 2026-08-10T19:35:52Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:35:52Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T19:35:52Z completed by a-root
- 2026-08-10T19:42:22Z accepted by a-root
- 2026-08-10T19:42:22Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:42:22Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T19:42:22Z completed by a-root
- 2026-08-10T19:48:52Z accepted by a-root
- 2026-08-10T19:48:52Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T19:48:52Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into trunk
- 2026-08-10T19:48:52Z completed by a-root
- 2026-08-10T21:08:20Z accepted by a-root
- 2026-08-10T21:08:20Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:08:20Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into main
- 2026-08-10T21:08:20Z completed by a-root
- 2026-08-10T21:20:06Z accepted by a-root
- 2026-08-10T21:20:06Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:20:06Z deliverable: dacli/009-python-worker-protocol-versioned-json-stdin-stdout-go-invoker is merged into main
- 2026-08-10T21:20:06Z completed by a-root
