package analyze

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
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
	Version      string                             `json:"version"`
	Dependencies map[string]packageLockV1Dependency `json:"dependencies"`
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
		return matchPackageLockV2V3(file), nil
	default:
		return manifestParserResult{}, nil
	}
}

func matchPackageLockV1Dependencies(dependencies map[string]json.RawMessage) manifestParserResult {
	if len(dependencies) == 0 {
		return manifestParserResult{
			Matched:         true,
			HasDependencies: boolPtr(false),
		}
	}
	seen := make(map[string]struct{})
	var resolved []packageLockDependency
	var visit func(name string, dep packageLockV1Dependency)
	visit = func(name string, dep packageLockV1Dependency) {
		if name == "" {
			return
		}
		key := name
		if dep.Version != "" {
			key = name + "@" + dep.Version
		}
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			resolved = append(resolved, packageLockDependency{Name: name, Version: dep.Version})
		}
		for _, n := range mapKeysV1ChildDeps(dep.Dependencies) {
			visit(n, dep.Dependencies[n])
		}
	}
	for _, name := range mapKeys(dependencies) {
		var parsed packageLockV1Dependency
		if err := json.Unmarshal(dependencies[name], &parsed); err != nil {
			continue
		}
		visit(name, parsed)
	}
	return packageLockResultFromDependencies(resolved)
}

func mapKeysV1ChildDeps(dependencies map[string]packageLockV1Dependency) []string {
	if len(dependencies) == 0 {
		return nil
	}
	keys := make([]string, 0, len(dependencies))
	for name := range dependencies {
		if name == "" {
			continue
		}
		keys = append(keys, name)
	}
	if len(keys) == 0 {
		return nil
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

func buildPackageLockRootSectionByName(packages map[string]packageLockPackage) map[string]string {
	root, ok := packages[""]
	if !ok {
		return nil
	}
	section := make(map[string]string)
	assign := func(names []string, sec string) {
		for _, name := range names {
			if name == "" {
				continue
			}
			if _, ok := section[name]; !ok {
				section[name] = sec
			}
		}
	}
	assign(mapKeys(root.Dependencies), "dependencies")
	assign(mapKeys(root.DevDependencies), "devDependencies")
	assign(mapKeys(root.OptionalDependencies), "optionalDependencies")
	return section
}

func packageNameFromNodeModulesPath(path string) string {
	idx := strings.LastIndex(path, "node_modules/")
	if idx == -1 {
		return ""
	}
	rest := path[idx+len("node_modules/"):]
	if strings.HasPrefix(rest, "@") {
		parts := strings.SplitN(rest, "/", 3)
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	return strings.SplitN(rest, "/", 2)[0]
}

func matchPackageLockV2V3FromRootOnly(file packageLockFile, root packageLockPackage) manifestParserResult {
	seen := make(map[string]struct{})
	var resolved []packageLockDependency
	appendWithDedupe := func(names []string, sec string) {
		for _, name := range names {
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			dep := packageLockDependency{Name: name, Section: sec}
			if p, ok := file.Packages[packagePathForPackageName(name)]; ok {
				dep.Version = p.Version
			}
			resolved = append(resolved, dep)
		}
	}
	appendWithDedupe(mapKeys(root.Dependencies), "dependencies")
	appendWithDedupe(mapKeys(root.DevDependencies), "devDependencies")
	appendWithDedupe(mapKeys(root.OptionalDependencies), "optionalDependencies")
	return packageLockResultFromDependencies(resolved)
}

func packagePathForPackageName(name string) string {
	// lockfile "packages" key for a top-level install (v2+). Scoped names include one slash in the name.
	if strings.HasPrefix(name, "@") {
		if idx := strings.Index(name, "/"); idx != -1 {
			return "node_modules/" + name
		}
	}
	return "node_modules/" + name
}

func matchPackageLockV2V3(file packageLockFile) manifestParserResult {
	if len(file.Packages) == 0 {
		return manifestParserResult{
			Matched:         true,
			HasDependencies: boolPtr(false),
		}
	}
	sectionByName := buildPackageLockRootSectionByName(file.Packages)
	paths := make([]string, 0, len(file.Packages))
	for p := range file.Packages {
		if p == "" {
			continue
		}
		paths = append(paths, p)
	}
	// `packages` with only the root `""` entry (no `node_modules/*` yet) is valid; use root's dep maps.
	if len(paths) == 0 {
		root, ok := file.Packages[""]
		if !ok {
			return manifestParserResult{
				Matched:         true,
				HasDependencies: boolPtr(false),
			}
		}
		return matchPackageLockV2V3FromRootOnly(file, root)
	}
	slices.Sort(paths)
	var resolved []packageLockDependency
	for _, path := range paths {
		name := packageNameFromNodeModulesPath(path)
		if name == "" {
			continue
		}
		pkg := file.Packages[path]
		dep := packageLockDependency{
			Name:    name,
			Version: pkg.Version,
		}
		if s, ok := sectionByName[name]; ok {
			dep.Section = s
		}
		resolved = append(resolved, dep)
	}
	return packageLockResultFromDependencies(resolved)
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
