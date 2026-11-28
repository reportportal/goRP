//go:build special

// Package tagged is a Ginkgo suite that only exists with -tags special. Without the tag,
// Ginkgo still counts the package as a suite (it does not evaluate build constraints), while
// ginkgo-rp does not register it in ReportPortal.
package tagged

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	_ "github.com/reportportal/goRP/pkg/ginkgo/rpagent"
)

func TestTagged(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tagged Suite")
}

var _ = Describe("tagged", func() {
	It("runs only with -tags special", func() { Expect(1).To(Equal(1)) })
})
