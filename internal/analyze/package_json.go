package analyze

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type sourceFact interface {
	sourceFact()
}

type javascriptProjectFact struct {
	hasDependencies bool
	manager         string
	managerInvalid  bool
	workspaces      []string
}

func (javascriptProjectFact) sourceFact() {}

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
	hasDependencyLikeContent := false

	for _, group := range packageJSONGroups {
		raw, exists := root[group.name]
		if !exists || isJSONNull(raw) {
			continue
		}
		var entries map[string]json.RawMessage
		if err := json.Unmarshal(raw, &entries); err != nil {
			hasDependencyLikeContent = true
			incomplete = append(incomplete, fmt.Sprintf("%s: expected an object of dependency specifiers", group.name))
			continue
		}
		names := make([]string, 0, len(entries))
		for name := range entries {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			hasDependencyLikeContent = true
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

	fact := packageJSONProjectFact(root, hasDependencyLikeContent)
	result := sourceAnalyzerResult{
		Recognized:   true,
		Dependencies: dependencies,
		Facts:        []sourceFact{fact},
	}
	switch {
	case len(dependencies) > 0 && len(incomplete) > 0:
		result.Analysis = SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial}
	case len(dependencies) > 0:
		result.Analysis = SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}
	case len(incomplete) > 0:
		result.Analysis = SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionUnsupported}
	default:
		result.Analysis = SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}
	}
	result.Diagnostics = diagnosticsFromMessages(DiagnosticWarning, incompleteExtractionCode, incomplete)
	return result, nil
}

func packageJSONProjectFact(root map[string]json.RawMessage, hasDependencies bool) javascriptProjectFact {
	fact := javascriptProjectFact{
		hasDependencies: hasDependencies,
		workspaces:      decodeJavaScriptWorkspaces(root["workspaces"]),
	}
	if raw, ok := root["packageManager"]; ok && !isJSONNull(raw) {
		var manager string
		if err := json.Unmarshal(raw, &manager); err != nil {
			fact.managerInvalid = true
		} else if manager != "" {
			fact.manager = normalizeJavaScriptManager(manager)
			fact.managerInvalid = fact.manager == ""
		}
	}
	return fact
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
		PackageType:       PackageType("npm"),
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
			"specifier":     specifier,
		}
	case strings.HasPrefix(specifier, "workspace:"):
		dependency.OriginKind = OriginWorkspace
		dependency.VersionConstraint = strings.TrimPrefix(specifier, "workspace:")
		dependency.Attributes = map[string]string{"specifier": specifier}
	case strings.HasPrefix(specifier, "file:"), strings.HasPrefix(specifier, "link:"):
		protocol, value, _ := strings.Cut(specifier, ":")
		dependency.OriginKind = OriginPath
		dependency.VersionConstraint = ""
		dependency.Attributes = map[string]string{"protocol": protocol, "path": value}
	case isPackageJSONGitSpecifier(specifier):
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
	case strings.Contains(specifier, ":"):
		dependency.VersionConstraint = ""
		dependency.Attributes = map[string]string{"specifier": specifier}
	default:
		dependency.OriginKind = OriginRegistry
	}
	return dependency
}

func parseNPMAlias(value string) (string, string) {
	if value == "" {
		return "", ""
	}
	if strings.HasPrefix(value, "@") {
		slash := strings.Index(value, "/")
		if slash < 0 {
			return value, ""
		}
		if separator := strings.LastIndex(value, "@"); separator > slash {
			return value[:separator], value[separator+1:]
		}
		return value, ""
	}
	if separator := strings.LastIndex(value, "@"); separator > 0 {
		return value[:separator], value[separator+1:]
	}
	return value, ""
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

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}
