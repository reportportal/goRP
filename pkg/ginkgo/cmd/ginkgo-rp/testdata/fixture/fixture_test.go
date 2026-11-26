// Package fixture is a tiny Ginkgo suite the wrapper tests run for real, through ginkgo.
// Which specs run is chosen by environment variables, so one suite serves every scenario.
package fixture

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/reportportal/goRP/pkg/ginkgo/rpagent"
)

func TestFixture(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fixture Suite")
}

var _ = Describe("fixture", func() {
	It("passes", func() { Expect(1).To(Equal(1)) })

	if os.Getenv("FIXTURE_FAIL") != "" {
		It("fails", func() { Expect(1).To(Equal(2), "intended failure") })
	}
	if os.Getenv("FIXTURE_ATTACH") != "" {
		It("attaches", func() {
			By("writing a file")
			path := filepath.Join(GinkgoT().TempDir(), "evidence.txt")
			Expect(os.WriteFile(path, []byte("evidence"), 0o600)).To(Succeed())
			rpagent.Log("WARN", "custom log line")
			rpagent.AddAttachment(path, "the evidence")
			GinkgoWriter.Println("writer line")
		})
	}
	if os.Getenv("FIXTURE_SLEEP") != "" {
		It("sleeps", func() { time.Sleep(60 * time.Second) })
	}
})
