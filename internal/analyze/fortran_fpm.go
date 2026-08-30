package analyze

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type fortranFPMMatcherConfig struct{}

type fortranFPMParser struct{}

type fortranFPMDependencyGroup struct {
	name  string
	scope DependencyScope
}

var fortranFPMDependencyGroups = []fortranFPMDependencyGroup{
	{name: "dependencies", scope: ScopeRuntime},
	{name: "dev-dependencies", scope: ScopeTest},
}

func newFortranFPMParser(fortranFPMMatcherConfig) (sourceAnalyzer, error) {
	return fortranFPMParser{}, nil
}

func (fortranFPMParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var manifest map[string]any
	if err := toml.Unmarshal(content, &manifest); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Fortran fpm manifest %q: %w", path, err)
	}
	if !isFortranFPMManifest(manifest) {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, group := range fortranFPMDependencyGroups {
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
			dependency, message := fortranFPMDependency(declaredName, values[declaredName], group)
			if message != "" {
				incomplete = append(incomplete, message)
			}
			if dependency.Name != "" {
				dependencies = append(dependencies, dependency)
			}
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func isFortranFPMManifest(manifest map[string]any) bool {
	if name, ok := manifest["name"].(string); ok && strings.TrimSpace(name) != "" {
		return true
	}
	packageTable, ok := anyStringMap(manifest["package"])
	if !ok {
		return false
	}
	name, ok := packageTable["name"].(string)
	return ok && strings.TrimSpace(name) != ""
}

func fortranFPMDependency(declaredName string, rawValue any, group fortranFPMDependencyGroup) (DependencyReference, string) {
	name := strings.TrimSpace(declaredName)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("%s: dependency name must not be empty", group.name)
	}

	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  group.name,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        group.scope,
	}

	switch value := rawValue.(type) {
	case string:
		constraint := strings.TrimSpace(value)
		if constraint == "" {
			return DependencyReference{}, fmt.Sprintf("%s.%s: version constraint must not be empty", group.name, name)
		}
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
		return dependency, ""
	default:
		values, ok := anyStringMap(value)
		if !ok {
			return DependencyReference{}, fmt.Sprintf("%s.%s: expected a string or table dependency declaration", group.name, name)
		}
		return fortranFPMTableDependency(dependency, values, group.name)
	}
}

func fortranFPMTableDependency(dependency DependencyReference, values map[string]any, group string) (DependencyReference, string) {
	gitURL, hasGit := fortranFPMString(values, "git")
	sourcePath, hasPath := fortranFPMString(values, "path")
	version, hasVersion := fortranFPMString(values, "version")

	switch {
	case hasGit:
		if gitURL == "" {
			return DependencyReference{}, fmt.Sprintf("%s.%s.git: must not be empty", group, dependency.Name)
		}
		dependency.Raw += "@" + gitURL
		dependency.OriginKind = OriginGit
		dependency.Attributes = map[string]string{"source_url": gitURL}
		for _, refKind := range []string{"tag", "branch", "rev"} {
			if ref, ok := fortranFPMString(values, refKind); ok && ref != "" {
				dependency.Attributes["source_ref"] = ref
				dependency.Attributes["source_ref_kind"] = refKind
				break
			}
		}
	case hasPath:
		if sourcePath == "" {
			return DependencyReference{}, fmt.Sprintf("%s.%s.path: must not be empty", group, dependency.Name)
		}
		dependency.Raw += "@" + sourcePath
		dependency.OriginKind = OriginPath
		dependency.Attributes = map[string]string{"path": sourcePath}
	case hasVersion:
		if version == "" {
			return DependencyReference{}, fmt.Sprintf("%s.%s.version: must not be empty", group, dependency.Name)
		}
		dependency.Raw += "@" + version
		dependency.VersionConstraint = version
	default:
		return DependencyReference{}, fmt.Sprintf("%s.%s: expected git, path, or version dependency declaration", group, dependency.Name)
	}
	return dependency, ""
}

func fortranFPMString(values map[string]any, key string) (string, bool) {
	value, exists := values[key]
	if !exists {
		return "", false
	}
	stringValue, ok := value.(string)
	return strings.TrimSpace(stringValue), ok
}
