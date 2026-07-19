package analyze

import (
	"fmt"

	"golang.org/x/mod/modfile"
)

type goModMatcherConfig struct{}

type goModMatcher struct{}

func newGoModMatcher(raw goModMatcherConfig) (sourceAnalyzer, error) {
	return goModMatcher{}, nil
}

func (m goModMatcher) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	parsed, err := modfile.Parse(path, content, nil)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse go.mod file %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0, len(parsed.Require))
	for _, req := range parsed.Require {
		dep := DependencyReference{
			Raw:     req.Mod.Path,
			Name:    req.Mod.Path,
			Version: req.Mod.Version,
		}
		if req.Indirect {
			dep.SourceGroup = "indirect"
		}
		dependencies = append(dependencies, dep)
	}

	return sourceAnalyzerResult{
		Dependencies: dependencies,
		Analysis:     completeAnalysis(dependencies),
		Recognized:   true,
	}, nil
}
