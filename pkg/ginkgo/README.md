# Ginkgo v2 agent for ReportPortal

A [ReportPortal](https://reportportal.io) agent for [Ginkgo v2](https://github.com/onsi/ginkgo) — the Go BDD test framework.

Reports test results to ReportPortal in real time as each spec runs, including priorities, external test reference IDs, structured logs, and file attachments. Supports parallel execution across multiple Ginkgo processes.

## Requirements

- Go 1.25+ (the version in `go.mod`)
- Ginkgo v2 (`github.com/onsi/ginkgo/v2`)
- A running ReportPortal instance (v5+)

## Installation

```bash
go get github.com/reportportal/goRP/pkg/ginkgo
```

Install the `ginkgo-rp` wrapper binary:

```bash
go install github.com/reportportal/goRP/pkg/ginkgo/cmd/ginkgo-rp@latest
```

> `pkg/ginkgo` is a separate Go module inside goRP, so it does not add Ginkgo to the main goRP dependencies.
> It depends on a released goRP (`v5.2.2` or newer), not on the surrounding checkout. Once a
> `pkg/ginkgo/vX.Y.Z` tag is published, the commands above install that release. Until then
> `go install ...@latest` has no version to resolve; build from a checkout instead:
>
> ```bash
> cd pkg/ginkgo && go install ./cmd/ginkgo-rp
> ```

Also make sure the Ginkgo CLI is installed:

```bash
go install github.com/onsi/ginkgo/v2/ginkgo@latest
```

## Configuration

Create a `reportportal.properties` file in your project root:

```properties
rp.endpoint = https://your-reportportal-host
rp.api.key  = your_api_key_here
rp.project  = your_project_name
rp.launch   = MY_LAUNCH_NAME

# Optional: log HTTP traffic to rp-agent.log (default: false)
rp.debug = false
```

| Key           | Required | Description                        |
|---------------|----------|------------------------------------|
| `rp.endpoint` | Yes      | Full URL of your ReportPortal host (trailing slash and `/api` suffix are stripped automatically) |
| `rp.api.key`  | Yes      | API key from your user profile     |
| `rp.project`  | Yes      | Project slug in ReportPortal       |
| `rp.launch`   | No       | Name for the test launch (default `GINKGO-RP-LAUNCH`) |
| `rp.debug`    | No       | Set `true` to log all HTTP request/response bodies to `rp-agent.log` |
| `rp.description` | No    | Launch description                 |
| `rp.attributes`  | No    | Launch attributes: `key:value;tag;key2:value2` (a part without `:` is a bare tag) |
| `rp.mode`        | No    | `DEFAULT` (default; shown on the Launches page) or `DEBUG` |
| `rp.skipped.issue` | No  | `true` (default): a spec that calls `Skip("reason")` is filed under To Investigate. `false`: it is marked NOT_ISSUE (see "Skipped and Pending Specs") |

Every key can also be set by an environment variable, which **wins over the file**: `RP_ENDPOINT`, `RP_API_KEY`, `RP_PROJECT`, `RP_LAUNCH`, `RP_DEBUG`, `RP_DESCRIPTION`, `RP_ATTRIBUTES`, `RP_MODE`, `RP_SKIPPED_ISSUE`. If `RP_ENDPOINT`, `RP_PROJECT` and `RP_API_KEY` are all set, no properties file is needed (CI).

```bash
RP_ENDPOINT=https://rp.example.com RP_PROJECT=my_proj RP_API_KEY=$SECRET \
RP_ATTRIBUTES="build:$BUILD_ID;env:ci" ginkgo-rp -r ./...
```

Ginkgo `Label(...)` values (other than the `rp.*` control labels) are reported as bare-tag attributes on the spec item.

Add `reportportal.properties` to `.gitignore` — it contains your API key.

## Running Tests

> **`ginkgo-rp` covers test execution only.** It is not a full replacement for the Ginkgo CLI.
> Dev-time subcommands (`bootstrap`, `generate`, `outline`, `build`, `watch`, `unfocus`, `labels`) are not supported — run those with plain `ginkgo` directly.

Replace `ginkgo` with `ginkgo-rp` for test runs. All standard Ginkgo execution flags are passed through unchanged:

```bash
# Run all suites recursively
ginkgo-rp -r --keep-going

# Parallel execution (auto-detect CPU count)
ginkgo-rp -r -p --keep-going

# Fixed number of parallel workers
ginkgo-rp -r --procs 4 --keep-going

# With Ginkgo verbose output
ginkgo-rp -r --keep-going -v

# Filter by test name
ginkgo-rp -r --focus "login"

# Filter by label
ginkgo-rp -r --label-filter "P0"

# Run only one package
ginkgo-rp ./pkg/auth

# Dry run (no RP launch created)
ginkgo-rp --dry-run
```

When the launch has been finished, `ginkgo-rp` prints the link to it, for example `→ https://rp.example.com/ui/#my_project/launches/all/<id>`. The link is the one ReportPortal returns, so it is right for the ReportPortal version you run; if the server returns none, nothing is printed.

> **`--keep-going` is strongly recommended in CI.** Without it, if one suite fails Ginkgo stops early and the remaining suites are never reported to ReportPortal.

## Importing the Agent

Import the agent package (blank import) in your suite file so the `init()` hook registers the Ginkgo reporting hooks:

```go
// my_suite_test.go
package mypackage_test

import (
    "testing"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    _ "github.com/reportportal/goRP/pkg/ginkgo/rpagent"
)

func TestMySuite(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "My Suite")
}
```

## Features

### Test Priorities (P0–P4)

Tag tests with a priority level for better organisation in ReportPortal:

```go
import rpagent "github.com/reportportal/goRP/pkg/ginkgo/rpagent"

var _ = Describe("Checkout", func() {
    It("should complete payment",
        Label(rpagent.Priority(0)), // P0 = Critical
        func() {
            // test body
        })

    It("should show order history",
        Label(rpagent.Priority(3)), // P3 = Low
        func() {
            // test body
        })
})
```

| Priority | Meaning        |
|----------|----------------|
| P0       | Critical       |
| P1       | High           |
| P2       | Medium         |
| P3       | Low            |
| P4       | Lowest         |

A spec that has no priority label gets **no** `priority` attribute in ReportPortal, so the attribute can be used to filter: it marks only the specs where someone chose a priority. A priority set on a `Describe`/`Context` applies to the specs inside it. The `ginkgo-rp` summary at the end of a run still counts unlabelled specs as P1.

**Leaf label takes precedence.** When a `Describe` block and an `It` block both carry a priority label, the `It` label wins. This lets you set a default on a container and override it on individual tests.

### Test Reference IDs

Link a test to an external test management system (e.g. TestRail, Jira):

```go
It("should process refund",
    Label(rpagent.TestReference("PAY-TC-042")),
    func() { ... })
```

Combine with priority:

```go
Label(rpagent.Priority(1), rpagent.TestReference("PAY-TC-042"))
```

`TestReference("")` is safe — it emits a no-op placeholder label instead of the empty string that Ginkgo would reject. Characters that Ginkgo parses specially (`& | ! , ( ) /`) are automatically replaced with `_`.

### Structured Logs

Send log entries at specific levels directly to the test item in ReportPortal:

```go
It("should validate token", func() {
    rpagent.Log("INFO",  "Starting token validation")
    rpagent.Log("DEBUG", fmt.Sprintf("token length: %d", len(token)))
    rpagent.Log("ERROR", "token expired")
})
```

Valid levels: `DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`.

`rpagent.Log("FATAL", ...)` sends a FATAL-level log entry to RP and **returns normally** — it does not call `os.Exit`. Use it to flag catastrophic events in your test narrative without aborting the spec.

Logs must be called inside an `It` block. Logs called in `BeforeSuite` are silently dropped (Ginkgo flushes report entries only after the spec body completes).

### File Attachments

Attach any file (screenshot, log, JSON response) to the current test item:

```go
It("should render the dashboard", func() {
    // ... take screenshot ...
    rpagent.AddAttachment("screenshot.png", "Dashboard after login")
    rpagent.AddAttachment("api_response.json", "User API response")
})
```

- Must be called inside an `It` block.
- Maximum file size: **10 MB**.
- Files are streamed — not loaded fully into memory.
- Attachment payloads are JSON-encoded internally, so file paths containing `#` or spaces are handled correctly.
- Temp copies in `$TMPDIR` are cleaned up automatically after the suite finishes.
- A file that cannot be attached (missing, over the size limit, or not copyable) never fails the spec. An `ERROR` log is added to the item instead, for example `Attachment not uploaded: big.txt ("a big file"): the file is 10485761 bytes, the limit is 10485760 bytes`.

### By() Steps

`By()` statements inside a spec are reported as child STEP items in ReportPortal. Logs and attachments are routed to the `By()` child that was active when they were emitted:

```go
It("should complete checkout", func() {
    By("adding item to cart")
    rpagent.Log("INFO", "cart is empty")
    // logs and attachments here go to the "adding item to cart" step

    By("entering payment details")
    rpagent.AddAttachment("card_form.png", "Payment form")
    // logs and attachments here go to the "entering payment details" step

    By("confirming order")
    // ...
})
```

Steps, logs and attachments carry the times Ginkgo recorded while the spec ran, not the time they were uploaded. A step starts at its `By()` call and ends when the next step starts, when its `By(text, func(){...})` callback returns, or when the spec ends. A log or attachment has the time of the `rpagent.Log` / `rpagent.AddAttachment` call. A spec item ends at the spec's own end time. All times are clamped into the spec item's window, because ReportPortal requires children to start after their parent and end before it.

Each `By()` step has its own status. When a spec fails, is skipped or is interrupted, only the step that was running at that moment gets that status (plus any `By(text, func(){...})` callback step enclosing it); steps that were already past stay PASSED. The spec item itself always reports the spec's final status. The failure message is logged on that same running step, so it appears right under the red step; if no step was running, it is logged on the spec item. A skipped spec gets an informational `SKIPPED` entry with the reason (Ginkgo records `Skip("reason")` as a failure internally, but it is not reported as an error).

### GinkgoWriter and stdout Output

Whatever a spec writes to `GinkgoWriter` is sent to ReportPortal as `DEBUG` logs, on the item that was running when it was written: text before the first `By()` goes on the spec item, and text after a `By()` goes on that step. ANSI colour codes are removed. Ginkgo keeps no timestamp per write, so each piece carries the start time of the item it belongs to.

Ginkgo also captures what the spec printed to stdout and stderr, but only when it intercepts output, which it does in parallel runs (`-p`). In a serial run that stream is empty. When present it is sent as one `INFO` log on the spec item, with stdout and stderr merged in the order Ginkgo captured them.

For a spec that Ginkgo ran more than once, that log is labeled `[stdout/stderr of all N attempts]`, because the output of the attempts cannot be told apart (see "Flaky Specs").

Output is split into entries of at most 32 KiB, and at most 1 MiB per spec and stream is sent; if a spec writes more, a `WARN` log says how many bytes were omitted.

### Parameterized / Table-Driven Tests

`DescribeTable` entries are detected and reported automatically. Parameters appear in the RP test item when the `Entry` description follows `key='value'` format:

```go
DescribeTable("string operations",
    func(input, expected string) {
        Expect(strings.ToUpper(input)).To(Equal(expected))
    },
    Entry("input='hello' expected='HELLO'"),           // ✅ 2 parameters shown in RP
    Entry("input='world' expected='WORLD'"),           // ✅ 2 parameters shown in RP
    Entry("uppercase world"),                          // ❌ no key='value' → no parameters
)
```

The entry description is also used as the RP item name, so make it human-readable. Combine with priority:

```go
Entry("input='hello' expected='HELLO'", Label(rpagent.Priority(2))),
```

Detection rule: a spec is treated as a table entry if its name contains `=` and at least one `'` or `"` character. Parameters are extracted by the regex `(\w+)=['"]([^'"]*?)['"]`.

### Stable Test Identity (History)

ReportPortal links the same test across launches by its `testCaseId` (and `uniqueId`). The agent builds them only from data that does not change between runs:

```
testCaseId = <module path>/<package dir> > <Describe> > <Context> > <spec text>
codeRef    = <file path relative to the module root>:<line>
uniqueId   = UUIDv5 of the testCaseId
```

The format of both keys is part of this agent's contract: tests pin literal `testCaseId` and `uniqueId` values (and the UUIDv5 namespace), because changing either after a release would split every user's history.

For example `github.com/acme/shop/cart > Cart > Checkout > applies a coupon`. `By()` steps get `... > By: <step text>`. Nothing launch-specific (item IDs, launch IDs, absolute paths, timestamps) is part of the identity, so history, retries and auto-analysis work across runs and machines.

Renaming a spec, container or package starts a new history for it. Moving a spec within its file only changes `codeRef`.

Two specs with the same package, containers and text share one identity (and one history), because Ginkgo gives the agent no stable way to tell identical names apart. Their `codeRef` still differs, and the agent logs a warning naming both locations. Rename one of them to separate their history.

### Flaky Specs (FlakeAttempts) and MustPassRepeatedly

A spec that Ginkgo retries (`FlakeAttempts(n)`, `--flake-attempts`) is reported as ReportPortal **retries**: one item per attempt, each started as a retry of the previous one. The last attempt is the item you see and the only one counted in launch statistics; the earlier attempts are folded under it, each with its own failure message, `GinkgoWriter` output and `By()` steps. The failure of an earlier attempt is read from Ginkgo's `SpecReport.AdditionalFailures`, so it is complete even when the attempt failed in an `AfterEach` or a `DeferCleanup` rather than in the `It`.

`GinkgoWriter` output is split exactly per attempt. stdout/stderr cannot be: Ginkgo captures it for the whole spec (and only in parallel runs, see below) and records no offsets, so it cannot tell which attempt wrote what. The earlier attempts therefore get no stdout log, and the last attempt's `INFO` log holds the output of **all** attempts, labeled `[stdout/stderr of all N attempts]`. The same label is used for a spec with `MustPassRepeatedly`, whose repetitions share one item.

A failure that happened after a spec's main failure (a cleanup node that failed too, for example) is logged as an `ADDITIONAL FAILURE` error on the attempt it happened in. This also applies to specs that were not retried.

`MustPassRepeatedly(n)` is **not** a retry. In ReportPortal a retry means an earlier attempt failed, and a repeated spec has not failed, so reporting it as retries would mark it as flaky. The spec stays one item with its status, and an `INFO` log "Repetition k of n" marks where each repetition began; the output and `By()` steps of all repetitions are on that one item. If a repetition fails, Ginkgo stops there and the item is FAILED with that repetition's failure.

### Network Resilience (Retries)

While specs run, the agent's calls to ReportPortal (start/finish test item, logs, attachments) are retried up to **3 attempts** with exponential backoff and jitter (200 ms base). The `ginkgo-rp` wrapper's own calls (creating the launch and suites, finishing suites and the launch) are single attempts.

| Failure | Retried? |
|---------|----------|
| Network errors (connection refused/reset, timeout, EOF, DNS; detected by error type, not message text) | Yes |
| HTTP 5xx | Yes |
| HTTP 429 | Yes |
| Other HTTP 4xx (e.g. 400, 401, 404) | No, fails immediately |

Each item start request carries one client-generated UUID that is reused across attempts. If the server created the item but the response was lost, the retry is deduplicated by ReportPortal instead of creating a duplicate item.

If every attempt fails, the agent logs a warning and the test run continues; a ReportPortal outage during a run does not fail your specs.

The logs of one spec (`Log()` calls, `GinkgoWriter` and stdout output, the failure message) are sent together in batch requests of at most 200 logs or 2 MB, before the spec's items are finished. Attachments are uploaded one file at a time.

## Examples and Sanity Check

`examples/` contains three small suites (`calculator`, `order-management`, `rp-features`; 16 specs in total) that exercise every feature above against a real ReportPortal: priorities, test references, `By()` steps, all log levels, file attachments, failures with evidence, parameterized entries and skips.

They are behind the `sanity` build tag, so the intentional failures never run under a plain `go test ./...`.

1. Create `examples/reportportal.properties` with your endpoint, API key, project and launch name (it is git-ignored).
2. Run:

```bash
cd pkg/ginkgo/examples
ginkgo-rp --tags sanity -r -p --keep-going
```

Expected: 16 specs (12 passed, 3 failed, 1 skipped). The 3 failures are intentional, one each in `calculator`, `order-management` and `rp-features`. The command exits non-zero, and a launch appears in ReportPortal with the suites, steps, logs and attachments.

To run the same specs as plain Ginkgo, without ReportPortal:

```bash
go test -tags sanity ./examples/...
```

## Status Mapping

| Ginkgo state | ReportPortal status |
|-------------|---------------------|
| `passed`    | Passed              |
| `failed`    | Failed              |
| `panicked`  | Failed              |
| `skipped`   | Skipped             |
| `pending`   | Skipped             |
| `interrupted` | Interrupted       |
| `aborted`   | Interrupted         |
| `timedout`  | Interrupted         |

### Skipped and Pending Specs

ReportPortal files every SKIPPED item that has no issue under **To Investigate**, as if it were a failure nobody has looked at. Most skips need nobody's attention, so the agent finishes them with the issue type `NOT_ISSUE`, which keeps them out of To Investigate:

| Spec | Issue |
|------|-------|
| Pending (`PIt`, `XIt`, `Pending`) | `NOT_ISSUE` |
| Filtered out (`--focus`, `--skip`, `--label-filter`, ...) or skipped by Ginkgo itself (`--fail-fast`, an interrupt, a timeout): skipped without a message | `NOT_ISSUE` |
| Skipped because of another spec in an ordered container ("Spec skipped because ...") | `NOT_ISSUE` |
| Called `Skip("reason")` | left to ReportPortal, so To Investigate, unless `rp.skipped.issue=false` |

So only the spec that skipped itself, and the spec that failed, stay To Investigate. Set `rp.skipped.issue=false` (or `RP_SKIPPED_ISSUE=false`) when your `Skip(...)` calls are routine, for example a spec that skips itself on the wrong operating system. The launch records the setting in a system attribute, `skippedIssue`.

### Container and Suite Status

`Describe`/`Context` items are shared by every spec (and every parallel worker) beneath them, so the `ginkgo-rp` wrapper finishes them after all workers are done, deepest first. A container is `FAILED` if any spec beneath it failed, panicked, was interrupted or timed out; `SKIPPED` if every spec beneath it was skipped or pending; otherwise `PASSED`. It ends when the last spec beneath it ended, and a suite likewise ends when its last spec ended. Without this, ReportPortal closes them itself when the launch finishes, giving every container the launch's finish time.

### BeforeSuite and AfterSuite Failures

A failing `BeforeSuite` or `AfterSuite` (including the `Synchronized` variants) is reported as its own item under the suite: type `BEFORE_SUITE` or `AFTER_SUITE`, status `FAILED`, with an `ERROR` log holding the failure message and location. The suite and launch end `FAILED` even if every spec passed. ReportPortal does not count these configuration items in the launch statistics, so a launch can read "all passed" next to a failed status; the failed item inside the suite is where the cause is. The `ginkgo-rp` summary lists them under "Suite setup / teardown failures" and keeps them out of the test counts. When `BeforeSuite` fails Ginkgo does not run the specs, so the suite then contains only that failed item. Ginkgo reports how many specs it skipped but not their names, so the item's log says "N specs were not run because [BeforeSuite] failed." instead of listing them.

Ginkgo reports these nodes only at the end of the suite, so they appear in ReportPortal when the suite finishes, not while it runs. In a parallel run the same failure is reported once, not once per process.

### Temporary Files

During a run the agent keeps small coordination files in `$TMPDIR` (per-process results, the shared container cache, and an attachment staging directory created only when a spec attaches a file). Every name contains the run's session ID, and the `ginkgo-rp` wrapper removes exactly that run's files when it finishes, so parallel runs on one machine never touch each other's. If the wrapper is killed before it can clean up, the leftovers are `rp_*` files and `rp-agent-attachments-*` directories in `$TMPDIR`; they are safe to delete once no run is in progress.

## Environment Variables

| Variable                 | Default | Description |
|--------------------------|---------|-------------|
| `RP_SESSION_ID`          | —       | Set automatically by `ginkgo-rp`; do not set manually |
| `RP_PROPERTIES_DEPTH`    | 10      | Directory levels to walk up when searching for `reportportal.properties` |
| `RP_CONFIG_FILE`         | —       | Exact path of the properties file. Set automatically by `ginkgo-rp` for the workers; set it yourself to use a file outside the directory tree |
| `RP_STRICT`              | false   | `true` makes ReportPortal failures fail the run instead of only warning |
| `RP_FINALIZE_TIMEOUT`    | 3m      | How long `ginkgo-rp` may take to finish the launch after the tests ran, as a Go duration (`90s`, `5m`). When it runs out, the launch is stopped instead (see "Interrupts and aborts") |
| `RP_LOCK_RETRY_COUNT`    | 50      | How many times a losing worker polls waiting for the lock winner to write the shared cache file. Default = 10 s total (50 × 200 ms). Increase for 100+ workers or slow disks. |
| `RP_LOCK_RETRY_SLEEP_MS` | 200     | Milliseconds between each poll. Total max wait = `COUNT × SLEEP_MS`. For normal use (≤16 workers, fast network) defaults are fine. |

## Why ginkgo-rp instead of ginkgo?

**Ginkgo's reporter hooks only fire inside the worker process that runs tests.** They have no way to create the RP launch before tests start, or finalize it after all tests finish.

Every test reporting system needs three phases:

| Phase | What happens | Who owns it |
|-------|-------------|-------------|
| ① Before tests | Create RP launch, create suite items, write shared state to `$TMPDIR` | `ginkgo-rp` wrapper |
| ② During tests | `ReportBeforeEach`/`ReportAfterEach` hooks stream each spec's result in real time | `rpagent` plugin (inside Ginkgo worker) |
| ③ After tests | Read per-worker NDJSON files, finish Describe/Context and suite items, finalize launch with pass/fail | `ginkgo-rp` wrapper |

The `ginkgo` binary only covers phase ②.

**Why not `BeforeSuite` / `AfterSuite`?**

- **Parallel execution breaks it.** With `ginkgo -p`, N worker processes each run `BeforeSuite` independently — every worker would create its own launch in RP, giving you N duplicates instead of one.
- **Multi-suite runs have no shared scope.** `AfterSuite` runs once per suite directory. When running `ginkgo -r` across multiple packages, there is no hook that fires after *all* suites finish — so you can never finalize the launch with complete aggregate stats.

**How it works:**

```
ginkgo-rp
  │
  ├─ Phase 1 (wrapper):  POST /launch  →  POST /item (SUITE × N)
  │                      write $TMPDIR/rp_agent_state_{session}.json  [no API key stored]
  │                      set RP_SESSION_ID env var
  │
  ├─ Phase 2 (worker):   run ginkgo as a child process and wait for it
  │                      (SIGINT/SIGTERM are passed on to it, see "Interrupts and aborts")
  │                        └─ init() sees RP_SESSION_ID → re-reads API key from
  │                             reportportal.properties (not from state file)
  │                             registers ReportBeforeEach / ReportAfterEach hooks
  │                             ReportBeforeEach → POST Describe/It items to RP
  │                             ReportAfterEach  → PUT finish, POST logs/attachments
  │                             appends NDJSON to $TMPDIR/rp_status_{session}_{pid}.json
  │                             ReportAfterSuite → write done-flag, clean up temp files
  │
  └─ Phase 3 (wrapper):  ginkgo exited → all workers already exited
                         wait up to 5s for the workers' done-flags → read NDJSON files
                         PUT finish suite items  →  PUT /launch/finish
```

**API key security:** The API key is never written to `$TMPDIR`. The state file contains only endpoint, project, launch ID, and session ID. Workers re-read the API key independently from `reportportal.properties`.

**`--dry-run` and dormancy:** When `--dry-run` is passed, `ginkgo-rp` skips Phase 1 entirely — no launch is created and `RP_SESSION_ID` is never set. If a worker's `init()` cannot find the state file (e.g. under `--dry-run`, or when running plain `ginkgo`), it logs a warning and stays dormant. Running `go test ./...` never makes any HTTP calls to ReportPortal.

**Suite discovery:** `ginkgo-rp` decides for itself which packages to register in ReportPortal; it does not ask Ginkgo. A package is a suite when one of its `*_test.go` files imports `github.com/onsi/ginkgo` (any version or subpackage) **and** the Go build would compile that file: build constraints are applied, so `--tags` (and `//go:build` lines, `GOOS`/`GOARCH` suffixes) decide which files count. With `-r`, subdirectories are searched too, except `vendor`, `testdata` and directories whose name starts with `.` or `_`. Packages excluded by `--skip-package` are not listed either (the filter is matched against the path as Ginkgo writes it, `./relative/path`), so they never appear as empty suites. The suite name is the `RunSpecs` description, taken from a string literal or a package-level string constant; otherwise the directory name is used. Flags are split from package paths with Ginkgo's own flag definitions, so flag values such as `--tags sanity` are never mistaken for paths. Targets outside the working directory are allowed: the wrapper passes the location of the properties file it used to the workers (`RP_CONFIG_FILE`). If a package's tests run but it was not registered by `ginkgo-rp`, its worker logs an `ERROR ... will NOT be reported` line (tests are never aborted).

**How discovery differs from Ginkgo's own:** Ginkgo runs a regular expression over the text of every `_test.go` file in a directory. It does not evaluate build constraints, so an import path that only appears in a comment or a string also counts, and `-r` does descend into `testdata`. In practice the two agree, with one edge case worth knowing: a package whose Ginkgo test files are **all** excluded by build constraints (for example behind `//go:build integration`, run without `-tags integration`) is still a suite to Ginkgo, which then fails to compile it ("build constraints exclude all Go files"), but it is not registered in ReportPortal, and `ginkgo-rp` prints a warning that no suite was found there. Nothing is reported for it, and if no suite at all is found, the tests still run with reporting off. Pass the same `--tags` you use to run your tests and the two agree.

**If ReportPortal is unreachable:** reporting problems never decide the result of your test run. If the launch cannot be created, `ginkgo-rp` prints a warning, runs the tests without reporting, and exits with Ginkgo's exit code. If finalizing the launch fails after the tests ran, it warns and still exits with Ginkgo's exit code. Set `RP_STRICT=true` to fail the run instead (exit code 1 before the tests, 2 after them).

**Interrupts and aborts:** on Ctrl-C (SIGINT), SIGTERM or SIGHUP (a closing terminal), `ginkgo-rp` passes the signal to ginkgo and its test processes (SIGHUP as SIGTERM, which Ginkgo handles gracefully), waits for Ginkgo's own cleanup, finalizes the launch, and exits with 130 (SIGINT), 143 (SIGTERM) or 129 (SIGHUP). A signal that arrives before the tests have started closes the launch too. A second signal kills ginkgo and its test processes immediately. On Windows a signal cannot be passed on, so ginkgo is stopped at once and the launch is finalized. The spec that was running is reported as INTERRUPTED with Ginkgo's message, and the launch is FAILED. If `ginkgo-rp` has to give up after creating the launch (for example `ginkgo` is not on `PATH`), it finishes the suites and then the launch as STOPPED, so nothing stays In Progress. The same applies while the launch is being finished after the tests: that step is limited by `RP_FINALIZE_TIMEOUT` (default 3 minutes) so a ReportPortal that stops answering cannot hold the run for hours, and it gives up early when three finish requests in a row fail. Ctrl-C during it stops the launch as STOPPED and exits with 130; a second Ctrl-C gives up at once.

**Job control and stdin:** `ginkgo-rp` starts ginkgo in its own process group (see above), so the terminal's Ctrl-Z would reach only the wrapper. The wrapper therefore passes SIGTSTP on to ginkgo's whole group and stops itself, and passes the SIGCONT of `fg` or `bg` on as well: Ctrl-Z suspends the wrapper and the running tests together, as it does with plain `ginkgo`, and `fg`/`bg` resume them (your shell may report the wrapper as stopped by SIGSTOP). Test processes started by Ginkgo have no stdin (a read returns EOF), with or without `ginkgo-rp`, so the separate process group cannot make a test block reading the terminal. On Windows there is no job control and none of this applies.

**Name and ID limits:** ReportPortal rejects an item name longer than 1,024 characters (HTTP 400, the item is never created), so a longer name is cut and ends in `...`. It stores at most 1,024 characters of a `testCaseId` and silently cuts the rest, which would make two long IDs with a common start identical. So a longer `testCaseId` is cut and ends in `~` plus 16 hex characters of a hash of the full ID: it stays deterministic between runs and different for different tests. The `uniqueId` is always the UUIDv5 of the full `testCaseId`, before any cut, so two long IDs that differ only after the cut still get different `uniqueId`s.

## How Parallel Execution Works

Parallel Ginkgo processes coordinate using a two-tier cache:

- **L1 (in-process):** `sync.Map` — instant lookups within a single worker process.
- **L2 (cross-process):** a shared lock file (`rp_cache_{key}.lock`) claimed with `O_EXCL`. The winner creates the RP item, writes the ID to a `.tmp` file, syncs, and atomically renames it to the final cache path. All other workers wait for the rename and read back the same ID.

This ensures `Describe`/`Context` container items are created exactly once in ReportPortal even when multiple parallel workers encounter the same parent block simultaneously. Containers are not finished per-spec — the wrapper closes them all in Phase 3 after all workers have exited.

Tune the lock with `RP_LOCK_RETRY_COUNT` and `RP_LOCK_RETRY_SLEEP_MS` if you run very high worker counts.

## License

Apache 2.0 — see [LICENSE](LICENSE).
