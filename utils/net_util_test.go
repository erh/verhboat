package utils

import (
	"testing"

	"go.viam.com/rdk/logging"
	"go.viam.com/test"
)

func TestFindInterface(t *testing.T) {
	logger := logging.NewTestLogger(t)
	n, err := findInterface("garbage")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, n.InterfaceName, test.ShouldEqual, "")

	all, err := findAllGoodNetworks()
	test.That(t, err, test.ShouldBeNil)

	for _, n := range all {
		n2, err := findInterface(n.Addr.String()[0:7])
		test.That(t, err, test.ShouldBeNil)
		logger.Infof("n: %v", n)
		logger.Infof("n2: %v", n2)
		test.That(t, n, test.ShouldResemble, n2)
	}

}
