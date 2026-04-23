package analyze

import (
	"encoding/json"
	"fmt"
	"slices"
)

type composerLockParser struct{}

type composerLockFile struct {
	Packages    []composerLockPackage `json:"packages"`
	PackagesDev []composerLockPackage `json:"packages-dev"`
}

type composerLockPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func newComposerLockParser(raw composerLockMatcherConfig) (manifestParser, error) {
	return composerLockParser{}, nil
}

func (p composerLockParser) Match(path string, content []byte) (manifestParserResult, error) {
	var file composerLockFile
	if err := json.Unmarshal(content, &file); err != nil {
		return manifestParserResult{}, fmt.Errorf("parse json file %q: %w", path, err)
	}

	if file.Packages == nil && file.PackagesDev == nil {
		return manifestParserResult{}, nil
	}

	dependencies := composerLockDependencies(file.Packages, file.PackagesDev)
	if len(dependencies) == 0 {
		return manifestParserResult{
			Matched:         true,
			HasDependencies: boolPtr(false),
		}, nil
	}

	return manifestParserResult{
		Dependencies:    dependenciesFromStrings(dependencies),
		Matched:         true,
		HasDependencies: boolPtr(true),
	}, nil
}

func composerLockDependencies(groups ...[]composerLockPackage) []string {
	if len(groups) == 0 {
		return nil
	}

	values := make([]string, 0)
	seen := make(map[string]struct{})
	for _, group := range groups {
		for _, pkg := range group {
			if pkg.Name == "" {
				continue
			}

			value := pkg.Name
			if pkg.Version != "" {
				value += "@" + pkg.Version
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			values = append(values, value)
		}
	}

	if len(values) == 0 {
		return nil
	}

	slices.Sort(values)
	return values
}
