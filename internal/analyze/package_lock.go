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
	DevDependencies      map[string]json.RawMessage `json:"devDependencies"`
	OptionalDependencies map[string]json.RawMessage `json:"optionalDependencies"`
	Version              string                     `json:"version"`
}

type packageLockV1Dependency struct {
	Version string `json:"version"`
}

type packageLockDependency struct {
	Name    string
	Version string
	Section string
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
		seen := make(map[string]struct{})
		var resolved []packageLockDependency
		appendWithDedupe := func(names []string, section string) {
			for _, name := range names {
				if name == "" {
					continue
				}
				if _, ok := seen[name]; ok {
					continue
				}
				seen[name] = struct{}{}
				dep := packageLockDependency{Name: name, Section: section}
				if pkg, ok := file.Packages[packageLockPackagePath(name)]; ok {
					dep.Version = pkg.Version
				}
				resolved = append(resolved, dep)
			}
		}
		appendWithDedupe(mapKeys(root.Dependencies), "dependencies")
		appendWithDedupe(mapKeys(root.DevDependencies), "devDependencies")
		appendWithDedupe(mapKeys(root.OptionalDependencies), "optionalDependencies")
		return packageLockResultFromDependencies(resolved), nil
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

	deps := make([]Dependency, 0, len(resolved))
	for _, r := range resolved {
		if r.Name == "" {
			continue
		}
		dep := Dependency{
			Raw:     r.Name,
			Name:    r.Name,
			Section: r.Section,
		}
		if r.Version != "" {
			dep.Raw = r.Name + "@" + r.Version
			dep.Version = r.Version
		}
		deps = append(deps, dep)
	}
	return manifestParserResult{
		Dependencies:    deps,
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
