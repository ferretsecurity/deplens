package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type crystalShardMatcherConfig struct{}

type crystalShardParser struct{}

type crystalShardDependencyGroup struct {
	name  string
	scope DependencyScope
}

var crystalShardDependencyGroups = []crystalShardDependencyGroup{
	{name: "dependencies", scope: ScopeRuntime},
	{name: "development_dependencies", scope: ScopeDevelopment},
}

func newCrystalShardParser(crystalShardMatcherConfig) (sourceAnalyzer, error) {
	return crystalShardParser{}, nil
}

func (crystalShardParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var manifest map[string]any
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Crystal Shard manifest %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, group := range crystalShardDependencyGroups {
		dependencies, incomplete = appendCrystalShardDependencyGroup(dependencies, incomplete, manifest[group.name], group)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func appendCrystalShardDependencyGroup(dependencies []DependencyReference, incomplete []string, raw any, group crystalShardDependencyGroup) ([]DependencyReference, []string) {
	if raw == nil {
		return dependencies, incomplete
	}

	entries, ok := anyStringMap(raw)
	if !ok {
		return dependencies, append(incomplete, fmt.Sprintf("%s: expected an object of dependency declarations", group.name))
	}
	for _, declaredName := range sortedAnyMapKeys(entries) {
		dependency, message := crystalShardDependency(declaredName, entries[declaredName], group)
		if message != "" {
			incomplete = append(incomplete, message)
		}
		if dependency.Name != "" {
			dependencies = append(dependencies, dependency)
		}
	}
	return dependencies, incomplete
}

func crystalShardDependency(declaredName string, raw any, group crystalShardDependencyGroup) (DependencyReference, string) {
	name := strings.TrimSpace(declaredName)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("%s: dependency name must not be empty", group.name)
	}
	declaration, ok := anyStringMap(raw)
	if !ok {
		return DependencyReference{}, fmt.Sprintf("%s.%s: expected an object of dependency fields", group.name, name)
	}

	dependency := DependencyReference{
		Name:         name,
		SourceGroup:  group.name,
		Relationship: RelationshipDirect,
		Scope:        group.scope,
	}
	github := crystalShardString(declaration, "github")
	gitURL := crystalShardString(declaration, "git")
	sourcePath := crystalShardString(declaration, "path")
	version := crystalShardString(declaration, "version")

	switch {
	case github != "":
		dependency.OriginKind = OriginGit
		ensureDependencyAttribute(&dependency, "source_url", "github:"+github)
	case gitURL != "":
		dependency.OriginKind = OriginGit
		ensureDependencyAttribute(&dependency, "source_url", gitURL)
	case sourcePath != "":
		dependency.OriginKind = OriginPath
		ensureDependencyAttribute(&dependency, "path", sourcePath)
	default:
		return DependencyReference{}, fmt.Sprintf("%s.%s: expected github, git, or path source", group.name, name)
	}

	if version != "" {
		dependency.VersionConstraint = version
		dependency.Raw = name + "@" + version
	} else if dependency.OriginKind == OriginPath {
		dependency.Raw = name + "@" + sourcePath
	} else {
		dependency.Raw = name + "@" + dependency.Attributes["source_url"]
	}

	for _, field := range []string{"branch", "tag", "commit"} {
		if ref := crystalShardString(declaration, field); ref != "" {
			ensureDependencyAttribute(&dependency, "source_ref", ref)
			ensureDependencyAttribute(&dependency, "source_ref_kind", field)
			break
		}
	}
	return dependency, ""
}

func crystalShardString(declaration map[string]any, field string) string {
	value, _ := declaration[field].(string)
	return strings.TrimSpace(value)
}
