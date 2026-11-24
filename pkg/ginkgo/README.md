# Ginkgo v2 agent for ReportPortal

A [ReportPortal](https://reportportal.io) agent for [Ginkgo v2](https://github.com/onsi/ginkgo) — the Go BDD test framework.

Reports test results to ReportPortal in real time as each spec runs, including priorities, external test reference IDs, structured logs, and file attachments. Supports parallel execution across multiple Ginkgo processes.

## Requirements

- Go 1.21+
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
| `rp.launch`   | Yes      | Name for the test launch           |
| `rp.debug`    | No       | Set `true` to log all HTTP request/response bodies to `rp-agent.log` |

| `rp.description` | No    | Launch description                 |
| `rp.attributes`  | No    | Launch attributes: `key:value;tag;key2:value2` (a part without `:` is a bare tag) |
| `rp.mode`        | No    | `DEFAULT` (shown on the Launches page) or `DEBUG` |

Every key can also be set by an environment variable, which **wins over the file**: `RP_ENDPOINT`, `RP_API_KEY`, `RP_PROJECT`, `RP_LAUNCH`, `RP_DEBUG`, `RP_DESCRIPTION`, `RP_ATTRIBUTES`, `RP_MODE`. If `RP_ENDPOINT`, `RP_PROJECT` and `RP_API_KEY` are all set, no properties file is needed (CI).

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
| P1       | High (default) |
| P2       | Medium         |
| P3       | Low            |
| P4       | Lowest         |

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

For example `github.com/acme/shop/cart > Cart > Checkout > applies a coupon`. `By()` steps get `... > By: <step text>`. Nothing launch-specific (item IDs, launch IDs, absolute paths, timestamps) is part of the identity, so history, retries and auto-analysis work across runs and machines.

Renaming a spec, container or package starts a new history for it. Moving a spec within its file only changes `codeRef`.

Two specs with the same package, containers and text share one identity (and one history), because Ginkgo gives the agent no stable way to tell identical names apart. Their `codeRef` still differs, and the agent logs a warning naming both locations. Rename one of them to separate their history.

### Network Resilience (Retries)

While specs run, the agent's calls to ReportPortal (start/finish test item, logs, attachments) are retried up to **3 attempts** with exponential backoff and jitter (200 ms base). The `ginkgo-rp` wrapper's own calls (creating the launch and suites, finishing suites and the launch) are single attempts.

| Failure | Retried? |
|---------|----------|
| Network errors (connection refused/reset, timeout, EOF, DNS) | Yes |
| HTTP 5xx | Yes |
| HTTP 429 | Yes |
| Other HTTP 4xx (e.g. 400, 401, 404) | No, fails immediately |

Each item start request carries one client-generated UUID that is reused across attempts. If the server created the item but the response was lost, the retry is deduplicated by ReportPortal instead of creating a duplicate item.

If every attempt fails, the agent logs a warning and the test run continues; a ReportPortal outage during a run does not fail your specs.

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
  ├─ Phase 2 (worker):   exec("ginkgo", ...) — blocking call
  │                        └─ init() sees RP_SESSION_ID → re-reads API key from
  │                             reportportal.properties (not from state file)
  │                             registers ReportBeforeEach / ReportAfterEach hooks
  │                             ReportBeforeEach → POST Describe/It items to RP
  │                             ReportAfterEach  → PUT finish, POST logs/attachments
  │                             appends NDJSON to $TMPDIR/rp_status_{session}_{pid}.json
  │                             ReportAfterSuite → write done-flag, clean up temp files
  │
  └─ Phase 3 (wrapper):  cmd.Run() returned → all workers already exited
                         5s OS-flush wait → read NDJSON files
                         PUT finish suite items  →  PUT /launch/finish
```

**API key security:** The API key is never written to `$TMPDIR`. The state file contains only endpoint, project, launch ID, and session ID. Workers re-read the API key independently from `reportportal.properties`.

**`--dry-run` and dormancy:** When `--dry-run` is passed, `ginkgo-rp` skips Phase 1 entirely — no launch is created and `RP_SESSION_ID` is never set. If a worker's `init()` cannot find the state file (e.g. under `--dry-run`, or when running plain `ginkgo`), it logs a warning and stays dormant. Running `go test ./...` never makes any HTTP calls to ReportPortal.

**Suite discovery:** A suite is any directory with a `*_test.go` file that calls `RunSpecs` (the file need not be named `*_suite_test.go`). The suite name is the `RunSpecs` description, taken from a string literal or a package-level string constant; otherwise the directory name is used. Targets outside the working directory are allowed. If a package's tests run but it was not registered by `ginkgo-rp`, its worker logs an `ERROR ... will NOT be reported` line (tests are never aborted).

## How Parallel Execution Works

Parallel Ginkgo processes coordinate using a two-tier cache:

- **L1 (in-process):** `sync.Map` — instant lookups within a single worker process.
- **L2 (cross-process):** a shared lock file (`rp_cache_{key}.lock`) claimed with `O_EXCL`. The winner creates the RP item, writes the ID to a `.tmp` file, syncs, and atomically renames it to the final cache path. All other workers wait for the rename and read back the same ID.

This ensures `Describe`/`Context` container items are created exactly once in ReportPortal even when multiple parallel workers encounter the same parent block simultaneously. Containers are not finished per-spec — the wrapper closes them all in Phase 3 after all workers have exited.

Tune the lock with `RP_LOCK_RETRY_COUNT` and `RP_LOCK_RETRY_SLEEP_MS` if you run very high worker counts.

## License

Apache 2.0 — see [LICENSE](LICENSE).
