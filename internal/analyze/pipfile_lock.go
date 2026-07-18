package analyze

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type pipfileLockParser struct{}

type pipfileLockFile struct {
	Meta    json.RawMessage                  `json:"_meta"`
	Default map[string]pipfileLockDependency `json:"default"`
	Develop map[string]pipfileLockDependency `json:"develop"`
}

type pipfileLockDependency struct {
	Version string `json:"version"`
}

func newPipfileLockParser(raw pipfileLockMatcherConfig) (sourceAnalyzer, error) {
	return pipfileLockParser{}, nil
}

func (p pipfileLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var file pipfileLockFile
	if err := json.Unmarshal(content, &file); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse json file %q: %w", path, err)
	}

	if file.Meta == nil && file.Default == nil && file.Develop == nil {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := pipfileLockDependencies(
		pipfileLockDependencyGroup{Name: "default", Packages: file.Default},
		pipfileLockDependencyGroup{Name: "develop", Packages: file.Develop},
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

type pipfileLockDependencyGroup struct {
	Name     string
	Packages map[string]pipfileLockDependency
}

func pipfileLockDependencies(groups ...pipfileLockDependencyGroup) []DependencyReference {
	if len(groups) == 0 {
		return nil
	}

	values := make([]DependencyReference, 0)
	for _, group := range groups {
		if group.Name == "" || len(group.Packages) == 0 {
			continue
		}

		for _, name := range sortedStringKeys(group.Packages) {
			if name == "" {
				continue
			}

			rawVersion := group.Packages[name].Version
			raw := name
			version := ""
			if rawVersion != "" {
				raw = name + rawVersion
				version = strings.TrimPrefix(rawVersion, "==")
			}
			values = append(values, DependencyReference{
				Raw:         raw,
				Name:        name,
				Version:     version,
				SourceGroup: group.Name,
			})
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

func sortedStringKeys[T any](values map[string]T) []string {
	if len(values) == 0 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil
	}

	slices.Sort(keys)
	return keys
}
