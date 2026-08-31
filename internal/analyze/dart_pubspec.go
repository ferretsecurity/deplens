package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type dartPubspecParser struct{}

type dartPubspecDependencyGroup struct {
	name  string
	scope DependencyScope
}

var dartPubspecDependencyGroups = []dartPubspecDependencyGroup{
	{name: "dependencies", scope: ScopeRuntime},
	{name: "dev_dependencies", scope: ScopeDevelopment},
	{name: "dependency_overrides", scope: ScopeRuntime},
}

func newDartPubspecParser(dartPubspecMatcherConfig) (sourceAnalyzer, error) {
	return dartPubspecParser{}, nil
}

func (dartPubspecParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var manifest map[string]any
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Dart pubspec %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, group := range dartPubspecDependencyGroups {
		entries, exists := manifest[group.name]
		if !exists || entries == nil {
			continue
		}

		values, ok := anyStringMap(entries)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("%s: expected an object of dependency declarations", group.name))
			continue
		}
		for _, declaredName := range sortedAnyMapKeys(values) {
			dependency, message := dartPubspecDependency(declaredName, values[declaredName], group)
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

func dartPubspecDependency(declaredName string, raw any, group dartPubspecDependencyGroup) (DependencyReference, string) {
	name := strings.TrimSpace(declaredName)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("%s: dependency name must not be empty", group.name)
	}

	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  group.name,
		Relationship: RelationshipDirect,
		Scope:        group.scope,
	}

	switch value := raw.(type) {
	case string:
		constraint := strings.TrimSpace(value)
		if constraint == "" {
			return DependencyReference{}, fmt.Sprintf("%s.%s: version constraint must not be empty", group.name, name)
		}
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
		dependency.OriginKind = OriginRegistry
		return dependency, ""
	default:
		values, ok := anyStringMap(value)
		if !ok {
			return DependencyReference{}, fmt.Sprintf("%s.%s: expected a string or object dependency declaration", group.name, name)
		}
		return dartPubspecSourceDependency(dependency, values, group.name)
	}
}

func dartPubspecSourceDependency(dependency DependencyReference, values map[string]any, group string) (DependencyReference, string) {
	if sdk, ok := dartPubspecString(values, "sdk"); ok {
		if sdk == "" {
			return DependencyReference{}, fmt.Sprintf("%s.%s.sdk: must not be empty", group, dependency.Name)
		}
		dependency.Raw += "@sdk:" + sdk
		dependency.Attributes = map[string]string{"sdk": sdk}
		return dependency, ""
	}
	if sourcePath, ok := dartPubspecString(values, "path"); ok {
		if sourcePath == "" {
			return DependencyReference{}, fmt.Sprintf("%s.%s.path: must not be empty", group, dependency.Name)
		}
		dependency.Raw += "@" + sourcePath
		dependency.OriginKind = OriginPath
		dependency.Attributes = map[string]string{"path": sourcePath}
		return dependency, ""
	}
	if git, exists := values["git"]; exists {
		return dartPubspecGitDependency(dependency, git, group)
	}
	if hosted, exists := values["hosted"]; exists {
		return dartPubspecHostedDependency(dependency, hosted, group)
	}
	return DependencyReference{}, fmt.Sprintf("%s.%s: expected sdk, path, git, or hosted source", group, dependency.Name)
}

func dartPubspecGitDependency(dependency DependencyReference, raw any, group string) (DependencyReference, string) {
	url := ""
	attributes := make(map[string]string)
	switch value := raw.(type) {
	case string:
		url = strings.TrimSpace(value)
	default:
		values, ok := anyStringMap(value)
		if !ok {
			return DependencyReference{}, fmt.Sprintf("%s.%s.git: expected a URL string or object", group, dependency.Name)
		}
		url, _ = dartPubspecString(values, "url")
		if ref, ok := dartPubspecString(values, "ref"); ok && ref != "" {
			attributes["source_ref"] = ref
		}
		if sourcePath, ok := dartPubspecString(values, "path"); ok && sourcePath != "" {
			attributes["source_path"] = sourcePath
		}
	}
	if url == "" {
		return DependencyReference{}, fmt.Sprintf("%s.%s.git: URL is required", group, dependency.Name)
	}

	dependency.Raw += "@" + url
	dependency.OriginKind = OriginGit
	attributes["source_url"] = url
	dependency.Attributes = attributes
	return dependency, ""
}

func dartPubspecHostedDependency(dependency DependencyReference, raw any, group string) (DependencyReference, string) {
	hostedURL := ""
	switch value := raw.(type) {
	case string:
		hostedURL = strings.TrimSpace(value)
	default:
		values, ok := anyStringMap(value)
		if !ok {
			return DependencyReference{}, fmt.Sprintf("%s.%s.hosted: expected a URL string or object", group, dependency.Name)
		}
		hostedURL, _ = dartPubspecString(values, "url")
		if packageName, ok := dartPubspecString(values, "name"); ok && packageName != "" && packageName != dependency.Name {
			dependency.Attributes = map[string]string{"declared_name": dependency.Name}
			dependency.Name = packageName
		}
	}
	if hostedURL == "" {
		return DependencyReference{}, fmt.Sprintf("%s.%s.hosted: URL is required", group, dependency.Name)
	}

	dependency.Raw += "@hosted:" + hostedURL
	dependency.OriginKind = OriginRegistry
	ensureDependencyAttribute(&dependency, "hosted_url", hostedURL)
	return dependency, ""
}

func dartPubspecString(values map[string]any, key string) (string, bool) {
	value, exists := values[key]
	if !exists {
		return "", false
	}
	stringValue, ok := value.(string)
	return strings.TrimSpace(stringValue), ok
}
