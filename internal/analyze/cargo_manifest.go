package analyze

import (
	"fmt"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

type cargoManifestParser struct{}

type cargoManifestGroup struct {
	name         string
	scope        DependencyScope
	relationship Relationship
	target       string
	values       map[string]any
}

func newCargoManifestParser(cargoManifestMatcherConfig) (sourceAnalyzer, error) {
	return cargoManifestParser{}, nil
}

func (cargoManifestParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root map[string]any
	if err := toml.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Cargo manifest %q: %w", path, err)
	}

	groups := cargoManifestGroups(root)
	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, group := range groups {
		names := sortedAnyMapKeys(group.values)
		for _, declaredName := range names {
			dependency, ok, message := cargoManifestDependency(declaredName, group.values[declaredName], group)
			if message != "" {
				incomplete = append(incomplete, message)
			}
			if ok {
				dependencies = append(dependencies, dependency)
			}
		}
	}
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func cargoManifestGroups(root map[string]any) []cargoManifestGroup {
	groups := make([]cargoManifestGroup, 0)
	appendGroup := func(name string, scope DependencyScope, relationship Relationship, target string, parent map[string]any) {
		if values, ok := anyStringMap(parent[name]); ok {
			sourceGroup := name
			if target != "" {
				sourceGroup = "target." + target + "." + name
			}
			groups = append(groups, cargoManifestGroup{name: sourceGroup, scope: scope, relationship: relationship, target: target, values: values})
		}
	}

	appendGroup("dependencies", ScopeRuntime, RelationshipDirect, "", root)
	appendGroup("dev-dependencies", ScopeTest, RelationshipDirect, "", root)
	appendGroup("build-dependencies", ScopeBuild, RelationshipDirect, "", root)

	if targets, ok := anyStringMap(root["target"]); ok {
		for _, target := range sortedAnyMapKeys(targets) {
			targetTable, ok := anyStringMap(targets[target])
			if !ok {
				continue
			}
			appendGroup("dependencies", ScopeRuntime, RelationshipDirect, target, targetTable)
			appendGroup("dev-dependencies", ScopeTest, RelationshipDirect, target, targetTable)
			appendGroup("build-dependencies", ScopeBuild, RelationshipDirect, target, targetTable)
		}
	}
	if workspace, ok := anyStringMap(root["workspace"]); ok {
		if values, ok := anyStringMap(workspace["dependencies"]); ok {
			groups = append(groups, cargoManifestGroup{
				name: "workspace.dependencies", scope: ScopeRuntime,
				relationship: RelationshipInconclusive, values: values,
			})
		}
	}
	return groups
}

func cargoManifestDependency(declaredName string, rawValue any, group cargoManifestGroup) (DependencyReference, bool, string) {
	if strings.TrimSpace(declaredName) == "" {
		return DependencyReference{}, false, fmt.Sprintf("%s: dependency name must not be empty", group.name)
	}
	dependency := DependencyReference{
		Raw:          declaredName,
		Name:         declaredName,
		SourceGroup:  group.name,
		Relationship: group.relationship,
		Scope:        group.scope,
	}
	if group.target != "" {
		dependency.Attributes = map[string]string{"target": group.target}
	}

	switch value := rawValue.(type) {
	case string:
		dependency.Raw += "@" + value
		dependency.VersionConstraint = value
		dependency.OriginKind = OriginRegistry
		return dependency, true, ""
	case map[string]any:
		return populateCargoTableDependency(dependency, declaredName, value, group.name)
	default:
		if converted, ok := anyStringMap(rawValue); ok {
			return populateCargoTableDependency(dependency, declaredName, converted, group.name)
		}
		return DependencyReference{}, false, fmt.Sprintf("%s.%s: expected a string or table dependency specification", group.name, declaredName)
	}
}

func populateCargoTableDependency(dependency DependencyReference, declaredName string, value map[string]any, group string) (DependencyReference, bool, string) {
	if actualName, ok := value["package"].(string); ok && strings.TrimSpace(actualName) != "" {
		dependency.Name = strings.TrimSpace(actualName)
		ensureDependencyAttribute(&dependency, "declared_name", declaredName)
	}
	version, _ := value["version"].(string)
	gitURL, _ := value["git"].(string)
	sourcePath, _ := value["path"].(string)
	registry, _ := value["registry"].(string)
	workspace, _ := value["workspace"].(bool)

	switch {
	case strings.TrimSpace(gitURL) != "":
		dependency.OriginKind = OriginGit
		ensureDependencyAttribute(&dependency, "source_url", strings.TrimSpace(gitURL))
		for _, field := range []string{"rev", "tag", "branch"} {
			if sourceRef, ok := value[field].(string); ok && strings.TrimSpace(sourceRef) != "" {
				ensureDependencyAttribute(&dependency, "source_ref", strings.TrimSpace(sourceRef))
				ensureDependencyAttribute(&dependency, "source_ref_kind", field)
				break
			}
		}
	case strings.TrimSpace(sourcePath) != "":
		dependency.OriginKind = OriginPath
		ensureDependencyAttribute(&dependency, "path", strings.TrimSpace(sourcePath))
	case workspace:
		dependency.OriginKind = OriginWorkspace
	default:
		dependency.OriginKind = OriginRegistry
		if strings.TrimSpace(registry) != "" {
			ensureDependencyAttribute(&dependency, "registry", strings.TrimSpace(registry))
		}
	}

	if strings.TrimSpace(version) != "" {
		dependency.VersionConstraint = strings.TrimSpace(version)
		dependency.Raw += "@" + strings.TrimSpace(version)
	} else {
		switch dependency.OriginKind {
		case OriginGit:
			dependency.Raw += "@" + strings.TrimSpace(gitURL)
		case OriginPath:
			dependency.Raw += "@" + strings.TrimSpace(sourcePath)
		case OriginWorkspace:
			dependency.Raw += "@workspace"
		}
	}
	if optional, ok := value["optional"].(bool); ok && optional {
		dependency.Scope = ScopeOptional
	}
	if dependency.OriginKind == OriginRegistry && version == "" {
		return dependency, true, fmt.Sprintf("%s.%s: registry dependency has no string version constraint", group, declaredName)
	}
	return dependency, true, ""
}

func anyStringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	default:
		return nil, false
	}
}

func sortedAnyMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func ensureDependencyAttribute(dependency *DependencyReference, key, value string) {
	if dependency.Attributes == nil {
		dependency.Attributes = make(map[string]string)
	}
	dependency.Attributes[key] = value
}
