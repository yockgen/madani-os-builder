package debutils_test

import (
	"testing"

	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/debutils"
	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/resolvertest"
)

func TestDEBResolver(t *testing.T) {
	resolvertest.RunResolverTestsFunc(
		t,
		"debutils",
		debutils.ResolvePackageInfos, // directly passing your function
	)
}
