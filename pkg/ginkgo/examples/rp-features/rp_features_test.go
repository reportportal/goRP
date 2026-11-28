//go:build sanity

package rpfeatures_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rpagent "github.com/reportportal/goRP/pkg/ginkgo/rpagent"
)

// ─── 1. P0 — By() steps with INFO logs (log-to-step routing) ─────────────────
//
// Demonstrates: By() child steps, rpagent.Log(), TestReference attribute.
// Each rpagent.Log() call is routed to the By() step it was emitted under,
// using TimelineLocation.Order for ordering.

var _ = Describe("Payment service", func() {
	It("processes a valid payment",
		Label(rpagent.Priority(0), rpagent.TestReference("PAY-TC-001")),
		func() {
			By("validating card details")
			rpagent.Log("INFO", "card=****1234  amount=99.99  currency=INR")
			Expect("4111111111111234").To(HaveLen(16))

			By("authorising charge")
			rpagent.Log("INFO", "auth gateway responded: APPROVED")
			Expect(200).To(Equal(200))

			By("confirming order")
			rpagent.Log("INFO", "order-id=ORD-0042 confirmed")
			Expect(true).To(BeTrue())
		})
})

// ─── 2. P1 — JSON attachment + DEBUG logs ────────────────────────────────────
//
// Demonstrates: rpagent.AddAttachment(), DEBUG log level, JSON file upload.

var _ = Describe("Inventory API", func() {
	It("returns stock levels with correct schema",
		Label(rpagent.Priority(1), rpagent.TestReference("INV-TC-002")),
		func() {
			rpagent.Log("DEBUG", "GET /api/v1/inventory?sku=WIDGET-99")

			fixture := filepath.Join("testdata", "inventory-response.json")
			payload, err := os.ReadFile(fixture)
			Expect(err).NotTo(HaveOccurred())

			rpagent.Log("INFO", "response 200 OK — attaching payload")
			rpagent.AddAttachment(fixture, "inventory API response")

			Expect(string(payload)).To(ContainSubstring(`"available":124`))
		})
})

// ─── 3. P2 — intentional failure + ERROR log + stack-trace attachment ────────
//
// Demonstrates: ERROR log level, stack-trace file attachment,
// how RP shows a FAILED result with evidence attached.

var _ = Describe("Checkout validation", func() {
	It("fails when cart is empty — captures stack trace",
		Label(rpagent.Priority(2), rpagent.TestReference("CHK-TC-003")),
		func() {
			rpagent.Log("WARN", "cart contains 0 items — proceeding to trigger validation error")

			trace := debug.Stack()
			f, err := os.CreateTemp("", "stack-trace-*.txt")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(f.Name())
			fmt.Fprintf(f, "=== stack trace at failure ===\n%s\n=== end ===\n", string(trace))
			f.Close()

			rpagent.Log("ERROR", "cart is empty — checkout must be rejected")
			rpagent.AddAttachment(f.Name(), "goroutine stack trace")

			Expect("cart").To(Equal("not empty")) // intentional failure
		})
})

// ─── 4. P1 — FATAL log level ─────────────────────────────────────────────────
//
// Demonstrates: FATAL severity in RP without crashing the test process.
// rpagent.Log("FATAL", ...) sends a log entry with severity FATAL via the
// normal POST /log path — it does NOT call os.Exit. The spec continues and
// passes on its own assertions.

var _ = Describe("FATAL log level", func() {
	It("sends a FATAL log entry to RP without crashing the test process",
		Label(rpagent.Priority(1), rpagent.TestReference("LOG-TC-FATAL")),
		func() {
			rpagent.Log("INFO", "simulating a catastrophic dependency failure")
			rpagent.Log("FATAL", "database connection pool exhausted — all retries failed")
			rpagent.Log("INFO", "test process still alive — FATAL is a severity label, not os.Exit")
			Expect(true).To(BeTrue())
		})
})

// ─── 5. P4 — CSV attachment + Eventually polling ──────────────────────────────
//
// Demonstrates: CSV file attachment, async readiness polling with Eventually.

var _ = Describe("Report generation", func() {
	It("generates and attaches a CSV report within 500 ms",
		Label(rpagent.Priority(4), rpagent.TestReference("RPT-TC-005")),
		func() {
			rpagent.Log("INFO", "starting report generation")

			start := time.Now()
			var ready bool
			Eventually(func() bool {
				ready = true
				return ready
			}, 500*time.Millisecond, 50*time.Millisecond).Should(BeTrue())
			elapsed := time.Since(start)
			rpagent.Log("DEBUG", fmt.Sprintf("report ready in %dms", elapsed.Milliseconds()))

			csv := "order_id,sku,qty,total\nORD-001,WIDGET-99,3,29.97\nORD-002,GADGET-7,1,49.99\n"
			f, err := os.CreateTemp("", "sales-report-*.csv")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(f.Name())
			fmt.Fprint(f, csv)
			f.Close()

			rpagent.Log("INFO", "attaching generated CSV report")
			rpagent.AddAttachment(f.Name(), "sales report CSV")
			Expect(ready).To(BeTrue())
		})
})
