//go:build sanity

package calculator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rpagent "github.com/reportportal/goRP/pkg/ginkgo/rpagent"
)

var _ = Describe("Calculator", func() {

	// ─── 1. P0 — addition with By() steps and INFO logs ───────────────────────

	Describe("Addition", func() {
		It("adds two numbers step by step",
			Label(rpagent.Priority(0), rpagent.TestReference("CALC-TC-001")),
			func() {
				By("adding two positive numbers")
				rpagent.Log("INFO", "input: 2 + 3")
				Expect(2 + 3).To(Equal(5))
				rpagent.Log("INFO", "result: 5 ✓")

				By("adding a positive and a negative number")
				rpagent.Log("INFO", "input: 10 + (-3)")
				Expect(10 + -3).To(Equal(7))
				rpagent.Log("INFO", "result: 7 ✓")
			})
	})

	// ─── 2. P1 — subtraction with WARN log on boundary value ─────────────────

	Describe("Subtraction", func() {
		It("subtracts and warns on zero result",
			Label(rpagent.Priority(1), rpagent.TestReference("CALC-TC-002")),
			func() {
				rpagent.Log("DEBUG", "input: 10 - 4")
				Expect(10 - 4).To(Equal(6))
				rpagent.Log("INFO", "result: 6 ✓")

				rpagent.Log("WARN", "boundary check: subtracting equal values gives 0")
				Expect(5 - 5).To(Equal(0))
			})
	})

	// ─── 3. P2 — multiplication + intentional failure to show ERROR in RP ─────

	Describe("Multiplication", func() {
		It("multiplies correctly then fails intentionally",
			Label(rpagent.Priority(2), rpagent.TestReference("CALC-TC-003")),
			func() {
				rpagent.Log("INFO", "input: 3 × 4")
				Expect(3 * 4).To(Equal(12))
				rpagent.Log("INFO", "result: 12 ✓")
				Expect(1 + 1).To(Equal(3)) // intentional failure
			})
	})

	// ─── 4. P1 — table-driven division (DescribeTable) ───────────────────────

	DescribeTable("Division",
		func(a, b, expected int) {
			rpagent.Log("DEBUG", "input: %d ÷ %d")
			Expect(a / b).To(Equal(expected))
			rpagent.Log("INFO", "result matches expected value ✓")
		},
		Entry("input='10' divisor='2' expected='5'", Label(rpagent.Priority(1)), 10, 2, 5),
		Entry("input='9' divisor='3' expected='3'", Label(rpagent.Priority(1)), 9, 3, 3),
	)

	// ─── 5. P3 — skip example to show SKIPPED status in RP ───────────────────

	Describe("Square root", func() {
		It("is not yet implemented",
			Label(rpagent.Priority(3)),
			func() {
				Skip("square root not implemented yet")
			})
	})
})
