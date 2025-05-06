package rpmutils

import (
	"fmt"
	"path/filepath"

	"github.com/cavaliergopher/rpm"
	"go.uber.org/zap"
)

// Index maps provided capabilities to RPM paths and RPM paths to their requirements.
type Index struct {
	Provides map[string][]string // capability name → []rpm paths
	Requires map[string][]string // rpm path → []required capability names
}

// BuildIndex scans all RPM files under dir and builds the Index.
func BuildIndex(dir string) (*Index, error) {
	//logger := zap.L().Sugar()
	idx := &Index{
		Provides: make(map[string][]string),
		Requires: make(map[string][]string),
	}

	pattern := filepath.Join(dir, "*.rpm")
	rpmFiles, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	for _, rpmPath := range rpmFiles {
		// Open the RPM file
		pkgFile, err := rpm.Open(rpmPath)
		if err != nil {
			return nil, fmt.Errorf("opening RPM %s: %w", rpmPath, err)
		}

		// Extract capabilities it Provides()
		provDeps := pkgFile.Provides() // []rpm.Dependency
		for _, dep := range provDeps {
			name := dep.Name() // call method to get string
			idx.Provides[name] = append(idx.Provides[name], rpmPath)
			// logger.Debugf("RPM %s provides %s", rpmPath, name)
		}

		// Extract its Requires()
		reqDeps := pkgFile.Requires() // []rpm.Dependency
		reqNames := make([]string, len(reqDeps))
		for i, dep := range reqDeps {
			reqNames[i] = dep.Name() // call method
			// logger.Debugf("RPM %s requires %s", rpmPath, reqNames[i])
		}
		idx.Requires[rpmPath] = reqNames
	}

	return idx, nil
}

// ResolveDependencies returns the full set of RPMs needed
// starting from the given root paths, walking requires -> provides.
func ResolveDependencies(roots []string, idx *Index) []string {
	logger := zap.L().Sugar()
	needed := make(map[string]struct{})
	queue := append([]string{}, roots...)

	logger.Infof("resolving dependencies for %d RPMs", len(roots))
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if _, seen := needed[cur]; seen {
			continue
		}
		needed[cur] = struct{}{}

		// Enqueue all providers of each required capability
		for _, capName := range idx.Requires[cur] {
			for _, providerRPM := range idx.Provides[capName] {
				if _, seen := needed[providerRPM]; !seen {
					queue = append(queue, providerRPM)
				}
			}
		}
	}

	// Collect result
	result := make([]string, 0, len(needed))
	for pkg := range needed {
		result = append(result, pkg)
	}
	return result
}
