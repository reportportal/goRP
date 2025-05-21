![Build Status](https://github.com/reportportal/goRP/workflows/Build/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/reportportal/goRP)](https://goreportcard.com/report/github.com/reportportal/goRP)
[![License MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/reportportal/goRP/master/LICENSE)
[![Release](https://img.shields.io/github/release/reportportal/goRP.svg)](https://github.com/reportportal/goRP/releases/latest)
[![GitHub Releases Stats of goRP](https://img.shields.io/github/downloads/reportportal/goRP/total.svg?logo=github)](https://somsubhra.github.io/github-release-stats/?username=reportportal&repository=gorP)

# goRP

Golang Client, Reporter and CLI Utility for [ReportPortal](https://reportportal.io)

1. [Installation](#installation)
2. [CLI Usage](#usage)
3. [Go Test Reporter](#using-as-golang-test-results-agent)

## Installation

- Via Go Install
```sh
go install github.com/reportportal/goRP@latest
```
- Via cURL (passing version and arch)
```sh
curl -sL https://github.com/avarabyeu/goRP/releases/download/v5.0.2/goRP_5.0.2_darwin_amd64.tar.gz | tar zx -C .
```
- Via cURL (latest one)
```sh
curl -s https://api.github.com/repos/reportportal/goRP/releases/latest | \
  jq -r '.assets[] | select(.name | contains ("tar.gz")) | .browser_download_url' | \
  grep "$(uname)_$(arch)" | \
  xargs curl -sL |  tar zx -C .
```
## Usage

```
gorp [global options] command [command options] [arguments...]   

COMMANDS:
   launch   Operations over launches
   report   Reports input to report portal
   init     Initializes configuration cache
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --uuid value, -u value     Access Token [$GORP_UUID]
   --project value, -p value  ReportPortal Project Name [$GORP_PROJECT]
   --host value               ReportPortal Server Name
   --help, -h                 show help (default: false)
   --version, -v              print the version (default: false)
```

### Init command

    NAME:
        gorp init - Initializes configuration cache
    USAGE:
        gorp init [command options] [arguments...]
    OPTIONS:
        --help, -h  show help (default: false)

### Launch command

```
USAGE:
   goRP launch command [command options] [arguments...]

COMMANDS:
   list     List launches
   merge    Merge Launches
   help, h  Shows a list of commands or help for one command
```

#### List Launches

```
USAGE:
   goRP launch list [command options] [arguments...]

OPTIONS:
   --filter-name value, --fn value  Filter Name [$FILTER_NAME]
   --filter value, -f value         Filter [$Filter]
   --help, -h                       show help (default: false)
```

### Report command
```
    NAME:
        goRP report - Reports input to report portal
    USAGE:
        goRP report command [command options] [arguments...]
    COMMANDS:
        test2json  Input format: test2json
    OPTIONS:
        --help, -h  show help (default: false)
```

#### Report test2json command
```
NAME:
   goRP report test2json - Input format: test2json

USAGE:
   goRP report test2json [command options]

OPTIONS:
   --file string, -f string                               File Name [$FILE]
   --launchName string, --ln string                       Launch Name (default: "gorp launch") [$LAUNCH_NAME]
   --reportEmptyPkg, --ep                                 Whether empty packages need to be reporter. Default is false (default: false) [$REPORT_EMPTY_PKG]
   --attr string, -a string [ --attr string, -a string ]  Launch attribute with format 'key:value'. Omitting a ':' separator will tag the launch with the value.
   --quality-gate-check, --qgc                            Check quality gate status. Exits with exit code 10 if quality gate check fails. (default: false) [$QUALITY_GATE_CHECK]
   --help, -h                                             show help
```

## Using as Golang Test Results Agent
Run tests with JSON output
```
go test -json ./... > results.txt
```
Report The results
```
gorp report test2json -f results.txt
```
Report directly from go test output
```
go test -json ./... | gorp report test2json
```