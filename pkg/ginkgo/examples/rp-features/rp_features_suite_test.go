//go:build sanity

package rpfeatures_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	_ "github.com/reportportal/goRP/pkg/ginkgo/rpagent"
)

func TestRPFeatures(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RP Features Suite")
}
