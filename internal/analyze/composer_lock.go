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

func newComposerLockParser(raw composerLockMatcherConfig) (sourceAnalyzer, error) {
	return composerLockParser{}, nil
}

func (p composerLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var file composerLockFile
	if err := json.Unmarshal(content, &file); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse json file %q: %w", path, err)
	}

	if file.Packages == nil && file.PackagesDev == nil {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := composerLockDependencies(
		composerLockDependencyGroup{Name: "packages", Packages: file.Packages},
		composerLockDependencyGroup{Name: "packages-dev", Packages: file.PackagesDev},
	)
	if len(dependencies) == 0 {
		return sourceAnalyzerResult{
			Recognized: true,
			Analysis:   SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
		}, nil
	}

	return sourceAnalyzerResult{
		Dependencies: dependencies,
		Recognized:   true,
		Analysis:     SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
	}, nil
}

type composerLockDependencyGroup struct {
	Name     string
	Packages []composerLockPackage
}

func composerLockDependencies(groups ...composerLockDependencyGroup) []DependencyReference {
	if len(groups) == 0 {
		return nil
	}

	values := make([]DependencyReference, 0)
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
			dep := DependencyReference{
				Raw:         raw,
				Name:        pkg.Name,
				Version:     pkg.Version,
				SourceGroup: group.Name,
			}
			if pkg.Type != "" {
				dep.Attributes = map[string]string{"package_type": pkg.Type}
			}
			values = append(values, dep)
		}
	}

	if len(values) == 0 {
		return nil
	}

	slices.SortFunc(values, func(a, b DependencyReference) int {
		if a.SourceGroup == b.SourceGroup {
			switch {
			case a.Raw < b.Raw:
				return -1
			case a.Raw > b.Raw:
				return 1
			default:
				return 0
			}
		}
		if a.SourceGroup < b.SourceGroup {
			return -1
		}
		return 1
	})
	return values
}
