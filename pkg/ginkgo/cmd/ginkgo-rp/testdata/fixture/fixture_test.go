// Package fixture is a tiny Ginkgo suite the wrapper tests run for real, through ginkgo.
// Which specs run is chosen by environment variables, so one suite serves every scenario.
package fixture

import (
	"fmt"
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
	if os.Getenv("FIXTURE_FLAKY_STDOUT") != "" {
		// Prints to stdout on every attempt; Ginkgo captures stdout only when it runs in
		// parallel, and cannot say which attempt wrote what.
		var attempts int
		It("flaky with stdout", FlakeAttempts(3), func() {
			attempts++
			GinkgoWriter.Printf("writer says attempt %d\n", attempts)
			fmt.Printf("stdout says attempt %d\n", attempts)
			Expect(attempts).To(BeNumerically(">=", 3), "failed on attempt %d", attempts)
		})
	}
	if os.Getenv("FIXTURE_NESTED") != "" {
		// Three nested containers: enough finish requests in a row to hit the failure streak.
		Describe("level1", func() {
			Describe("level2", func() {
				Describe("level3", func() {
					It("deep", func() {})
				})
			})
		})
	}
	if os.Getenv("FIXTURE_SKIPS") != "" {
		It("user skip", func() { Skip("not on this OS") })
		PIt("pending spec", func() {})
		It("filtered out", func() {}) // the test filters it out with --skip
		Describe("ordered group", Ordered, func() {
			It("breaks", func() { Fail("boom") })
			It("cascade skipped", func() {})
		})
	}
	if os.Getenv("FIXTURE_PRIORITY") != "" {
		It("explicit default", Label(rpagent.Priority(1)), func() {})
		It("explicit critical", Label(rpagent.Priority(0)), func() {})
		Describe("priority container", Label(rpagent.Priority(3)), func() {
			It("inherits the container", func() {})
			It("overrides the container", Label(rpagent.Priority(0)), func() {})
		})
	}
	if os.Getenv("FIXTURE_ATTACH_ERRORS") != "" {
		// Each failure mode of AddAttachment leaves an error entry for the agent.
		It("attach errors", func() {
			dir := GinkgoT().TempDir()
			rpagent.AddAttachment(filepath.Join(dir, "missing.txt"), "no such file")

			big := filepath.Join(dir, "big.txt")
			Expect(os.WriteFile(big, []byte("0123456789"), 0o600)).To(Succeed())
			rpagent.SetAttachmentConfig(rpagent.AttachmentConfig{MaxFileSize: 3})
			rpagent.AddAttachment(big, "too big")

			small := filepath.Join(dir, "small.txt")
			Expect(os.WriteFile(small, []byte("ab"), 0o600)).To(Succeed())
			rpagent.SetAttachmentConfig(rpagent.AttachmentConfig{TempDir: filepath.Join(dir, "no", "such", "dir")})
			rpagent.AddAttachment(small, "cannot copy")
		})
	}
	if os.Getenv("FIXTURE_FLAKY_AFTEREACH") != "" {
		// Attempt 1 fails in an AfterEach, not in the It: Ginkgo only keeps that failure in
		// SpecReport.AdditionalFailures.
		Describe("flaky cleanup", func() {
			var attempts int
			AfterEach(func() {
				attempts++
				if attempts == 1 {
					Fail("cleanup failed on attempt 1")
				}
			})
			It("flaky after each", FlakeAttempts(2), func() {
				GinkgoWriter.Println("body ran")
			})
		})
	}
	if os.Getenv("FIXTURE_REPEAT") != "" {
		It("repeats", MustPassRepeatedly(3), func() {
			By("repetition body")
			GinkgoWriter.Println("repeat output")
		})
	}
	if os.Getenv("FIXTURE_REPEAT_FAIL") != "" {
		var runs int
		It("repeat fails", MustPassRepeatedly(3), func() {
			runs++
			GinkgoWriter.Println("run", runs)
			Expect(runs).To(BeNumerically("<", 2), "failed on repetition %d", runs)
		})
	}
	if os.Getenv("FIXTURE_CLEANUP_FAIL") != "" {
		It("fails then cleanup fails", func() {
			DeferCleanup(func() { Fail("cleanup failure after the spec failed") })
			Fail("primary failure")
		})
	}
	if os.Getenv("FIXTURE_SLEEP") != "" {
		It("sleeps", func() { time.Sleep(60 * time.Second) })
	}
})
