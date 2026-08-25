package analyze

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type gleamMatcherConfig struct{}

type gleamParser struct{}

type gleamDependencyGroup struct {
	name  string
	scope DependencyScope
}

var gleamDependencyGroups = []gleamDependencyGroup{
	{name: "dependencies", scope: ScopeRuntime},
	{name: "dev-dependencies", scope: ScopeTest},
}

func newGleamParser(gleamMatcherConfig) (sourceAnalyzer, error) {
	return gleamParser{}, nil
}

func (gleamParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var manifest map[string]any
	if err := toml.Unmarshal(content, &manifest); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Gleam manifest %q: %w", path, err)
	}
	if !isGleamManifest(manifest) {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, group := range gleamDependencyGroups {
		rawValues, exists := manifest[group.name]
		if !exists || rawValues == nil {
			continue
		}
		values, ok := anyStringMap(rawValues)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("%s: expected a table of dependency declarations", group.name))
			continue
		}
		for _, declaredName := range sortedAnyMapKeys(values) {
			dependency, message := gleamDependency(declaredName, values[declaredName], group)
			if message != "" {
				incomplete = append(incomplete, message)
				continue
			}
			dependencies = append(dependencies, dependency)
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func isGleamManifest(manifest map[string]any) bool {
	name, ok := manifest["name"].(string)
	return ok && strings.TrimSpace(name) != ""
}

func gleamDependency(declaredName string, rawValue any, group gleamDependencyGroup) (DependencyReference, string) {
	name := strings.TrimSpace(declaredName)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("%s: package name must not be empty", group.name)
	}
	constraint, ok := rawValue.(string)
	if !ok {
		return DependencyReference{}, fmt.Sprintf("%s.%s: version constraint must be a string", group.name, name)
	}
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return DependencyReference{}, fmt.Sprintf("%s.%s: version constraint must not be empty", group.name, name)
	}
	return DependencyReference{
		Raw:               name + "@" + constraint,
		Name:              name,
		VersionConstraint: constraint,
		SourceGroup:       group.name,
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             group.scope,
	}, ""
}
