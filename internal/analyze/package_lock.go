package analyze

import (
	"encoding/json"
	"fmt"
	"slices"
)

type packageLockParser struct{}

type packageLockFile struct {
	LockfileVersion int                           `json:"lockfileVersion"`
	Dependencies    map[string]json.RawMessage    `json:"dependencies"`
	Packages        map[string]packageLockPackage `json:"packages"`
}

type packageLockPackage struct {
	Dependencies         map[string]json.RawMessage `json:"dependencies"`
	OptionalDependencies map[string]json.RawMessage `json:"optionalDependencies"`
}

func newPackageLockParser(raw packageLockMatcherConfig) (manifestParser, error) {
	return packageLockParser{}, nil
}

func (p packageLockParser) Match(path string, content []byte) (manifestParserResult, error) {
	var file packageLockFile
	if err := json.Unmarshal(content, &file); err != nil {
		return manifestParserResult{}, fmt.Errorf("parse json file %q: %w", path, err)
	}

	switch file.LockfileVersion {
	case 1:
		return matchPackageLockDependencies(file.Dependencies), nil
	case 2, 3:
		root, ok := file.Packages[""]
		if !ok {
			return manifestParserResult{
				Matched:         true,
				HasDependencies: boolPtr(false),
			}, nil
		}
		dependencyNames := collectPackageLockDependencyNames(root.Dependencies, root.OptionalDependencies)
		return packageLockResultFromNames(dependencyNames), nil
	default:
		return manifestParserResult{}, nil
	}
}

func matchPackageLockDependencies(dependencies map[string]json.RawMessage) manifestParserResult {
	return packageLockResultFromNames(mapKeys(dependencies))
}

func collectPackageLockDependencyNames(dependencies map[string]json.RawMessage, optionalDependencies map[string]json.RawMessage) []string {
	names := make([]string, 0, len(dependencies)+len(optionalDependencies))
	names = append(names, mapKeys(dependencies)...)
	names = append(names, mapKeys(optionalDependencies)...)

	if len(names) == 0 {
		return nil
	}

	slices.Sort(names)
	return slices.Compact(names)
}

func packageLockResultFromNames(names []string) manifestParserResult {
	if len(names) == 0 {
		return manifestParserResult{
			Matched:         true,
			HasDependencies: boolPtr(false),
		}
	}

	return manifestParserResult{
		Dependencies:    dependenciesFromStrings(names),
		Matched:         true,
		HasDependencies: boolPtr(true),
	}
}

func mapKeys(values map[string]json.RawMessage) []string {
	if len(values) == 0 {
		return nil
	}

	names := make([]string, 0, len(values))
	for name := range values {
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil
	}
	slices.Sort(names)
	return slices.Compact(names)
}
