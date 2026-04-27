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
	Type    string `json:"type"`
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

	dependencies := composerLockDependencies(
		composerLockDependencyGroup{Name: "packages", Packages: file.Packages},
		composerLockDependencyGroup{Name: "packages-dev", Packages: file.PackagesDev},
	)
	if len(dependencies) == 0 {
		return manifestParserResult{
			Matched:         true,
			HasDependencies: boolPtr(false),
		}, nil
	}

	return manifestParserResult{
		Dependencies:    dependencies,
		Matched:         true,
		HasDependencies: boolPtr(true),
	}, nil
}

type composerLockDependencyGroup struct {
	Name     string
	Packages []composerLockPackage
}

func composerLockDependencies(groups ...composerLockDependencyGroup) []Dependency {
	if len(groups) == 0 {
		return nil
	}

	values := make([]Dependency, 0)
	for _, group := range groups {
		if group.Name == "" {
			continue
		}

		seen := make(map[string]struct{})
		for _, pkg := range group.Packages {
			if pkg.Name == "" {
				continue
			}

			raw := pkg.Name
			if pkg.Version != "" {
				raw += "@" + pkg.Version
			}
			if _, ok := seen[raw]; ok {
				continue
			}
			seen[raw] = struct{}{}
			dep := Dependency{
				Raw:     raw,
				Name:    pkg.Name,
				Version: pkg.Version,
				Section: group.Name,
			}
			if pkg.Type != "" {
				dep.Extras = map[string]string{"package_type": pkg.Type}
			}
			values = append(values, dep)
		}
	}

	if len(values) == 0 {
		return nil
	}

	slices.SortFunc(values, func(a, b Dependency) int {
		if a.Section == b.Section {
			switch {
			case a.Raw < b.Raw:
				return -1
			case a.Raw > b.Raw:
				return 1
			default:
				return 0
			}
		}
		if a.Section < b.Section {
			return -1
		}
		return 1
	})
	return values
}
