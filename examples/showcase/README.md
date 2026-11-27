# Showcase: a rich launch reported with `pkg/gorp`

`showcase` reports a deterministic "Payments API" regression to ReportPortal using the `pkg/gorp` reporting client directly. It covers most of what an agent can send, so it doubles as a reference for writing Go integrations.

| Feature | Where in the launch |
|---|---|
| Launch attribute contract (`env`, `build`, `version`, `branch`, `suite`, `platform`, `team`, `trigger`) and a Markdown description with a CI link | launch |
| System attributes `agent`, `os`, `go`, `skippedIssue` | launch info |
| Suites, nested `TEST` containers (up to three levels), `STEP` tests | `payments-api`, `ledger-service` |
| Stable identity: `testCaseId` (with the parameter variant in it), `codeRef` | every step |
| Parameters | `full refund` (`currency`, `amount`), `convert` (`from`, `to`), `… account` (`kind`) |
| Item attributes (`feature`, `priority`, `owner`, `tms_id`) and descriptions | tests and containers |
| `BEFORE_SUITE`, `BEFORE_METHOD`, `AFTER_METHOD` fixtures | `payments-api`, `Refunds` |
| Nested steps without statistics (`hasStats=false`) | `partial refund` |
| All log levels, a Markdown log, attachments (PNG, JSON, text) in multipart batches | `payload trace`, failures |
| Retries: two failed attempts (finished as No Defect), then a pass, chained with `retryOf` | `delivery retries on 503` |
| Skipped as No Defect and skipped as To Investigate | `signature rotation`, `export to S3` |
| Defect types on finish: Product Bug, Automation Bug, System Issue (with comments); To Investigate left on purpose | failures |
| History across builds: a regression appears in build 102, a bug is fixed in 102 and comes back in 103 | `-build` |
| Launch reruns (`rerun` + `rerunOf`): the failed tests are reported again, become retries of the old items, and the server recomputes the statistics | `-mode rerun1`, `-mode rerun2` |

## Run

```sh
export GORP_HOST=https://reportportal.example.com GORP_PROJECT=my_project GORP_API_KEY=...

go run ./examples/showcase -build 101
go run ./examples/showcase -build 102
go run ./examples/showcase -build 103        # prints "ReportPortal Launch UUID: <uuid>" and the launch link

# two reruns of build 103 after fixes; the server keeps the UUID of the original launch
go run ./examples/showcase -build 103 -mode rerun1 -rerun-of <uuid>
go run ./examples/showcase -build 103 -mode rerun2 -rerun-of <uuid>
```

Use a sandbox project: each run creates a launch named `Go Showcase - Payments API`.

A rerun reports only some tests, so it finishes the launch **without** a status and lets the server compute it from all items, as the Java agents do. `-mode refinish -rerun-of <uuid>` reopens a launch as a rerun without reporting any test, so the server recomputes its status.

On ReportPortal 6.x, `GORP_PROJECT` is the project key, for example `superadmin_personal` (not the slug shown in the UI URL).
