package analyze

import (
	"fmt"

	"golang.org/x/mod/modfile"
)

type goModMatcherConfig struct{}

type goModMatcher struct{}

func newGoModMatcher(raw goModMatcherConfig) (manifestParser, error) {
	return goModMatcher{}, nil
}

func (m goModMatcher) Match(path string, content []byte) (manifestParserResult, error) {
	parsed, err := modfile.Parse(path, content, nil)
	if err != nil {
		return manifestParserResult{}, fmt.Errorf("parse go.mod file %q: %w", path, err)
	}

	dependencies := make([]Dependency, 0, len(parsed.Require))
	for _, req := range parsed.Require {
		if req.Indirect {
			continue
		}
		dependencies = append(dependencies, Dependency{Name: req.Mod.Path})
	}

	hasDependencies := len(dependencies) > 0
	return manifestParserResult{
		Dependencies:    dependencies,
		HasDependencies: boolPtr(hasDependencies),
		Matched:         true,
	}, nil
}
