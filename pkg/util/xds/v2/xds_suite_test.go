package v2_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestXds(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Xds V2 Suite")
}
