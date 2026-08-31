package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
)

type rRenvLockParser struct{}

type rRenvLockFile struct {
	R        *rRenvLockR                 `json:"R"`
	Packages map[string]rRenvLockPackage `json:"Packages"`
}

type rRenvLockR struct {
	Version string `json:"Version"`
}

type rRenvLockPackage struct {
	Package string `json:"Package"`
	Version string `json:"Version"`
}

func newRRenvLockParser(rRenvLockParserConfig) (sourceAnalyzer, error) {
	return rRenvLockParser{}, nil
}

func (rRenvLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var lock rRenvLockFile
	if err := json.Unmarshal(content, &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse renv lockfile %q: %w", path, err)
	}
	if lock.R == nil || strings.TrimSpace(lock.R.Version) == "" || lock.Packages == nil {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(lock.Packages))
	seen := make(map[string]struct{}, len(lock.Packages))
	for packageName, pkg := range lock.Packages {
		name := strings.TrimSpace(pkg.Package)
		if name == "" {
			name = strings.TrimSpace(packageName)
		}
		version := strings.TrimSpace(pkg.Version)
		if name == "" || version == "" {
			continue
		}

		raw := name + "@" + version
		if _, exists := seen[raw]; exists {
			continue
		}
		seen[raw] = struct{}{}
		dependencies = append(dependencies, DependencyReference{
			Raw:     raw,
			Name:    name,
			Version: version,
		})
	}

	sortDependencyReferences(dependencies)
	if len(dependencies) == 0 {
		return sourceAnalyzerResult{
			Recognized: true,
			Analysis:   completeAnalysis(nil),
		}, nil
	}
	return sourceAnalyzerResult{
		Recognized:   true,
		Analysis:     completeAnalysis(dependencies),
		Dependencies: dependencies,
	}, nil
}
