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
	Version              string                     `json:"version"`
}

type packageLockV1Dependency struct {
	Version string `json:"version"`
}

type packageLockDependency struct {
	Name    string
	Version string
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
		return matchPackageLockV1Dependencies(file.Dependencies), nil
	case 2, 3:
		root, ok := file.Packages[""]
		if !ok {
			return manifestParserResult{
				Matched:         true,
				HasDependencies: boolPtr(false),
			}, nil
		}
		dependencyNames := collectPackageLockDependencyNames(root.Dependencies, root.OptionalDependencies)
		return packageLockResultFromDependencies(resolvePackageLockDependencies(dependencyNames, file.Packages)), nil
	default:
		return manifestParserResult{}, nil
	}
}

func matchPackageLockV1Dependencies(dependencies map[string]json.RawMessage) manifestParserResult {
	resolved := make([]packageLockDependency, 0, len(dependencies))
	for _, name := range mapKeys(dependencies) {
		dependency := packageLockDependency{Name: name}
		var parsed packageLockV1Dependency
		if err := json.Unmarshal(dependencies[name], &parsed); err == nil {
			dependency.Version = parsed.Version
		}
		resolved = append(resolved, dependency)
	}
	return packageLockResultFromDependencies(resolved)
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

func resolvePackageLockDependencies(names []string, packages map[string]packageLockPackage) []packageLockDependency {
	dependencies := make([]packageLockDependency, 0, len(names))
	for _, name := range names {
		dependency := packageLockDependency{Name: name}
		if pkg, ok := packages[packageLockPackagePath(name)]; ok {
			dependency.Version = pkg.Version
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies
}

func packageLockPackagePath(name string) string {
	return "node_modules/" + name
}

func packageLockResultFromDependencies(resolved []packageLockDependency) manifestParserResult {
	if len(resolved) == 0 {
		return manifestParserResult{
			Matched:         true,
			HasDependencies: boolPtr(false),
		}
	}

	return manifestParserResult{
		Dependencies:    dependenciesFromStrings(formatPackageLockDependencies(resolved)),
		Matched:         true,
		HasDependencies: boolPtr(true),
	}
}

func formatPackageLockDependencies(resolved []packageLockDependency) []string {
	values := make([]string, 0, len(resolved))
	for _, dependency := range resolved {
		if dependency.Name == "" {
			continue
		}
		if dependency.Version == "" {
			values = append(values, dependency.Name)
			continue
		}
		values = append(values, dependency.Name+"@"+dependency.Version)
	}
	return values
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
