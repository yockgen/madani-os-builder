package rpmutils_test

import (
	"testing"

	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/resolvertest"
	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/rpmutils"
)

func TestRPMResolver(t *testing.T) {
	resolvertest.RunResolverTestsFunc(
		t,
		"rpmutils",
		rpmutils.ResolvePackageInfos, // directly passing your function
	)
}
