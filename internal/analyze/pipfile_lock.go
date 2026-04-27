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

func newPipfileLockParser(raw pipfileLockMatcherConfig) (manifestParser, error) {
	return pipfileLockParser{}, nil
}

func (p pipfileLockParser) Match(path string, content []byte) (manifestParserResult, error) {
	var file pipfileLockFile
	if err := json.Unmarshal(content, &file); err != nil {
		return manifestParserResult{}, fmt.Errorf("parse json file %q: %w", path, err)
	}

	if file.Meta == nil && file.Default == nil && file.Develop == nil {
		return manifestParserResult{}, nil
	}

	dependencies := pipfileLockDependencies(
		pipfileLockDependencyGroup{Name: "default", Packages: file.Default},
		pipfileLockDependencyGroup{Name: "develop", Packages: file.Develop},
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

type pipfileLockDependencyGroup struct {
	Name     string
	Packages map[string]pipfileLockDependency
}

func pipfileLockDependencies(groups ...pipfileLockDependencyGroup) []Dependency {
	if len(groups) == 0 {
		return nil
	}

	values := make([]Dependency, 0)
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
			values = append(values, Dependency{
				Raw:     raw,
				Name:    name,
				Version: version,
				Section: group.Name,
			})
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
