# Agent Guide — goRP

This is the canonical project guide for AI coding agents working in this repo.
`CLAUDE.md` is a symlink to this file. `.cursor/rules/gorp.mdc` is a thin pointer
that tells Cursor to read this file — don't duplicate content into it.

## What this repo is

A Go CLI tool (`gorp`) and library (`pkg/gorp`) for [ReportPortal](https://reportportal.io).

| Path | Role |
|------|------|
| `main.go` | Binary entry point; global flags (`--config`, `--api-key`, `--project`, `--host`, `--log-level`) and command wiring |
| `internal/commands/` | CLI command implementations (see below) |
| `pkg/gorp/` | Hand-written public library (see below) |
| `pkg/openapi/` | **Generated** OpenAPI client — never edit manually |
| `scripts/` | OpenAPI spec preprocessing filter (`openapi-preprocess.jq`, run through the `jq` Docker image), short-name type aliases template (`openapi-compat/`) |
| `Taskfile.yml` | Task runner for build, test, lint, fmt, OpenAPI codegen |
| `.claude/skills/reportportal/` | Skill for CLI/library reporting workflows (`start-launch`, `test2json`, quality gates, etc.). `.cursor/skills/reportportal/SKILL.md` symlinks to it — same file, don't edit both |

### `internal/commands/`

| File | Contents |
|------|----------|
| `commands.go` | `init`, config loading (`~/.gorp`), client builders (`buildClient`, `buildReportingClient`) |
| `launch.go` | `launch list`, `launch merge` |
| `report.go` | `report test2json` pipeline (test2json event stream → RP); `reporter`/`testEvent` types |
| `report_steps.go` | Atomic report commands: `start-launch`, `start-test`, `log`, `finish-test`, `finish-launch` |
| `quality_gate.go` | `quality-gate check` (polls launch metadata until QG resolves) |
| `util.go` | Shared CLI helpers (config file path, validation, stdin checks) |

### `pkg/gorp/`

| File | Contents |
|------|----------|
| `client.go` | `Client` — embeds generated OpenAPI client + launch helpers; `ClientOption`, `WithApiKeyAuth` |
| `reporting_client.go` | `ReportingClient` — v2 reporting API (launches, test items, logs) via Resty |
| `launches.go` | Launch list/merge helpers (Resty, `/api/v1/…`) |
| `errors.go` | `HTTPError` (status, ReportPortal error code and message, OAuth error, raw body), sentinels `ErrUnauthorized`/`ErrForbidden`/`ErrNotFound`/`ErrNotAPIResponse`, `OpenAPIErrorMiddleware`, `ChainResponseMiddleware`, the Resty error handler |
| `nonjson_operations.go` | `nonJSONOperations`: generated-client operations whose successful response is not JSON (exempt from the 2xx HTML check), and the request → path-template matcher |
| `filters.go` | User-filter lookup for launch queries |
| `oauth2.go` | Password-grant token source, `WithPasswordOwnerGrantAuth`; a failed token request is returned as `HTTPError` |
| `quality_gate.go` | `ParseQualityGate` from launch metadata |
| `multipart.go` | `Multipart` interface + `FileMultipart`/`ReaderMultipart` |
| `model_api.go`, `model_enums.go` | Hand-written request/response types, `Statuses`, `TestItemTypes`, `LaunchModes`, log levels |

### CLI command tree

```
gorp
├── init
├── launch
│   ├── list
│   └── merge
├── report
│   ├── test2json
│   ├── start-launch
│   ├── start-test
│   ├── log
│   ├── finish-test
│   └── finish-launch
└── quality-gate (alias: qg)
    └── check (alias: qgc)
```

### Configuration

Settings are read from three sources (lowest → highest precedence):

1. Config file `~/.gorp` (JSON: `host`, `project`, `api_key`) — created by `gorp init`; `--config`/`GORP_CONFIG` selects another file instead (it must exist, no fallback to `~/.gorp`)
2. Environment variables (`GORP_API_KEY`, `GORP_PROJECT`, `GORP_HOST`, plus per-command vars like `LAUNCH_UUID`, `ITEM_UUID`)
3. CLI flags (`--host`, `--api-key`, `--project`, plus per-command flags)

`GORP_UUID` and `--uuid` are deprecated aliases for `GORP_API_KEY`/`--api-key`; a deprecation
warning is logged when they're used (see `main.go`'s `Before` hook). `-u` is a regular short alias of `--api-key`.

Global flag `--log-level` (default `info`; source locations are logged only at `debug`) controls `slog` verbosity; it is *not* validated
against a fixed enum — pass any value `slog.Level.UnmarshalText` accepts (`debug`, `info`, `warn`, `error`).

## Commands

| Task | Command |
|------|---------|
| Build binary → `bin/gorp` | `task build` |
| Run tests (with coverage) | `task test` |
| End-to-end tests against a real ReportPortal (not run by CI) | `task e2e` (see below) |
| Lint (Docker, golangci-lint v2.15.0) | `task lint` |
| Format (Docker) | `task fmt` |
| Regenerate OpenAPI client | `task generate-openapi-client` (runs `preprocess:schema` first, then swaps the result into `pkg/openapi/` only on success) |
| Refresh spec from upstream, then regenerate | Replace `reportportal-api-docs.json`, then `task generate-openapi-client` |
| Create release tag | `task tag VERSION=x.y.z` |
| Goreleaser release | `task release` |

Upstream snapshot: `reportportal-api-docs.json` (from [https://demo.reportportal.io/api/v1/api-docs](https://demo.reportportal.io/api/v1/api-docs)). `task preprocess:schema` applies `scripts/openapi-preprocess.jq` to it and writes the result to `openapi-modified.json` (git-ignored, derived), which is the input for codegen.

`task` is the task runner ([Taskfile.yml](Taskfile.yml)). Install it with `go install github.com/go-task/task/v3/cmd/task@latest` or via your package manager. `task lint`/`task fmt` run golangci-lint inside Docker (`golangci/golangci-lint:v2.15.0`) — no local install required, but Docker must be running. Podman works too: put a `docker` symlink to `podman` first on `PATH` — the Taskfile calls `docker` by name. File tasks use plain `mkdir`/`rm`/`cp`/`mv`/`test`; on Windows run Task from WSL/Git Bash or set `TASK_CORE_UTILS=1` (Task's built-in coreutils).

### End-to-end tests (`e2e/`)

Every file in `e2e/` has the `e2e` build tag, so `go test ./...` and CI never compile or run them. `task e2e`
(or `go test -tags e2e -count=1 -v ./e2e/...`) builds the `gorp` binary, runs it against the ReportPortal from
`.gorp` in the repository root (git-ignored, same JSON as `~/.gorp`; `GORP_CONFIG` selects another file;
`GORP_HOST`/`GORP_PROJECT`/`GORP_API_KEY` override single fields; `~/.gorp` is never used) and verifies the result through the API.
Use a **sandbox project**: the tests write real launches named `gorp-e2e-…` and delete them afterwards (leftovers of
crashed runs are swept at the start of the next run). Reporting is asynchronous, so assertions poll for up to 90 s.
The suite skips itself when no credentials are configured.

## Code conventions

- **Go version:** 1.27 (see `go.mod`; module requires `go 1.27.0`).
- **Module path:** `github.com/reportportal/goRP/v5`.
- **Imports:** three groups enforced by `gci` (`.golangci.yml`) — `standard`, `default` (third-party), `prefix(github.com/reportportal/goRP/v5)` (internal).
- **Formatting:** `gofumpt` with `extra-rules: true`, max line length 140 (`lll` linter). Run `task fmt`, don't hand-format.
- **Context:** every `ReportingClient` method takes `ctx context.Context` as the first argument and passes it to the underlying Resty request via `.SetContext(ctx)`.
- **Errors:** use `fmt.Errorf("…: %w", err)` for wrapping. Do not swallow errors (except in `defer` close, where logging via `slog` is acceptable).
- **CLI output:** write user-facing output to `cmd.Writer` (e.g. `fmt.Fprintln(cmd.Writer, …)` or `fmt.Fprintf(cmd.Writer, …)`), never to `os.Stdout` directly. This keeps commands testable — tests supply a `bytes.Buffer` as `cmd.Writer` to capture and assert on output.
- **Tests:** use `httptest.Server` for HTTP mocking. `errcheck` is suppressed in `_test.go` files (`.golangci.yml` exclusion). Set `cmd.Writer` to a `bytes.Buffer` when testing CLI commands so output can be verified without touching stdout. Use `t.Parallel()` in new tests where the existing file already does.
- **CLI flags:** built with `urfave/cli/v3`. Prefer `Sources: cli.EnvVars(...)` for env-var-backed flags, `Required: true` for mandatory ones, and `MutuallyExclusiveFlags` (see `quality_gate.go`) when exactly one of several flags must be given.

## Key design decisions

- `pkg/openapi/` uses **`net/http`**; `pkg/gorp/` uses **Resty v3**. Both share the same `*http.Client` built from an OAuth2 transport (API key via `oauth2.StaticTokenSource`, or password-grant via `WithPasswordOwnerGrantAuth`).
- Every failed API call, through either client, returns a `*gorp.HTTPError` built by `newHTTPError` (`pkg/gorp/errors.go`). The generated client gets it through `Configuration.ResponseMiddleware` (`OpenAPIErrorMiddleware`, set in `newAPIClient`), which runs before the generated per-status decoding, so that decoding is never reached for a failed response; the error unwraps to `*openapi.GenericOpenAPIError` (built with `openapi.NewGenericOpenAPIError` from the compat template). The body parser must never fail: unknown shapes fall back to the trimmed raw body. A 2xx `text/html` response is an error (`ErrNotAPIResponse`) on both clients; on the generated client the operations that legitimately return HTML/files or nothing (`GetLaunchReport`, `GetFile`, …) are exempt. They are listed by method + path template in `nonJSONOperations` (`pkg/gorp/nonjson_operations.go`) and matched against the end of the request path. The list is hand-written but derived from `pkg/openapi/api/openapi.yaml` (no 2xx `application/json` content, or that content is `format: binary`): `TestNonJSONOperationsMatchSpec` recomputes it and prints the list to paste, so after `task generate-openapi-client` run the tests and update it. The generated client has a single `ResponseMiddleware` slot: never assign over `OpenAPIErrorMiddleware`, combine with `ChainResponseMiddleware`. A failed password-grant token request (`oauth2.go`) is converted by `newTokenError` into the same `HTTPError`, which unwraps to `*oauth2.RetrieveError`.
- Quality gate polling uses a ticker (`time.NewTicker`) — the channel must be selected on, never `default`. The caller's `ctx` must be forwarded into each poll iteration. A failed/non-PASSED quality gate exits with **exit code 10** (`cli.Exit(..., 10)` in `quality_gate.go`); this is depended on by CI pipelines — do not change it casually.
- Log batch goroutines inside the `test2json` reporter are managed with `errgroup.Group` (not `sync.WaitGroup`). Errors surface at `errGroup.Wait()` in `receive()`.
- `pkg/openapi/` is regenerated from `openapi-modified.json`. `openapi-modified.json` is a git-ignored derived file: never edit it, change `reportportal-api-docs.json` or `scripts/openapi-preprocess.jq` instead.
- `report test2json` parses `go test -json` output (see `internal/commands/report.go`'s `testEvent` struct). As of **Go 1.27**, `test2json` annotates `"action":"output"` events with an `OutputType` field (`"frame"`, `"error"`, `"error-continue"`, or blank) and `"action":"fail"` events with `FailedBuild` when the failure was a build failure. The repo's `testEvent` struct captures both fields; `logLevelForOutputType` promotes `error`/`error-continue` output to `ERROR`-level RP logs, and `shouldMergeOutput` merges `error-continue` (or, for pre-1.27 streams with no `OutputType`, tab-indented) chunks into the previous log entry. `FailedBuild` is parsed but not currently acted on. See `report.go` and `report_log_test.go` for the full logic and test coverage. **Note:** despite the pre-existing fields (`time`, `action`, etc.) being tagged with lowercase `json:"..."` names while `OutputType`/`FailedBuild` use exact-case tags, `encoding/json.Unmarshal`'s case-insensitive fallback makes both work correctly against the real (PascalCase) `go test -json` wire format — this is a cosmetic inconsistency, not a bug.
- `Statuses.Canceled` (Go identifier, American spelling) serializes to the string `"CANCELLED"` (British spelling, matching the ReportPortal API) — don't "fix" the string value to match the identifier.
- Generated types use long fully-qualified names (`ComEpamReportportal...`); short names (`StartLaunchRQ`, `PageMetadata`, …) are type aliases kept for backward compatibility in `scripts/openapi-compat/compat.go.tmpl`, which `task generate-openapi-client` copies to `pkg/openapi/compat.go`. If a regenerated spec renames a type, `go build` fails in `compat.go` — fix the template. An alias must point to the type with the same meaning, not just one that compiles: e.g. `LaunchResource` is the reporting model, while launch list/get endpoints return `LaunchViewModel` / `PageLaunchViewModel`.

## What NOT to do

- **Do not edit any file in `pkg/openapi/`** — it is fully generated. Change the schema or generator config instead.
- Do not edit `pkg/openapi/compat.go` directly — edit `scripts/openapi-compat/compat.go.tmpl` and regenerate (or re-copy it with `cp`).
- Do not add `default:` cases to ticker `select` loops — it causes busy-looping.
- Do not use `sync.WaitGroup` for goroutines that can fail — use `errgroup.Group`.
- Do not ignore `scanner.Err()` after a `bufio.Scanner` loop.
- Do not use `fmt.Println`/`fmt.Printf` for CLI command output — use `fmt.Fprintln(cmd.Writer, …)` / `fmt.Fprintf(cmd.Writer, …)` instead.
- Do not change the quality-gate failure exit code (`10`) without checking CI usage first.
- Do not hand-run `gofmt`/`goimports` as a substitute for `task fmt` — the project's `gci`/`gofumpt` settings (module path, local-prefixes, extra-rules) aren't applied by bare `gofmt`.
- **Do not fork `CLAUDE.md`, `.cursor/rules/gorp.mdc`, or either `SKILL.md` into separate content** — they are symlinks/pointers to this file and to `.claude/skills/reportportal/SKILL.md` respectively. Edit the canonical file only.

## Verifying changes

Before considering a change done:

1. `go build ./...` and `go vet ./...`
2. `task test` (or `go test -cover ./...` if Docker/Task aren't available)
3. `task e2e` when the change affects what is sent to ReportPortal (needs credentials, writes to the configured project)
4. `task lint` (or `golangci-lint run ./...` if you have it installed locally with the pinned config)
5. For anything touching `report test2json` / `internal/commands/report.go`: generate real `go test -json` output from the installed Go toolchain and diff its shape against the `testEvent` struct and `report_log_test.go` fixtures rather than relying on release notes alone — Go's JSON stream format is a better source of truth than prose docs.
