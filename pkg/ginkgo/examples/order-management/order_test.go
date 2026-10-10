//go:build sanity

package ordermanagement_test

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rpagent "github.com/reportportal/goRP/pkg/ginkgo/rpagent"
)

// ─── P0 — place order end-to-end with By() steps ─────────────────────────────
//
// Demonstrates: multiple By() steps, log routing per step, full order lifecycle.

var _ = Describe("Order placement", func() {
	It("places an order end-to-end",
		Label(rpagent.Priority(0), rpagent.TestReference("ORD-TC-001")),
		func() {
			By("validating cart contents")
			rpagent.Log("INFO", "cart=[WIDGET-99 x2, GADGET-7 x1]  subtotal=109.93")
			Expect([]string{"WIDGET-99", "GADGET-7"}).To(HaveLen(2))

			By("applying discount code")
			rpagent.Log("INFO", "code=SAVE10 applied — discount=10.99")
			Expect(109.93 - 10.99).To(BeNumerically("~", 98.94, 0.01))

			By("processing payment")
			rpagent.Log("INFO", "card=****5678  amount=98.94  status=APPROVED")
			Expect(200).To(Equal(200))

			By("confirming order and sending email")
			rpagent.Log("INFO", "order-id=ORD-1001 confirmed — confirmation email queued")
			Expect("ORD-1001").To(HavePrefix("ORD-"))

			By("verifying inventory deducted")
			rpagent.Log("INFO", "WIDGET-99: stock 142→140  GADGET-7: stock 38→37")
			Expect(142 - 2).To(Equal(140))
		})
})

// ─── P1 — cancel order ────────────────────────────────────────────────────────

var _ = Describe("Order cancellation", func() {
	It("cancels an order within the cancellation window",
		Label(rpagent.Priority(1), rpagent.TestReference("ORD-TC-002")),
		func() {
			By("fetching order status")
			rpagent.Log("INFO", "order-id=ORD-1002 status=PENDING_SHIPMENT")
			Expect("PENDING_SHIPMENT").NotTo(Equal("SHIPPED"))

			By("submitting cancellation request")
			rpagent.Log("INFO", "cancellation accepted — refund=98.94 INR initiated")
			Expect(true).To(BeTrue())

			By("verifying order marked cancelled")
			rpagent.Log("INFO", "order-id=ORD-1002 status=CANCELLED")
			Expect("CANCELLED").To(Equal("CANCELLED"))
		})
})

// ─── P1 — track shipment ──────────────────────────────────────────────────────

var _ = Describe("Shipment tracking", func() {
	It("returns live tracking events for a shipped order",
		Label(rpagent.Priority(1), rpagent.TestReference("ORD-TC-003")),
		func() {
			By("requesting tracking info")
			rpagent.Log("DEBUG", "GET /api/v1/orders/ORD-1003/tracking")

			type event struct{ status, location string }
			events := []event{
				{"PICKED_UP", "Mumbai Hub"},
				{"IN_TRANSIT", "Delhi Sorting"},
				{"OUT_FOR_DELIVERY", "Bengaluru"},
			}
			rpagent.Log("INFO", fmt.Sprintf("tracking events received: %d", len(events)))

			By("attaching tracking payload as JSON")
			payload := `[{"status":"PICKED_UP"},{"status":"IN_TRANSIT"},{"status":"OUT_FOR_DELIVERY"}]`
			f, err := os.CreateTemp("", "tracking-*.json")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(f.Name())
			fmt.Fprint(f, payload)
			f.Close()
			rpagent.AddAttachment(f.Name(), "tracking events")

			By("asserting latest event")
			Expect(events[len(events)-1].status).To(Equal("OUT_FOR_DELIVERY"))
		})
})

// ─── P2 — duplicate order rejected (intentional failure) ─────────────────────

var _ = Describe("Duplicate order guard", func() {
	It("rejects a duplicate order submission — intentional failure",
		Label(rpagent.Priority(2), rpagent.TestReference("ORD-TC-004")),
		func() {
			By("submitting first order")
			rpagent.Log("INFO", "order-id=ORD-1004 created")

			By("resubmitting identical order")
			rpagent.Log("WARN", "duplicate detected — expecting 409 Conflict")
			rpagent.Log("ERROR", "service returned 200 instead of 409")

			Expect(200).To(Equal(409)) // intentional failure — duplicate not rejected
		})
})

// ─── P3 — refund SLA ─────────────────────────────────────────────────────────

var _ = Describe("Refund SLA", func() {
	It("processes a refund within the 500ms SLA",
		Label(rpagent.Priority(3), rpagent.TestReference("ORD-TC-005")),
		func() {
			By("initiating refund")
			rpagent.Log("INFO", "order-id=ORD-1005  refund=49.99 INR")

			start := time.Now()
			var done bool
			Eventually(func() bool {
				done = true
				return done
			}, 500*time.Millisecond, 50*time.Millisecond).Should(BeTrue())
			elapsed := time.Since(start)

			By("asserting SLA met")
			rpagent.Log("DEBUG", fmt.Sprintf("refund processed in %dms", elapsed.Milliseconds()))
			Expect(elapsed).To(BeNumerically("<", 500*time.Millisecond))
		})
})
