package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type dartPubspecLockParser struct{}

func newDartPubspecLockParser(dartPubspecLockMatcherConfig) (sourceAnalyzer, error) {
	return dartPubspecLockParser{}, nil
}

func (dartPubspecLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var lock struct {
		Packages map[string]any `yaml:"packages"`
	}
	if err := yaml.Unmarshal(content, &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Dart pubspec lockfile %q: %w", path, err)
	}
	if lock.Packages == nil {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(lock.Packages))
	incomplete := make([]string, 0)
	for _, declaredName := range sortedAnyMapKeys(lock.Packages) {
		dependency, message := dartPubspecLockDependency(declaredName, lock.Packages[declaredName])
		if message != "" {
			incomplete = append(incomplete, message)
		}
		if dependency.Name != "" {
			dependencies = append(dependencies, dependency)
		}
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func dartPubspecLockDependency(declaredName string, raw any) (DependencyReference, string) {
	name := strings.TrimSpace(declaredName)
	if name == "" {
		return DependencyReference{}, "packages: dependency name must not be empty"
	}
	entry, ok := anyStringMap(raw)
	if !ok {
		return DependencyReference{}, fmt.Sprintf("packages.%s: expected an object of resolved dependency fields", name)
	}

	version, ok := dartPubspecLockString(entry, "version")
	if !ok || version == "" {
		return DependencyReference{}, fmt.Sprintf("packages.%s.version: resolved version is required", name)
	}
	source, ok := dartPubspecLockString(entry, "source")
	if !ok || source == "" {
		return DependencyReference{}, fmt.Sprintf("packages.%s.source: source is required", name)
	}

	dependency := DependencyReference{
		Raw:         name + "@" + version,
		Name:        name,
		Version:     version,
		SourceGroup: "packages",
		Scope:       ScopeRuntime,
	}
	if relationship, scope := dartPubspecLockRelationship(entry); relationship != "" {
		dependency.Relationship = relationship
		dependency.Scope = scope
	}

	var message string
	switch source {
	case "hosted":
		dependency.OriginKind = OriginRegistry
		message = dartPubspecLockHosted(&dependency, entry["description"])
	case "git":
		dependency.OriginKind = OriginGit
		message = dartPubspecLockGit(&dependency, entry["description"])
	case "path":
		dependency.OriginKind = OriginPath
		message = dartPubspecLockPath(&dependency, entry["description"])
	case "sdk":
		message = dartPubspecLockSDK(&dependency, entry["description"])
	default:
		message = fmt.Sprintf("packages.%s.source: unsupported source %q", name, source)
	}
	if message != "" {
		return DependencyReference{}, message
	}
	return dependency, ""
}

func dartPubspecLockRelationship(entry map[string]any) (Relationship, DependencyScope) {
	dependency, exists := dartPubspecLockString(entry, "dependency")
	if !exists {
		return RelationshipInconclusive, ScopeRuntime
	}
	switch dependency {
	case "direct main":
		return RelationshipDirect, ScopeRuntime
	case "direct dev":
		return RelationshipDirect, ScopeDevelopment
	case "transitive":
		return RelationshipTransitive, ScopeRuntime
	default:
		return RelationshipInconclusive, ScopeRuntime
	}
}

func dartPubspecLockHosted(dependency *DependencyReference, raw any) string {
	switch description := raw.(type) {
	case string:
		if strings.TrimSpace(description) == "" {
			return fmt.Sprintf("packages.%s.description: hosted package name is required", dependency.Name)
		}
		return ""
	default:
		values, ok := anyStringMap(description)
		if !ok {
			return fmt.Sprintf("packages.%s.description: expected hosted package name or object", dependency.Name)
		}
		if name, ok := dartPubspecLockString(values, "name"); !ok || name == "" {
			return fmt.Sprintf("packages.%s.description.name: hosted package name is required", dependency.Name)
		}
		if url, ok := dartPubspecLockString(values, "url"); ok && url != "" {
			ensureDependencyAttribute(dependency, "hosted_url", url)
		}
		if checksum, ok := dartPubspecLockString(values, "sha256"); ok && checksum != "" {
			ensureDependencyAttribute(dependency, "sha256", checksum)
		}
		return ""
	}
}

func dartPubspecLockGit(dependency *DependencyReference, raw any) string {
	values, ok := anyStringMap(raw)
	if !ok {
		return fmt.Sprintf("packages.%s.description: expected Git source object", dependency.Name)
	}
	url, ok := dartPubspecLockString(values, "url")
	if !ok || url == "" {
		return fmt.Sprintf("packages.%s.description.url: Git URL is required", dependency.Name)
	}
	ensureDependencyAttribute(dependency, "source_url", url)
	if resolvedRef, ok := dartPubspecLockString(values, "resolved-ref"); ok && resolvedRef != "" {
		ensureDependencyAttribute(dependency, "source_ref", resolvedRef)
		ensureDependencyAttribute(dependency, "source_ref_kind", "revision")
	}
	if ref, ok := dartPubspecLockString(values, "ref"); ok && ref != "" {
		ensureDependencyAttribute(dependency, "source_requested_ref", ref)
	}
	return ""
}

func dartPubspecLockPath(dependency *DependencyReference, raw any) string {
	if sourcePath, ok := raw.(string); ok && strings.TrimSpace(sourcePath) != "" {
		ensureDependencyAttribute(dependency, "path", strings.TrimSpace(sourcePath))
		return ""
	}
	values, ok := anyStringMap(raw)
	if !ok {
		return fmt.Sprintf("packages.%s.description: expected path string or object", dependency.Name)
	}
	sourcePath, ok := dartPubspecLockString(values, "path")
	if !ok || sourcePath == "" {
		return fmt.Sprintf("packages.%s.description.path: path is required", dependency.Name)
	}
	ensureDependencyAttribute(dependency, "path", sourcePath)
	return ""
}

func dartPubspecLockSDK(dependency *DependencyReference, raw any) string {
	sdk, ok := raw.(string)
	if !ok || strings.TrimSpace(sdk) == "" {
		return fmt.Sprintf("packages.%s.description: SDK name is required", dependency.Name)
	}
	ensureDependencyAttribute(dependency, "sdk", strings.TrimSpace(sdk))
	return ""
}

func dartPubspecLockString(values map[string]any, key string) (string, bool) {
	value, exists := values[key]
	if !exists {
		return "", false
	}
	stringValue, ok := value.(string)
	return strings.TrimSpace(stringValue), ok
}
