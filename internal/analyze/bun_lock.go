package analyze

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type bunLockParser struct{}

type bunLockFile struct {
	LockfileVersion int                          `json:"lockfileVersion"`
	Workspaces      map[string]bunLockWorkspace  `json:"workspaces"`
	Packages        map[string][]json.RawMessage `json:"packages"`
}

type bunLockWorkspace struct {
	Dependencies         map[string]json.RawMessage `json:"dependencies"`
	DevDependencies      map[string]json.RawMessage `json:"devDependencies"`
	OptionalDependencies map[string]json.RawMessage `json:"optionalDependencies"`
	PeerDependencies     map[string]json.RawMessage `json:"peerDependencies"`
}

func newBunLockParser(bunLockMatcherConfig) (sourceAnalyzer, error) {
	return bunLockParser{}, nil
}

func (bunLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	jsonContent, err := normalizeJSONC(content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Bun lockfile %q: %w", path, err)
	}

	var lock bunLockFile
	if err := json.Unmarshal(jsonContent, &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Bun lockfile %q: %w", path, err)
	}
	if lock.LockfileVersion != 1 {
		return sourceAnalyzerResult{}, nil
	}

	sourceGroupByName := bunLockRootSourceGroups(lock.Workspaces[""])
	dependencies := make([]DependencyReference, 0, len(lock.Packages))
	for _, packageName := range sortedBunLockPackageNames(lock.Packages) {
		name, version := bunLockPackageNameVersion(packageName, lock.Packages[packageName])
		if name == "" {
			continue
		}
		dependency := DependencyReference{Raw: name, Name: name}
		if version != "" {
			dependency.Raw = name + "@" + version
			dependency.Version = version
		}
		dependency.SourceGroup = sourceGroupByName[name]
		dependencies = append(dependencies, dependency)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func bunLockRootSourceGroups(workspace bunLockWorkspace) map[string]string {
	groups := make(map[string]string)
	add := func(dependencies map[string]json.RawMessage, sourceGroup string) {
		for name := range dependencies {
			if name == "" {
				continue
			}
			if _, exists := groups[name]; !exists {
				groups[name] = sourceGroup
			}
		}
	}
	add(workspace.Dependencies, "dependencies")
	add(workspace.DevDependencies, "devDependencies")
	add(workspace.OptionalDependencies, "optionalDependencies")
	add(workspace.PeerDependencies, "peerDependencies")
	return groups
}

func sortedBunLockPackageNames(packages map[string][]json.RawMessage) []string {
	names := make([]string, 0, len(packages))
	for name := range packages {
		if name != "" {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func bunLockPackageNameVersion(packageName string, entry []json.RawMessage) (name, version string) {
	if len(entry) == 0 {
		return packageName, ""
	}
	var resolved string
	if err := json.Unmarshal(entry[0], &resolved); err != nil || resolved == "" {
		return packageName, ""
	}
	if strings.HasPrefix(resolved, "@") {
		if index := strings.LastIndex(resolved[1:], "@"); index > 0 {
			return resolved[:index+1], resolved[index+2:]
		}
	} else if index := strings.LastIndex(resolved, "@"); index > 0 {
		return resolved[:index], resolved[index+1:]
	}
	return packageName, ""
}
