# Agent Guide — goRP

## What this repo is

A Go CLI tool (`gorp`) and library (`pkg/gorp`) for [ReportPortal](https://reportportal.io).

- **`main.go`** — binary entry point; wires CLI flags and commands.
- **`internal/commands/`** — CLI command implementations (`init`, `launch`, `report`, `quality-gate`).
- **`pkg/gorp/`** — hand-written public library: `Client` (OpenAPI + launch helpers), `ReportingClient` (v2 reporting via Resty), OAuth2 auth, filters, quality-gate parser.
- **`pkg/openapi/`** — **generated** OpenAPI client; never edit manually.

## Commands

| Task | Command |
|------|---------|
| Build binary → `bin/gorp` | `task build` |
| Run tests | `task test` |
| Lint (Docker) | `task lint` |
| Format (Docker) | `task fmt` |
| Regenerate OpenAPI client | `task preprocess:schema && task generate-openapi-client` |

`task` is the task runner ([Taskfile.yml](Taskfile.yml)). Install it with `go install github.com/go-task/task/v3/cmd/task@latest` or via your package manager.

## Code conventions

- **Go version:** 1.25 (see `go.mod`).
- **Module path:** `github.com/reportportal/goRP/v5`.
- **Imports:** three groups — stdlib / third-party / internal (`github.com/reportportal/goRP/v5/…`). Enforced by `gci`/`goimports`.
- **Formatting:** `gofumpt` with `extra-rules: true`, max line length 140.
- **Context:** every `ReportingClient` method takes `ctx context.Context` as the first argument and passes it to the underlying Resty request via `.SetContext(ctx)`.
- **Errors:** use `fmt.Errorf("…: %w", err)` for wrapping. Do not swallow errors (except in `defer` close, where logging is acceptable).
- **Tests:** use `httptest.Server` for HTTP mocking. `errcheck` is suppressed in `_test.go` files.

## Key design decisions

- `pkg/openapi/` uses **`net/http`**; `pkg/gorp/` uses **Resty v3**. Both share the same `*http.Client` built from an OAuth2 transport.
- Quality gate polling uses a ticker (`time.NewTicker`) — the channel must be selected on, never `default`. The caller's `ctx` must be forwarded into each poll iteration.
- Log batch goroutines are managed with `errgroup.Group` (no `sync.WaitGroup`). Errors surface at `errGroup.Wait()` in `receive()`.
- `pkg/openapi/` is regenerated from `openapi-modified.json` (preprocessed with `jq` to convert `Map<String,Object>` fields to `map[string]interface{}`). Run `preprocess:schema` before `generate-openapi-client`.

## What NOT to do

- **Do not edit any file in `pkg/openapi/`** — it is fully generated. Change the schema or generator config instead.
- Do not add `default:` cases to ticker `select` loops — it causes busy-looping.
- Do not use `sync.WaitGroup` for goroutines that can fail — use `errgroup.Group`.
- Do not ignore `scanner.Err()` after a `bufio.Scanner` loop.
