// Command showcase reports a rich, deterministic "Payments API" regression launch to ReportPortal
// with the goRP library: an attribute contract, system attributes, suites, nested containers,
// parametrized steps, nested no-stats steps, before/after fixtures, every log level, attachments,
// retries with a retryOf chain, skipped items, defect types on finish, and launch reruns (rerunOf).
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/reportportal/goRP/v5/pkg/gorp"
	"github.com/reportportal/goRP/v5/pkg/openapi"
)

const (
	launchName = "Go Showcase - Payments API"
	module     = "github.com/example/payments"
)

// run modes
const (
	modeFull     = "full"     // the nightly regression: everything
	modeRerun1   = "rerun1"   // a rerun of the failed tests after a hotfix: two of them now pass
	modeRerun2   = "rerun2"   // a second rerun after the environment was fixed: the timeout passes too
	modeRefinish = "refinish" // reopen the launch as a rerun with no tests, so the server recomputes its status
	agentName    = "gorp-showcase"
	agentVer     = "1.0.0"
	ciURLFormat  = "https://ci.example.com/job/payments-nightly/%d/"
)

type reporter struct {
	ctx    context.Context
	c      *gorp.ReportingClient
	launch string
	clock  time.Time
	mode   string
	build  int
}

// tick advances the reporting clock, so items get realistic, strictly ordered times.
func (r *reporter) tick(d time.Duration) time.Time {
	r.clock = r.clock.Add(d)
	return r.clock
}

func attr(k, v string) openapi.ItemAttributesRQ {
	return openapi.ItemAttributesRQ{Key: openapi.PtrString(k), Value: v}
}

func params(kv ...string) []openapi.ComEpamReportportalBaseReportingParameterResource {
	var out []openapi.ComEpamReportportalBaseReportingParameterResource
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, openapi.ComEpamReportportalBaseReportingParameterResource{
			Key: kv[i], Value: openapi.PtrString(kv[i+1]),
		})
	}
	return out
}

type item struct {
	name, typ, codeRef, testCaseID, description string
	attrs                                       []openapi.ItemAttributesRQ
	params                                      []openapi.ComEpamReportportalBaseReportingParameterResource
	noStats                                     bool
	retryOf                                     string
}

func (r *reporter) start(parent string, it item) string {
	rq := &openapi.StartTestItemRQ{
		Uuid:       newUUID(),
		Name:       it.name,
		StartTime:  r.tick(50 * time.Millisecond),
		LaunchUuid: r.launch,
		Type:       it.typ,
		HasStats:   openapi.PtrBool(!it.noStats),
		Attributes: it.attrs,
		Parameters: it.params,
	}
	if it.codeRef != "" {
		rq.CodeRef = openapi.PtrString(it.codeRef)
	}
	if it.testCaseID != "" {
		rq.TestCaseId = openapi.PtrString(it.testCaseID)
	}
	if it.description != "" {
		rq.Description = openapi.PtrString(it.description)
	}
	if it.retryOf != "" {
		rq.Retry = openapi.PtrBool(true)
		rq.RetryOf = openapi.PtrString(it.retryOf)
	}
	var (
		rs  *openapi.EntryCreatedAsyncRS
		err error
	)
	if parent == "" {
		rs, err = r.c.StartTest(r.ctx, rq)
	} else {
		rs, err = r.c.StartChildTest(r.ctx, parent, rq)
	}
	if err != nil {
		log.Fatalf("start %q: %v", it.name, err)
	}
	return rs.GetId()
}

func (r *reporter) finish(
	id string,
	status gorp.Status,
	issue *openapi.ComEpamReportportalBaseReportingIssue,
) {
	rq := &openapi.FinishTestItemRQ{
		EndTime:    r.tick(120 * time.Millisecond),
		Status:     status.Ptr(),
		LaunchUuid: r.launch,
		Issue:      issue,
	}
	if _, err := r.c.FinishTest(r.ctx, id, rq); err != nil {
		log.Fatalf("finish %s: %v", id, err)
	}
}

func issue(locator, comment string) *openapi.ComEpamReportportalBaseReportingIssue {
	is := openapi.NewComEpamReportportalBaseReportingIssue(locator)
	if comment != "" {
		is.Comment = openapi.PtrString(comment)
	}
	return is
}

type logLine struct {
	level, msg string
	file       *gorp.ReaderMultipart
}

func (r *reporter) logs(id string, lines ...logLine) {
	var (
		batch []*openapi.SaveLogRQ
		files []gorp.Multipart
	)
	for _, l := range lines {
		rq := &openapi.SaveLogRQ{
			Uuid:       openapi.PtrString(newUUID()),
			ItemUuid:   openapi.PtrString(id),
			LaunchUuid: r.launch,
			Time:       r.tick(10 * time.Millisecond),
			Level:      openapi.PtrString(l.level),
			Message:    openapi.PtrString(l.msg),
		}
		if l.file != nil {
			rq.File = &openapi.ComEpamReportportalBaseReportingSaveLogRQFile{
				Name: openapi.PtrString(l.file.FileName),
			}
			files = append(files, l.file)
		}
		batch = append(batch, rq)
	}
	if _, err := r.c.SaveLogMultipart(r.ctx, batch, files); err != nil {
		log.Fatalf("logs for %s: %v", id, err)
	}
}

func jsonFile(name string, v any) *gorp.ReaderMultipart {
	b, _ := json.MarshalIndent(v, "", "  ")
	return &gorp.ReaderMultipart{
		FileName:    name,
		ContentType: "application/json",
		Reader:      bytes.NewReader(b),
	}
}

func textFile(name, s string) *gorp.ReaderMultipart {
	return &gorp.ReaderMultipart{
		FileName:    name,
		ContentType: "text/plain",
		Reader:      strings.NewReader(s),
	}
}

// screenshot draws a small "error page" so failures carry a real image attachment.
func screenshot(name string, c color.RGBA) *gorp.ReaderMultipart {
	img := image.NewRGBA(image.Rect(0, 0, 320, 180))
	for y := 0; y < 180; y++ {
		for x := 0; x < 320; x++ {
			switch {
			case y < 28:
				img.Set(x, y, color.RGBA{40, 44, 52, 255})
			case x > 40 && x < 280 && y > 60 && y < 120:
				img.Set(x, y, c)
			default:
				img.Set(x, y, color.RGBA{245, 246, 248, 255})
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return &gorp.ReaderMultipart{FileName: name, ContentType: "image/png", Reader: &buf}
}

// runs reports which tests run in this mode: a full run reports everything, a rerun only the tests
// that failed before.
func (r *reporter) runs(name string) bool {
	switch r.mode {
	case modeRerun1:
		return name == "full refund[GBP]" || name == "partial refund" || name == "convert[JPY]"
	case modeRerun2:
		return name == "convert[JPY]"
	case modeRefinish:
		return false
	default:
		return true
	}
}

func main() {
	build := flag.Int("build", 1, "build number")
	mode := flag.String("mode", modeFull, "full | rerun1 | rerun2")
	rerunOf := flag.String("rerun-of", "", "UUID of the launch to rerun")
	flag.Parse()

	ctx := context.Background()
	host, project, apiKey := os.Getenv(
		"GORP_HOST",
	), os.Getenv(
		"GORP_PROJECT",
	), os.Getenv(
		"GORP_API_KEY",
	)
	if host == "" || project == "" || apiKey == "" {
		log.Fatal("set GORP_HOST, GORP_PROJECT and GORP_API_KEY")
	}
	client := gorp.NewReportingClient(host, project, gorp.WithApiKeyAuth(ctx, apiKey))

	trigger := "nightly"
	if *mode != modeFull {
		trigger = "ci-rerun"
	}
	attrs := []openapi.ItemAttributesRQ{
		attr(
			"env",
			"staging",
		),
		attr("build", fmt.Sprint(*build)),
		attr("version", fmt.Sprintf("2.4.%d", *build)),
		attr("branch", "main"),
		attr("suite", "regression"),
		attr("platform", "linux"),
		attr("team", "payments"),
		attr("trigger", trigger),
	}
	attrs = append(attrs,
		systemAttr("agent", agentName+"|"+agentVer),
		systemAttr("os", runtime.GOOS+"|"+runtime.GOARCH),
		systemAttr("go", runtime.Version()),
		// skipped items without an issue are To Investigate unless the launch says otherwise
		systemAttr("skippedIssue", "false"),
	)

	desc := fmt.Sprintf("**Payments API regression**, build `%d` on `main` (staging).\n\n"+
		"CI job: [payments-nightly #%d]("+ciURLFormat+")\n\n"+
		"Reported with the goRP library: attribute contract, nested containers, parameters, "+
		"retries (`retryOf`), skipped items as No Defect, defect types and reruns (`rerunOf`).",
		*build, *build, *build)
	if *mode != modeFull {
		desc += fmt.Sprintf("\n\n_Rerun (%s) of the failed tests after a fix._", *mode)
	}

	startTime := time.Now().Add(-10 * time.Minute)
	rq := &openapi.StartLaunchRQ{
		Uuid:        newUUID(),
		Name:        launchName,
		StartTime:   startTime,
		Description: openapi.PtrString(desc),
		Attributes:  attrs,
		Mode:        openapi.PtrString(string(gorp.LaunchModes.Default)),
	}
	if *rerunOf != "" {
		rq.Rerun = openapi.PtrBool(true)
		rq.RerunOf = openapi.PtrString(*rerunOf)
	}
	rs, err := client.StartLaunch(ctx, rq)
	if err != nil {
		log.Fatalf("start launch: %v", err)
	}
	// on a rerun the server answers with the UUID of the original launch
	r := &reporter{
		ctx:    ctx,
		c:      client,
		launch: rs.GetId(),
		clock:  startTime,
		mode:   *mode,
		build:  *build,
	}

	failed := r.paymentsSuite()
	if r.ledgerSuite() {
		failed = true
	}

	status := gorp.Statuses.Passed
	if failed {
		status = gorp.Statuses.Failed
	}
	finishRQ := &openapi.FinishExecutionRQ{EndTime: r.tick(time.Second)}
	if *rerunOf == "" {
		// a rerun reports only some tests, so the server must compute the launch status from all of them
		finishRQ.Status = status.Ptr()
	}
	frs, err := client.FinishLaunch(ctx, r.launch, finishRQ)
	if err != nil {
		log.Fatalf("finish launch: %v", err)
	}
	fmt.Printf("ReportPortal Launch UUID: %s\nReportPortal Launch: %s\n", r.launch, frs.GetLink())
}

// paymentsSuite reports the payments-api suite; it returns whether anything failed.
func (r *reporter) paymentsSuite() bool {
	failed := false
	suite := r.start("", item{
		name: "payments-api", typ: "SUITE", codeRef: module + "/api",
		description: "HTTP API of the payments service: refunds, webhooks and currency conversion.",
		attrs:       []openapi.ItemAttributesRQ{attr("component", "api")},
	})

	// suite fixture
	if r.mode == modeFull {
		fx := r.start(
			suite,
			item{
				name:    "start sandbox gateway",
				typ:     "BEFORE_SUITE",
				codeRef: module + "/api.TestMain",
			},
		)
		r.logs(fx,
			logLine{level: "INFO", msg: "starting gateway stub on :8443"},
			logLine{level: "DEBUG", msg: "TLS: self-signed certificate, 2048-bit RSA"},
			logLine{level: "INFO", msg: "gateway ready in 412ms"})
		r.finish(fx, gorp.Statuses.Passed, nil)
	}

	// --- Refunds: a parametrized test and a test with nested steps
	refunds := r.start(suite, item{
		name:       "Refunds",
		typ:        "TEST",
		codeRef:    module + "/api/refunds",
		testCaseID: module + "/api > Refunds",
		attrs:      []openapi.ItemAttributesRQ{attr("feature", "refunds"), attr("owner", "anna.k")},
	})
	for _, cur := range []string{"USD", "EUR", "GBP"} {
		name := "full refund[" + cur + "]"
		if !r.runs(name) {
			continue
		}
		id := r.start(refunds, item{
			name:       "full refund",
			typ:        "STEP",
			codeRef:    module + "/api/refunds_test.go:TestFullRefund",
			testCaseID: module + "/api > Refunds > full refund[" + cur + "]",
			params:     params("currency", cur, "amount", "100.00"),
			attrs: []openapi.ItemAttributesRQ{
				attr("priority", "p1"),
				attr("tms_id", "PAY-101"),
			},
		})
		r.logs(
			id,
			logLine{
				level: "INFO",
				msg:   fmt.Sprintf("POST /v1/payments {amount: 100.00, currency: %s}", cur),
			},
			logLine{level: "DEBUG", msg: "payment id pay_7f3a created"},
			logLine{level: "INFO", msg: "POST /v1/payments/pay_7f3a/refund {amount: 100.00}"},
		)
		if cur == "GBP" && r.mode == modeFull && r.build >= 102 { // regression since build 102
			r.logs(id,
				logLine{
					level: "ERROR", msg: "expected refund status `succeeded`, got `pending`\n\n" +
						"    refunds_test.go:88: assertion failed\n    refunds_test.go:41: TestFullRefund/GBP",
					file: jsonFile("refund-response.json", map[string]any{
						"id":       "re_19c2",
						"status":   "pending",
						"currency": "GBP",
						"reason":   "fx_settlement_delay",
					}),
				})
			r.finish(
				id,
				gorp.Statuses.Failed,
				issue("pb001", "GBP refunds stay pending: FX settlement delay. PAY-2291"),
			)
			failed = true
			continue
		}
		r.finish(id, gorp.Statuses.Passed, nil)
	}

	if r.runs("partial refund") {
		// before/after method fixtures around a test
		if r.mode == modeFull {
			bm := r.start(
				refunds,
				item{
					name:    "seed merchant balance",
					typ:     "BEFORE_METHOD",
					codeRef: module + "/api/refunds_test.go:seedBalance",
				},
			)
			r.logs(bm, logLine{level: "DEBUG", msg: "merchant m_42 balance set to 5,000.00 USD"})
			r.finish(bm, gorp.Statuses.Passed, nil)
		}
		id := r.start(refunds, item{
			name:        "partial refund",
			typ:         "STEP",
			codeRef:     module + "/api/refunds_test.go:TestPartialRefund",
			testCaseID:  module + "/api > Refunds > partial refund",
			description: "Refunds half of a captured payment and checks the ledger entries.",
			attrs: []openapi.ItemAttributesRQ{
				attr("priority", "p0"),
				attr("tms_id", "PAY-102"),
			},
		})
		steps := []string{"create payment", "refund 50%", "verify ledger"}
		fail := r.mode == modeFull
		for i, s := range steps {
			st := r.start(id, item{name: s, typ: "STEP", noStats: true})
			r.logs(st, logLine{level: "INFO", msg: "step: " + s})
			if fail && i == 2 {
				r.logs(st,
					logLine{
						level: "ERROR",
						msg:   "ledger mismatch: expected 2 entries, found 3 (duplicate reversal)",
						file:  screenshot("ledger-page.png", color.RGBA{220, 53, 69, 255}),
					},
					logLine{level: "WARN", msg: "ledger reconciliation job is lagging by 14s"})
				r.finish(st, gorp.Statuses.Failed, nil)
				continue
			}
			r.finish(st, gorp.Statuses.Passed, nil)
		}
		if fail {
			r.finish(id, gorp.Statuses.Failed, nil) // left To Investigate on purpose
			failed = true
		} else {
			r.logs(
				id,
				logLine{level: "INFO", msg: "ledger fix PAY-2295 deployed: 2 entries as expected"},
			)
			r.finish(id, gorp.Statuses.Passed, nil)
		}
		if r.mode == modeFull {
			am := r.start(
				refunds,
				item{
					name:    "reset merchant balance",
					typ:     "AFTER_METHOD",
					codeRef: module + "/api/refunds_test.go:resetBalance",
				},
			)
			r.finish(am, gorp.Statuses.Passed, nil)
		}
	}
	r.finish(refunds, statusOf(failed && r.mode == modeFull), nil)

	// --- Webhooks: retries with a retryOf chain, skipped items
	if r.mode == modeFull {
		hooks := r.start(suite, item{
			name:       "Webhooks",
			typ:        "TEST",
			codeRef:    module + "/api/webhooks",
			testCaseID: module + "/api > Webhooks",
			attrs: []openapi.ItemAttributesRQ{
				attr("feature", "webhooks"),
				attr("owner", "li.w"),
			},
		})
		flaky := item{
			name:       "delivery retries on 503",
			typ:        "STEP",
			codeRef:    module + "/api/webhooks_test.go:TestDeliveryRetry",
			testCaseID: module + "/api > Webhooks > delivery retries on 503",
			attrs: []openapi.ItemAttributesRQ{
				attr("priority", "p1"),
				attr("tms_id", "PAY-210"),
				attr("flaky", "true"),
			},
		}
		prev := ""
		for attempt := 1; attempt <= 3; attempt++ {
			it := flaky
			it.retryOf = prev // each attempt is a retry of the previous one
			id := r.start(hooks, it)
			r.logs(id, logLine{level: "INFO", msg: fmt.Sprintf("attempt %d of 3", attempt)})
			if attempt < 3 {
				r.logs(
					id,
					logLine{
						level: "ERROR",
						msg:   "receiver answered 503 three times; delivery not confirmed within 5s",
					},
				)
				// a failed attempt that is retried is not a defect: No Defect keeps it out of To Investigate
				r.finish(id, gorp.Statuses.Failed, issue("NOT_ISSUE", ""))
			} else {
				r.logs(id, logLine{level: "INFO", msg: "delivered after 2 retries"})
				r.finish(id, gorp.Statuses.Passed, nil)
			}
			prev = id
		}

		sk := r.start(hooks, item{
			name:       "signature rotation",
			typ:        "STEP",
			codeRef:    module + "/api/webhooks_test.go:TestSignatureRotation",
			testCaseID: module + "/api > Webhooks > signature rotation",
			attrs: []openapi.ItemAttributesRQ{
				attr("priority", "p2"),
				attr("tms_id", "PAY-214"),
			},
		})
		r.logs(
			sk,
			logLine{level: "WARN", msg: "skipped: key rotation is disabled on staging (PAY-1980)"},
		)
		r.finish(sk, gorp.Statuses.Skipped, issue("NOT_ISSUE", ""))

		tr := r.start(hooks, item{
			name: "payload trace", typ: "STEP",
			codeRef:    module + "/api/webhooks_test.go:TestPayloadTrace",
			testCaseID: module + "/api > Webhooks > payload trace",
			attrs:      []openapi.ItemAttributesRQ{attr("priority", "p3")},
		})
		r.logs(
			tr,
			logLine{level: "TRACE", msg: "raw frame: 16 03 01 02 00 01 00 01 fc 03 03"},
			logLine{level: "DEBUG", msg: "headers: X-Signature=v1,t=1764201600"},
			logLine{
				level: "INFO",
				msg:   "!!!MARKDOWN_MODE!!!\n| event | status |\n|---|---|\n| payment.succeeded | 200 |\n| refund.created | 200 |",
			},
			logLine{level: "WARN", msg: "receiver took 1.8s (budget 2s)"},
			logLine{
				level: "INFO",
				msg:   "full payload attached",
				file: jsonFile("webhook-payload.json", map[string]any{
					"type": "refund.created",
					"data": map[string]any{"id": "re_19c2", "amount": 5000},
				}),
			},
			logLine{
				level: "INFO",
				msg:   "receiver log attached",
				file:  textFile("receiver.log", "200 POST /hook 12ms\n200 POST /hook 9ms\n"),
			},
		)
		r.finish(tr, gorp.Statuses.Passed, nil)
		r.finish(hooks, gorp.Statuses.Passed, nil)
	}

	// --- Currency conversion: a table with a system issue
	conv := r.start(suite, item{
		name:       "Currency conversion",
		typ:        "TEST",
		codeRef:    module + "/api/fx",
		testCaseID: module + "/api > Currency conversion",
		attrs:      []openapi.ItemAttributesRQ{attr("feature", "fx"), attr("owner", "omar.s")},
	})
	convFailed := false
	for _, pair := range [][2]string{{"USD", "EUR"}, {"EUR", "PLN"}, {"USD", "JPY"}} {
		name := "convert[" + pair[1] + "]"
		if !r.runs(name) {
			continue
		}
		id := r.start(conv, item{
			name:       "convert",
			typ:        "STEP",
			codeRef:    module + "/api/fx_test.go:TestConvert",
			testCaseID: module + "/api > Currency conversion > convert[" + pair[0] + "-" + pair[1] + "]",
			params:     params("from", pair[0], "to", pair[1]),
			attrs: []openapi.ItemAttributesRQ{
				attr("priority", "p2"),
				attr("tms_id", "PAY-301"),
			},
		})
		r.logs(
			id,
			logLine{
				level: "INFO",
				msg:   fmt.Sprintf("GET /v1/fx/rate?from=%s&to=%s", pair[0], pair[1]),
			},
		)
		if pair[1] == "JPY" && r.mode != modeRerun2 {
			r.logs(
				id,
				logLine{
					level: "FATAL",
					msg:   "context deadline exceeded: rates provider did not answer in 10s",
				},
			)
			r.finish(
				id,
				gorp.Statuses.Failed,
				issue("si001", "Rates provider sandbox outage (INFRA-77)"),
			)
			convFailed = true
			continue
		}
		r.finish(id, gorp.Statuses.Passed, nil)
	}
	r.finish(conv, statusOf(convFailed), nil)
	if convFailed {
		failed = true
	}

	r.finish(suite, statusOf(failed), nil)
	return failed
}

// ledgerSuite reports a second suite with deeper nesting; only in a full run.
func (r *reporter) ledgerSuite() bool {
	if r.mode != modeFull {
		return false
	}
	suite := r.start("", item{
		name: "ledger-service", typ: "SUITE", codeRef: module + "/ledger",
		attrs: []openapi.ItemAttributesRQ{attr("component", "ledger")},
	})
	acc := r.start(
		suite,
		item{
			name:       "Accounts",
			typ:        "TEST",
			codeRef:    module + "/ledger/accounts",
			testCaseID: module + "/ledger > Accounts",
		},
	)
	for _, group := range []string{"opening", "closing"} {
		g := r.start(
			acc,
			item{name: group, typ: "TEST", testCaseID: module + "/ledger > Accounts > " + group},
		)
		groupFailed := false
		for _, c := range []string{"personal", "business"} {
			id := r.start(g, item{
				name: c + " account",
				typ:  "STEP",
				codeRef: module + "/ledger/accounts_test.go:Test" + strings.ToUpper(
					group[:1],
				) + group[1:],
				testCaseID: module + "/ledger > Accounts > " + group + " > " + c,
				params:     params("kind", c),
				attrs: []openapi.ItemAttributesRQ{
					attr("priority", "p2"),
					attr("feature", "accounts"),
					attr("owner", "dmitry.g"),
				},
			})
			r.logs(id, logLine{level: "INFO", msg: group + " " + c + " account: ok"})
			if group == "closing" && c == "business" &&
				r.build != 102 { // fixed in 102, back in 103
				r.logs(id, logLine{
					level: "ERROR", msg: "expected 409 for non-zero balance, got 200",
					file: screenshot("close-account.png", color.RGBA{255, 193, 7, 255}),
				})
				r.finish(
					id,
					gorp.Statuses.Failed,
					issue("ab001", "Test data: balance fixture not reset between runs"),
				)
				groupFailed = true
				continue
			}
			r.finish(id, gorp.Statuses.Passed, nil)
		}
		r.finish(g, statusOf(groupFailed), nil)
	}
	r.finish(acc, gorp.Statuses.Failed, nil)

	audit := r.start(
		suite,
		item{name: "Audit export", typ: "TEST", testCaseID: module + "/ledger > Audit export"},
	)
	id := r.start(audit, item{
		name:       "export to S3",
		typ:        "STEP",
		codeRef:    module + "/ledger/audit_test.go:TestExport",
		testCaseID: module + "/ledger > Audit export > export to S3",
		attrs:      []openapi.ItemAttributesRQ{attr("priority", "p3")},
	})
	r.logs(
		id,
		logLine{level: "WARN", msg: "skipped: S3 credentials are not configured for staging"},
	)
	r.finish(id, gorp.Statuses.Skipped, nil) // stays To Investigate: nobody marked it, on purpose
	r.finish(audit, gorp.Statuses.Skipped, nil)
	r.finish(suite, gorp.Statuses.Failed, nil)
	return true
}

// systemAttr is a system attribute: shown in the launch info, not in the attribute list.
func systemAttr(k, v string) openapi.ItemAttributesRQ {
	a := attr(k, v)
	a.System = openapi.PtrBool(true)
	return a
}

// newUUID returns a random (version 4) UUID. Items get client-side UUIDs, so a retried start
// request cannot create a duplicate item.
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func statusOf(failed bool) gorp.Status {
	if failed {
		return gorp.Statuses.Failed
	}
	return gorp.Statuses.Passed
}
