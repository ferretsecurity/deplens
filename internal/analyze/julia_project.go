package analyze

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type juliaProjectMatcherConfig struct{}

type juliaProjectParser struct{}

type juliaProjectDependencyGroup struct {
	name  string
	scope DependencyScope
}

var juliaProjectDependencyGroups = []juliaProjectDependencyGroup{
	{name: "deps", scope: ScopeRuntime},
	{name: "weakdeps", scope: ScopeOptional},
	{name: "extras", scope: ScopeRuntime},
}

func newJuliaProjectParser(juliaProjectMatcherConfig) (sourceAnalyzer, error) {
	return juliaProjectParser{}, nil
}

func (juliaProjectParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var project map[string]any
	if err := toml.Unmarshal(content, &project); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Julia project %q: %w", path, err)
	}
	if !isJuliaProject(project) {
		return sourceAnalyzerResult{}, nil
	}

	compatibility, incomplete := juliaProjectCompatibility(project)
	testExtras := juliaProjectTestExtras(project)
	dependencies := make([]DependencyReference, 0)
	for _, group := range juliaProjectDependencyGroups {
		rawValues, exists := project[group.name]
		if !exists || rawValues == nil {
			continue
		}
		values, ok := anyStringMap(rawValues)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("%s: expected a table of package UUIDs", group.name))
			continue
		}
		for _, declaredName := range sortedAnyMapKeys(values) {
			dependency, message := juliaProjectDependency(declaredName, values[declaredName], group, compatibility[declaredName])
			if message != "" {
				incomplete = append(incomplete, message)
				continue
			}
			if group.name == "extras" && testExtras[dependency.Name] {
				dependency.Scope = ScopeTest
			}
			dependencies = append(dependencies, dependency)
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func isJuliaProject(project map[string]any) bool {
	for _, key := range []string{"deps", "weakdeps", "extras", "compat", "targets"} {
		if _, exists := project[key]; exists {
			return true
		}
	}
	name, hasName := project["name"].(string)
	uuid, hasUUID := project["uuid"].(string)
	return hasName && strings.TrimSpace(name) != "" && hasUUID && strings.TrimSpace(uuid) != ""
}

func juliaProjectCompatibility(project map[string]any) (map[string]string, []string) {
	compatibility := make(map[string]string)
	rawValues, exists := project["compat"]
	if !exists || rawValues == nil {
		return compatibility, nil
	}
	values, ok := anyStringMap(rawValues)
	if !ok {
		return compatibility, []string{"compat: expected a table of version constraints"}
	}

	incomplete := make([]string, 0)
	for _, name := range sortedAnyMapKeys(values) {
		constraint, ok := values[name].(string)
		if !ok || strings.TrimSpace(constraint) == "" {
			incomplete = append(incomplete, fmt.Sprintf("compat.%s: version constraint must be a non-empty string", name))
			continue
		}
		compatibility[name] = strings.TrimSpace(constraint)
	}
	return compatibility, incomplete
}

func juliaProjectTestExtras(project map[string]any) map[string]bool {
	testExtras := make(map[string]bool)
	rawTargets, exists := project["targets"]
	if !exists || rawTargets == nil {
		return testExtras
	}
	targets, ok := anyStringMap(rawTargets)
	if !ok {
		return testExtras
	}
	rawTest, exists := targets["test"]
	if !exists {
		return testExtras
	}
	values, ok := rawTest.([]any)
	if !ok {
		return testExtras
	}
	for _, value := range values {
		name, ok := value.(string)
		if !ok {
			continue
		}
		testExtras[name] = true
	}
	return testExtras
}

func juliaProjectDependency(declaredName string, rawValue any, group juliaProjectDependencyGroup, constraint string) (DependencyReference, string) {
	name := strings.TrimSpace(declaredName)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("%s: package name must not be empty", group.name)
	}
	uuid, ok := rawValue.(string)
	if !ok || strings.TrimSpace(uuid) == "" {
		return DependencyReference{}, fmt.Sprintf("%s.%s: package UUID must be a non-empty string", group.name, name)
	}

	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  group.name,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        group.scope,
		Attributes:   map[string]string{"uuid": strings.TrimSpace(uuid)},
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency, ""
}
