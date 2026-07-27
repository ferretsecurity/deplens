package analyze

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type packageJSONParser struct{}

type packageJSONGroup struct {
	name  string
	scope DependencyScope
}

var packageJSONGroups = []packageJSONGroup{
	{name: "dependencies", scope: ScopeRuntime},
	{name: "devDependencies", scope: ScopeDevelopment},
	{name: "peerDependencies", scope: ScopeRuntime},
	{name: "optionalDependencies", scope: ScopeOptional},
}

func newPackageJSONParser(packageJSONMatcherConfig) (sourceAnalyzer, error) {
	return packageJSONParser{}, nil
}

func (packageJSONParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse package.json file %q: %w", path, err)
	}

	optionalPeers := packageJSONOptionalPeers(root["peerDependenciesMeta"])
	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)

	for _, group := range packageJSONGroups {
		raw, exists := root[group.name]
		if !exists || isJSONNull(raw) {
			continue
		}
		var entries map[string]json.RawMessage
		if err := json.Unmarshal(raw, &entries); err != nil {
			incomplete = append(incomplete, fmt.Sprintf("%s: expected an object of dependency specifiers", group.name))
			continue
		}
		if _, exists := entries[""]; exists {
			incomplete = append(incomplete, fmt.Sprintf("%s: dependency name must not be empty", group.name))
		}
		for _, name := range sortedStringKeys(entries) {
			var specifier string
			if err := json.Unmarshal(entries[name], &specifier); err != nil {
				incomplete = append(incomplete, fmt.Sprintf("%s.%s: expected a string dependency specifier", group.name, name))
				continue
			}
			scope := group.scope
			if group.name == "peerDependencies" && optionalPeers[name] {
				scope = ScopeOptional
			}
			dependencies = append(dependencies, packageJSONDependency(name, specifier, group.name, scope))
		}
	}

	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func packageJSONOptionalPeers(raw json.RawMessage) map[string]bool {
	result := make(map[string]bool)
	if len(raw) == 0 || isJSONNull(raw) {
		return result
	}
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil {
		return result
	}
	for name, value := range entries {
		var metadata struct {
			Optional bool `json:"optional"`
		}
		if json.Unmarshal(value, &metadata) == nil && metadata.Optional {
			result[name] = true
		}
	}
	return result
}

func packageJSONDependency(declaredName, specifier, group string, scope DependencyScope) DependencyReference {
	dependency := DependencyReference{
		Raw:               declaredName + "@" + specifier,
		Name:              declaredName,
		VersionConstraint: specifier,
		SourceGroup:       group,
		Relationship:      RelationshipDirect,
		Scope:             scope,
	}

	switch {
	case strings.HasPrefix(specifier, "npm:"):
		dependency.OriginKind = OriginRegistry
		targetName, constraint := parseNPMAlias(strings.TrimPrefix(specifier, "npm:"))
		if targetName != "" {
			dependency.Name = targetName
			dependency.VersionConstraint = constraint
		}
		dependency.Attributes = map[string]string{
			"declared_name": declaredName,
		}
	case strings.HasPrefix(specifier, "workspace:"):
		dependency.OriginKind = OriginWorkspace
		dependency.VersionConstraint = strings.TrimPrefix(specifier, "workspace:")
	case strings.HasPrefix(specifier, "file:"), strings.HasPrefix(specifier, "link:"):
		protocol, value, _ := strings.Cut(specifier, ":")
		dependency.OriginKind = OriginPath
		dependency.VersionConstraint = ""
		dependency.Attributes = map[string]string{"protocol": protocol, "path": value}
	case isLocalFilesystemPath(specifier),
		strings.HasPrefix(specifier, "~/"),
		strings.HasPrefix(specifier, ".\\"),
		strings.HasPrefix(specifier, "..\\"),
		strings.HasPrefix(specifier, "\\"),
		isWindowsAbsolutePath(specifier):
		dependency.OriginKind = OriginPath
		dependency.VersionConstraint = ""
		dependency.Attributes = map[string]string{"path": specifier}
	case isPackageJSONGitSpecifier(specifier), isPackageJSONHostedGitShorthand(specifier):
		dependency.OriginKind = OriginGit
		dependency.VersionConstraint = ""
		source, ref, _ := strings.Cut(specifier, "#")
		dependency.Attributes = map[string]string{"source_url": source}
		if ref != "" {
			dependency.Attributes["source_ref"] = ref
		}
	case strings.HasPrefix(specifier, "http://"), strings.HasPrefix(specifier, "https://"):
		dependency.OriginKind = OriginURL
		dependency.VersionConstraint = ""
		dependency.Attributes = map[string]string{"source_url": specifier}
	case isPackageJSONRegistrySpecifier(specifier):
		dependency.OriginKind = OriginRegistry
	default:
		dependency.VersionConstraint = ""
	}
	return dependency
}

func parseNPMAlias(value string) (string, string) {
	if value == "" {
		return "", ""
	}
	separator := strings.LastIndex(value, "@")
	if separator <= 0 {
		return value, ""
	}
	if strings.HasPrefix(value, "@") && separator < strings.Index(value, "/") {
		return value, ""
	}
	return value[:separator], value[separator+1:]
}

func isPackageJSONGitSpecifier(value string) bool {
	for _, prefix := range []string{
		"git:", "git+", "git@", "ssh:",
		"github:", "gitlab:", "bitbucket:",
	} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func isWindowsAbsolutePath(value string) bool {
	return len(value) >= 3 &&
		((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) &&
		value[1] == ':' && (value[2] == '/' || value[2] == '\\')
}

func isPackageJSONHostedGitShorthand(value string) bool {
	source, _, _ := strings.Cut(value, "#")
	if strings.HasPrefix(source, "@") || strings.ContainsAny(source, `\ :`) {
		return false
	}
	parts := strings.Split(source, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

var packageJSONRegistryTag = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func isPackageJSONRegistrySpecifier(value string) bool {
	if strings.ContainsAny(value, `:/\`) {
		return false
	}
	return dependencyVERS(PackageType("npm"), value) != "" || packageJSONRegistryTag.MatchString(value)
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}
