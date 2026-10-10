//go:build sanity

package ordermanagement_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	_ "github.com/reportportal/goRP/pkg/ginkgo/rpagent"
)

func TestOrderManagement(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Order Management Suite")
}
