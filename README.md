![Build Status](https://github.com/reportportal/goRP/workflows/Build/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/reportportal/goRP)](https://goreportcard.com/report/github.com/reportportal/goRP)
[![License MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/reportportal/goRP/master/LICENSE)
[![Release](https://img.shields.io/github/release/reportportal/goRP.svg)](https://github.com/reportportal/goRP/releases/latest)
[![GitHub Releases Stats of goRP](https://img.shields.io/github/downloads/reportportal/goRP/total.svg?logo=github)](https://somsubhra.github.io/github-release-stats/?username=reportportal&repository=gorP)

# goRP

Compact CLI, reporter, and Go client for [ReportPortal](https://reportportal.io). Examples use the `gorp` binary name (the release artifact is `goRP`).

## Installation
- Go toolchain
```sh
go install github.com/reportportal/goRP/v5@latest
```
- Release tarball (auto-detects your OS/arch)
```sh
curl -s https://api.github.com/repos/reportportal/goRP/releases/latest | \
  jq -r '.assets[] | select(.name | contains("tar.gz")) | .browser_download_url' | \
  grep "$(uname)_$(arch)" | \
  xargs curl -sL | tar zx -C /usr/local/bin
```

## Configure credentials
Run `gorp init` once to cache `host`, `uuid`, and `project` in `~/.gorp`. The wizard asks for:
1. ReportPortal base URL
2. Personal access token (UUID)
3. Default project name

Runtime overrides go through flags or env vars: `GORP_UUID`, `GORP_PROJECT`, and `GORP_HOST`. All commands accept `--log-level` (default `debug`).

## CLI overview
```
gorp [global flags] command [command options]

GLOBAL FLAGS:
  --uuid, -u         Access token (env: GORP_UUID)
  --project, -p      Project name (env: GORP_PROJECT)
  --host             ReportPortal server URL
  --log-level        slog level (trace|debug|info|warn|error)
```
```
COMMANDS:
  init            Initialize the local config cache
  launch          List or merge launches
  report          Ship test results (currently only Go test2json)
  quality-gate    Poll a launch quality gate status
```

## Launch operations
`gorp launch list [--filter key=value ... | --filter-name saved_filter]`
- `--filter` (`Filter` env) joins multiple key=value pairs with `&`.
- `--filter-name` (`FILTER_NAME` env) reuses a saved ReportPortal filter.
- No flags lists the most recent launches for the configured project.

`gorp launch merge --name "Nightly" [--ids 1 --ids 2 | --filter ... | --filter-name ...]`
- Provide either explicit `--ids` or a filter; otherwise the command exits.
- `--type` (`MERGE_TYPE`, default `DEEP`) controls the merge strategy.

## Reporting Go tests
`gorp report test2json [-f results.json] [flags]`
- Consumes `go test -json` output from a file or stdin.
- `--launchName, --ln` (`LAUNCH_NAME`, default `"gorp launch"`).
- `--reportEmptyPkg, --ep` (`REPORT_EMPTY_PKG`) creates suite entries even when packages have no tests.
- `--attr, -a key:value` attaches launch attributes; omitting `:` creates a tag.
- `--print-launch-uuid` writes `ReportPortal Launch UUID:<uuid>` for downstream tooling.
- `--quality-gate-check, --qgc` waits for the launch quality gate using the timeout/interval flags described below (exit code `10` on failure).

### Go test workflow
```
go test -json ./... > results.txt
 gorp report test2json -f results.txt --attr build:123 --print-launch-uuid
```
Direct streaming works as well:
```
go test -json ./... | gorp report test2json --attr env:ci
```

## Quality gate checks
`gorp quality-gate check [--launch-uuid UUID | --stdin] [--quality-gate-timeout 1m] [--quality-gate-check-interval 3s]`
- Supply the launch ID explicitly (`--launch-uuid`, env `LAUNCH_UUID`) or pipe the output of `gorp report ... --print-launch-uuid` and use `--stdin`.
- The command polls until the launch metadata exposes a quality gate result. It exits with code `0` on pass, `10` on fail, and `1` on timeouts/usage errors.

Use `gorp quality-gate check --stdin <<<'ReportPortal Launch UUID: <id>'` to test the flow locally.
