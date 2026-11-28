![Build Status](https://github.com/reportportal/goRP/workflows/Build/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/reportportal/goRP)](https://goreportcard.com/report/github.com/reportportal/goRP)
[![License MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/reportportal/goRP/master/LICENSE)
[![Release](https://img.shields.io/github/release/reportportal/goRP.svg)](https://github.com/reportportal/goRP/releases/latest)
[![GitHub Releases Stats of goRP](https://img.shields.io/github/downloads/reportportal/goRP/total.svg?logo=github)](https://somsubhra.github.io/github-release-stats/?username=reportportal&repository=gorP)

# goRP

**goRP** is a Go client library, CLI tool, and test reporter for [ReportPortal](https://reportportal.io).

- **CLI (`gorp`)** — manage launches, report `go test` results, and enforce quality gates from the terminal or CI.
- **Go library (`pkg/gorp`)** — integrate ReportPortal reporting directly into your Go applications.

## Quick start

```sh
gorp init                                              # host, API key, project -> ~/.gorp
go test -json ./... | gorp report test2json \
  --print-launch-uuid --quality-gate-check             # report the run, then enforce the quality gate
```

Exit code `10` means the quality gate failed; see [Exit Codes](#exit-codes).

---

## Table of Contents

1. [Installation](#installation)
2. [Configuration](#configuration)
3. [CLI Reference](#cli-reference)
   - [Global flags](#global-flags)
   - [init](#init)
   - [launch list](#launch-list)
   - [launch merge](#launch-merge)
   - [report test2json](#report-test2json)
   - [report start-launch](#report-start-launch)
   - [report start-test](#report-start-test)
   - [report log](#report-log)
   - [report finish-test](#report-finish-test)
   - [report finish-launch](#report-finish-launch)
   - [quality-gate check](#quality-gate-check)
4. [CI/CD Integration](#cicd-integration)
5. [Go Library](#go-library)
6. [Environment Variables](#environment-variables)
7. [Exit Codes](#exit-codes)
8. [Compatibility](#compatibility)
9. [Development](#development)

---

## Installation

### go install

```sh
go install github.com/reportportal/goRP/v5@latest
```

> `go install` names the binary after the module (`goRP`), while this README and the release archives use `gorp`.
> On a case-sensitive filesystem (Linux) either call `goRP` or add a link:
> `ln -s "$(go env GOPATH)/bin/goRP" "$(go env GOPATH)/bin/gorp"`.

### Pre-built binary — latest release (installs as `gorp`)

```sh
curl -s https://api.github.com/repos/reportportal/goRP/releases/latest \
  | jq -r '.assets[] | select(.name | contains("tar.gz")) | .browser_download_url' \
  | grep "$(uname)_$(arch)" \
  | xargs curl -sL | tar zx -C .
```

---

## Configuration

goRP reads connection settings from three sources, in order of increasing precedence:

1. **Config file** — `~/.gorp` (JSON), or the file given with `--config` / `GORP_CONFIG`
2. **Environment variables**
3. **CLI flags**

### Interactive setup

Run `gorp init` for a guided setup that writes `~/.gorp`:

```sh
gorp init
```

You will be prompted for the ReportPortal host, your API key (input is hidden), and the default project.
The file is written with mode `0600`.

### Config file format

`~/.gorp` is a JSON file with the following fields:

```json
{
  "host":    "https://reportportal.example.com",
  "project": "my_project",
  "api_key": "your_api_key_here"
}
```

### Multiple environments

`--config` / `GORP_CONFIG` selects a different file *instead of* `~/.gorp` (it must exist; there is no fallback):

```sh
GORP_CONFIG=./staging.gorp gorp launch list
gorp --config ./staging.gorp launch list
```

### Organizations

ReportPortal 26.x introduced organizations, but you do **not** need to configure one. Reporting, launch listing,
merging and quality-gate calls are addressed by project name only, so `host`, `project` and `api_key` are all that is needed.

### CLI flags (ad-hoc / override)

Any value in the config file can be overridden at runtime:

```sh
gorp --host https://rp.example.com \
     --api-key <token> \
     --project my_project \
     launch list
```

---

## CLI Reference

### Global flags

These flags apply to every command.

| Flag | Short | Env var | Description |
|------|-------|---------|-------------|
| `--config` | | `GORP_CONFIG` | Configuration file to use instead of `~/.gorp`. It must exist; there is no fallback to `~/.gorp` |
| `--api-key` | `-u` | `GORP_API_KEY` | ReportPortal API key (user token) |
| `--project` | `-p` | `GORP_PROJECT` | ReportPortal project name |
| `--host` | | `GORP_HOST` | ReportPortal server URL |
| `--log-level` | | | Log verbosity: `debug`, `info`, `warn`, `error` (default: `info`) |

---

### init

Interactively create or replace `~/.gorp` (or the file given with `--config`).

```sh
gorp init
```

If a config file already exists you will be asked whether to overwrite it.

---

### launch list

List launches in a project.

```sh
# All launches in the configured project
gorp launch list

# Filter by a saved filter name
gorp launch list --filter-name "nightly"

# Filter by raw RP filter expression
gorp launch list --filter "filter.eq.name=smoke&filter.eq.status=FAILED"
```

| Flag | Short | Env var | Description |
|------|-------|---------|-------------|
| `--filter-name` | `--fn` | `FILTER_NAME` | Saved filter name |
| `--filter` | `-f` | `FILTER` | Raw RP filter query string. Repeatable; parts are joined with `&` |

---

### launch merge

Merge several launches into one.

```sh
# Merge by explicit IDs
gorp launch merge --ids 101 --ids 102 --name "Weekly run"

# Merge all launches matching a saved filter
gorp launch merge --filter-name "nightly" --name "Merged nightly" --type DEEP

# Merge all launches matching a raw filter
gorp launch merge --filter "filter.eq.name=smoke" --name "Merged smoke"
```

| Flag | Short | Env var | Description |
|------|-------|---------|-------------|
| `--ids` | | `MERGE_LAUNCH_IDS` | Launch IDs to merge (repeatable) |
| `--filter` | `-f` | `MERGE_LAUNCH_FILTER` | Raw RP filter to select launches |
| `--filter-name` | `--fn` | `FILTER_NAME` | Saved filter name to select launches |
| `--name` | `-n` | `MERGE_LAUNCH_NAME` | **Required.** Name of the resulting launch |
| `--type` | `-t` | `MERGE_TYPE` | Merge strategy: `DEEP` (default) or `BASIC` |

Exactly one of `--ids`, `--filter` or `--filter-name` must be given. The ID of the merged launch is printed to stdout.

---

### report test2json

Report `go test -json` output to ReportPortal. Accepts input from a file (`--file`) or stdin (piped).

```sh
# Pipe directly from go test
go test -json ./... | gorp report test2json

# Read from a saved file
go test -json ./... > results.jsonl
gorp report test2json --file results.jsonl

# Set a custom launch name and attributes
go test -json ./... | gorp report test2json \
  --launchName "PR #42" \
  --attr "branch:main" \
  --attr "ci"

# Print the launch UUID after reporting (useful for chaining with quality-gate check)
go test -json ./... | gorp report test2json --print-launch-uuid

# Report and immediately check the quality gate in one step
go test -json ./... | gorp report test2json --quality-gate-check
```

| Flag | Short | Env var | Default | Description |
|------|-------|---------|---------|-------------|
| `--file` | `-f` | `FILE` | stdin | Input file (test2json format). Reads stdin if omitted. |
| `--launchName` | `--ln` | `LAUNCH_NAME` | `gorp launch` | Launch name in ReportPortal |
| `--attr` | `-a` | | | Launch attribute, format `key:value` or `value`. Repeatable. |
| `--reportEmptyPkg` | `--ep` | `REPORT_EMPTY_PKG` | `false` | Report packages with no test cases |
| `--print-launch-uuid` | | | `false` | Print `ReportPortal Launch UUID: <uuid>` to stdout after reporting |
| `--quality-gate-check` | `--qgc` | `QUALITY_GATE_CHECK` | `false` | Poll for quality gate result after reporting; exits `10` on failure |
| `--quality-gate-timeout` | `--qgt` | `QUALITY_GATE_TIMEOUT` | `1m` | Maximum time to wait for quality gate |
| `--quality-gate-check-interval` | `--qgci` | `QUALITY_GATE_CHECK_INTERVAL` | `3s` | How often to poll for quality gate status |

The launch finishes as `FAILED` if any test or package failed, otherwise `PASSED`. If the run is interrupted
(Ctrl-C, `SIGTERM`) or the input stream breaks midway, items that are still running are finished as `INTERRUPTED`,
followed by the launch, so nothing is left `IN_PROGRESS`. A second Ctrl-C terminates immediately.

---

### report start-launch

Start a new launch and print its UUID to stdout. Use the UUID with subsequent commands.

```sh
LAUNCH_UUID=$(gorp report start-launch --name "Nightly run" --attr "branch:main" --attr "ci")
```

| Flag | Short | Env var | Default | Description |
|------|-------|---------|---------|-------------|
| `--name` | `-n` | | | **Required.** Launch name |
| `--description` | | | | Launch description |
| `--attr` | `-a` | | | Attribute, format `key:value` or `value`. Repeatable. |
| `--mode` | | | `DEFAULT` | Launch mode: `DEFAULT` or `DEBUG` (case-insensitive; anything else is rejected) |
| `--rerun` | | | `false` | Rerun the latest launch with the same name instead of starting a new one |
| `--rerun-of` | | | | UUID of the launch to rerun. Implies `--rerun`. |

With `--rerun` or `--rerun-of` the command prints the UUID of the launch being rerun (ReportPortal 5.13.5 or newer), so
the following commands report into that launch. ReportPortal matches the items to the existing ones by test case ID
(derived from the code reference or the item's place in the tree when not given): a matched test becomes the latest
attempt and the earlier one is kept as its retry. If there is no launch to rerun, the command
fails with a 404 error.

```sh
LAUNCH_UUID=$(gorp report start-launch --name "Nightly run" --rerun-of "$PREVIOUS_LAUNCH_UUID")
```

---

### report start-test

Start a test item and print its UUID to stdout. Use `--parent-uuid` to create a child item (e.g. a test under a suite).

```sh
# Root item (suite)
SUITE_UUID=$(gorp report start-test \
  --launch-uuid "$LAUNCH_UUID" --name "pkg/foo" --type SUITE)

# Child item (test under suite)
TEST_UUID=$(gorp report start-test \
  --launch-uuid "$LAUNCH_UUID" --parent-uuid "$SUITE_UUID" \
  --name "TestBar" --type TEST --code-ref "pkg/foo/TestBar")
```

| Flag | Short | Env var | Default | Description |
|------|-------|---------|---------|-------------|
| `--launch-uuid` | | `LAUNCH_UUID` | | **Required.** Launch UUID |
| `--name` | `-n` | | | **Required.** Test item name |
| `--type` | `-t` | | `TEST` | Item type: `SUITE`, `STORY`, `TEST`, `SCENARIO`, `STEP`, `BEFORE_CLASS`, `BEFORE_GROUPS`, `BEFORE_METHOD`, `BEFORE_SUITE`, `BEFORE_TEST`, `AFTER_CLASS`, `AFTER_GROUPS`, `AFTER_METHOD`, `AFTER_SUITE`, `AFTER_TEST` (case-insensitive; anything else is rejected) |
| `--parent-uuid` | | `PARENT_UUID` | | Parent item UUID (creates a child item) |
| `--description` | | | | Item description |
| `--code-ref` | | | | Source code reference |
| `--attr` | `-a` | | | Attribute. Repeatable. |
| `--test-case-id` | | | | Test case ID: identifies the same test across launches. By default the server derives it from `--code-ref`. |
| `--param` | | | | Test parameter, format `key=value`. Repeatable. A comma separates several parameters, so a value can't contain one. |
| `--retry-of` | | | | UUID of the previous attempt: reports the item as its retry. Requires `--parent-uuid`. |
| `--has-stats` | | | `true` | Count the item in statistics. Pass `--has-stats=false` (with `=`) for a nested step. |

```sh
# Second attempt of a failed test: ReportPortal shows the first one as its retry
RETRY_UUID=$(gorp report start-test \
  --launch-uuid "$LAUNCH_UUID" --parent-uuid "$SUITE_UUID" \
  --name "TestBar" --type STEP --param "browser=chrome" --retry-of "$TEST_UUID")

# Nested step: shown inside the test, not counted in statistics
STEP_UUID=$(gorp report start-test \
  --launch-uuid "$LAUNCH_UUID" --parent-uuid "$TEST_UUID" \
  --name "open page" --type STEP --has-stats=false)
```

---

### report log

Report a log entry, optionally with a file attachment. Prints the log ID to stdout.

```sh
# Plain message
gorp report log --launch-uuid "$LAUNCH_UUID" --item-uuid "$TEST_UUID" \
  --message "Test output here" --level INFO

# With file attachment
gorp report log --launch-uuid "$LAUNCH_UUID" --item-uuid "$TEST_UUID" \
  --message "Failure screenshot" --level ERROR --file screenshot.png
```

| Flag | Short | Env var | Default | Description |
|------|-------|---------|---------|-------------|
| `--launch-uuid` | | `LAUNCH_UUID` | | **Required.** Launch UUID |
| `--item-uuid` | | `ITEM_UUID` | | Test item UUID (omit for launch-level log) |
| `--message` | `-m` | | | **Required.** Log message |
| `--level` | | | `INFO` | Log level: `DEBUG`, `INFO`, or `ERROR` |
| `--file` | `-f` | | | File path to attach |

---

### report finish-test

Finish a test item.

```sh
gorp report finish-test \
  --launch-uuid "$LAUNCH_UUID" --item-uuid "$TEST_UUID" --status PASSED
```

| Flag | Short | Env var | Default | Description |
|------|-------|---------|---------|-------------|
| `--launch-uuid` | | `LAUNCH_UUID` | | **Required.** Launch UUID |
| `--item-uuid` | | `ITEM_UUID` | | **Required.** Test item UUID |
| `--status` | | | | Item status: `PASSED`, `FAILED`, `SKIPPED`, etc. |
| `--issue-type` | | | | Defect type locator: `pb001` (product bug), `ab001` (automation bug), `si001` (system issue), `ti001` (to investigate), `nd001` (no defect), a custom locator of the project, or `NOT_ISSUE` |
| `--description` | | | | Item description. Replaces the one given at start. |
| `--attr` | `-a` | | | Attribute. Repeatable. Added to the ones given at start. |

ReportPortal applies `--issue-type` only to `FAILED` or `SKIPPED` items that are counted in statistics and are not
an earlier attempt of a retry. Without it such an item gets "To Investigate"; `NOT_ISSUE` leaves it without a defect.
An unknown locator is not reported back by the command: the item is not finished and stays in progress.

```sh
gorp report finish-test \
  --launch-uuid "$LAUNCH_UUID" --item-uuid "$TEST_UUID" --status FAILED --issue-type pb001
```

---

### report finish-launch

Finish a launch. Prints the launch UUID and, on the next line, the link to the launch in the ReportPortal UI.

```sh
gorp report finish-launch --launch-uuid "$LAUNCH_UUID" --status PASSED
```

| Flag | Short | Env var | Default | Description |
|------|-------|---------|---------|-------------|
| `--launch-uuid` | | `LAUNCH_UUID` | | **Required.** Launch UUID |
| `--status` | | | | Launch status: `PASSED`, `FAILED`, `STOPPED`, etc. |
| `--description` | | | | Text appended to the launch description given at start |
| `--attr` | `-a` | | | Attribute. Repeatable. Added to the ones given at start. |

---

### quality-gate check

Poll a launch for its quality gate result. One of `--launch-uuid` or `--stdin` is required.

```sh
# Check a specific launch UUID
gorp quality-gate check --launch-uuid 550e8400-e29b-41d4-a716-446655440000

# Parse launch UUID from a previous report run piped via stdin
go test -json ./... \
  | gorp report test2json --print-launch-uuid \
  | gorp quality-gate check --stdin
```

The `--stdin` mode scans its input for a line matching `ReportPortal Launch UUID: <uuid>`, which is exactly what `--print-launch-uuid` emits. This makes it easy to chain the two commands in a pipeline.

| Flag | Short | Env var | Default | Description |
|------|-------|---------|---------|-------------|
| `--launch-uuid` | | `LAUNCH_UUID` | | Launch UUID to check |
| `--stdin` | | | `false` | Read launch UUID from stdin |
| `--quality-gate-timeout` | `--qgt` | `QUALITY_GATE_TIMEOUT` | `1m` | Maximum time to wait |
| `--quality-gate-check-interval` | `--qgci` | `QUALITY_GATE_CHECK_INTERVAL` | `3s` | Poll interval |

Exits with code `10` if the quality gate status is not `PASSED`. See [Exit Codes](#exit-codes).

---

## CI/CD Integration

### GitHub Actions

```yaml
- name: Run tests and report to ReportPortal
  env:
    GORP_API_KEY: ${{ secrets.RP_API_KEY }}
    GORP_PROJECT: my_project
  run: |
    go install github.com/reportportal/goRP/v5@latest
    ln -s "$(go env GOPATH)/bin/goRP" "$(go env GOPATH)/bin/gorp"
    go test -json ./... | gorp report test2json \
      --host https://reportportal.example.com \
      --launchName "${{ github.workflow }} / ${{ github.ref_name }}" \
      --attr "build:${{ github.run_number }}" \
      --attr "branch:${{ github.ref_name }}" \
      --quality-gate-check
```

### Two-step pipeline with quality gate

Use `--print-launch-uuid` and `--stdin` to separate reporting from quality-gate enforcement (useful when you want the test run to finish before blocking the pipeline):

```sh
go test -json ./... \
  | gorp report test2json --print-launch-uuid \
  | gorp quality-gate check --stdin
```

Exit code `10` from the last command signals a quality gate failure to your CI system.

---

## Go Library

Import `pkg/gorp` to report results programmatically from your own Go code.

```sh
go get github.com/reportportal/goRP/v5
```

### Reporting client

`ReportingClient` sends results to the ReportPortal v2 reporting API. Pass a `context.Context` to every call so deadlines and cancellations are respected.

```go
import (
    "context"
    "net/url"
    "time"

    "github.com/reportportal/goRP/v5/pkg/gorp"
    "github.com/reportportal/goRP/v5/pkg/openapi"
)

func report(ctx context.Context) error {
    client := gorp.NewReportingClient(
        "https://reportportal.example.com",
        "my_project",
        gorp.WithApiKeyAuth(ctx, "your_api_key"),
    )

    // Start a launch
    launch, err := client.StartLaunch(ctx, &openapi.StartLaunchRQ{
        Name:      "My test run",
        StartTime: time.Now(),
        Mode:      openapi.PtrString(string(gorp.LaunchModes.Default)),
    })
    if err != nil {
        return err
    }
    launchUUID := launch.GetId()

    // Start a test item
    test, err := client.StartTest(ctx, &openapi.StartTestItemRQ{
        LaunchUuid: launchUUID,
        Name:       "TestSomething",
        Type:       string(gorp.TestItemTypes.Test),
        StartTime:  time.Now(),
    })
    if err != nil {
        return err
    }

    // Save a log entry
    _, err = client.SaveLog(ctx, &openapi.SaveLogRQ{
        LaunchUuid: launchUUID,
        ItemUuid:   test.Id,
        Level:      openapi.PtrString(gorp.LogLevelInfo),
        Time:       time.Now(),
        Message:    openapi.PtrString("Test passed"),
    })
    if err != nil {
        return err
    }

    // Finish the test item
    _, err = client.FinishTest(ctx, test.GetId(), &openapi.FinishTestItemRQ{
        LaunchUuid: launchUUID,
        Status:     gorp.Statuses.Passed.Ptr(),
        EndTime:    time.Now(),
    })
    if err != nil {
        return err
    }

    // Finish the launch
    _, err = client.FinishLaunch(ctx, launchUUID, &openapi.FinishExecutionRQ{
        Status:  gorp.Statuses.Passed.Ptr(),
        EndTime: time.Now(),
    })
    return err
}
```

### Read client (launch queries)

`Client` wraps the full generated OpenAPI client and adds higher-level helpers.

```go
u, _ := url.Parse("https://reportportal.example.com")
client := gorp.NewClient(u, gorp.WithApiKeyAuth(ctx, "your_api_key"))

// List the first page of launches
page, _, err := client.LaunchAPI.GetProjectLaunches(ctx, "my_project").
    PageSize(50).
    PageSort("startTime,DESC").
    Execute()

// Collect all launches across pages matching a filter string
all, err := client.GetAllLaunchesByFilterString(ctx, "my_project",
    "filter.eq.name=smoke&filter.eq.status=FAILED")
```

### Authentication

| Helper | Description |
|--------|-------------|
| `gorp.WithApiKeyAuth(ctx, apiKey)` | API key (user token) — recommended |
| `gorp.WithPasswordOwnerGrantAuth(ctx, cfg, user, pass)` | OAuth2 resource-owner password grant |

---

## Environment Variables

| Variable | CLI equivalent | Description |
|----------|----------------|-------------|
| `GORP_API_KEY` | `--api-key` | ReportPortal API key |
| `GORP_PROJECT` | `--project` | ReportPortal project name |
| `GORP_HOST` | `--host` | ReportPortal server URL |
| `GORP_CONFIG` | `--config` | Configuration file to use instead of `~/.gorp` |
| `LAUNCH_NAME` | `--launchName` | Launch name for `report test2json` |
| `FILE` | `--file` | Input file for `report test2json` |
| `REPORT_EMPTY_PKG` | `--reportEmptyPkg` | Report packages with no tests |
| `QUALITY_GATE_CHECK` | `--quality-gate-check` | Enable quality gate check after report |
| `QUALITY_GATE_TIMEOUT` | `--quality-gate-timeout` | Quality gate poll timeout (e.g. `2m`) |
| `QUALITY_GATE_CHECK_INTERVAL` | `--quality-gate-check-interval` | Quality gate poll interval (e.g. `5s`) |
| `LAUNCH_UUID` | `--launch-uuid` | Launch UUID for report step commands and `quality-gate check` |
| `ITEM_UUID` | `--item-uuid` | Test item UUID for `report log` and `report finish-test` |
| `PARENT_UUID` | `--parent-uuid` | Parent item UUID for `report start-test` |
| `FILTER` | `--filter` | Raw filter for `launch list` |
| `FILTER_NAME` | `--filter-name` | Saved filter name for launch queries |
| `MERGE_LAUNCH_IDS` | `--ids` | Comma-separated launch IDs for merge |
| `MERGE_LAUNCH_FILTER` | `--filter` | Raw filter for launch merge |
| `MERGE_LAUNCH_NAME` | `--name` | Result launch name for merge |
| `MERGE_TYPE` | `--type` | Merge strategy (`DEEP` or `BASIC`) |

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error (bad arguments, config missing, API error, etc.) |
| `10` | Quality gate check failed (status is not `PASSED`) |

---

## Compatibility

This version of the client targets ReportPortal **26.1+** (organizations API, TMS). For older ReportPortal versions use goRP **v5.2.x**.

---

## Development

```sh
task build      # bin/gorp
task test       # unit tests with coverage
task lint       # golangci-lint (Docker)
task e2e        # end-to-end tests against a real ReportPortal, not run by CI
```

`task e2e` reads credentials from a git-ignored `.gorp` file in the repository root (same JSON as `~/.gorp`) and
writes temporary `gorp-e2e-…` launches to the configured project, so point it at a sandbox project.
See [AGENTS.md](AGENTS.md) for the project layout, conventions and how to regenerate the OpenAPI client.
