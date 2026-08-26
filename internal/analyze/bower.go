package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
)

type bowerParser struct{}

type bowerGroup struct {
	name  string
	scope DependencyScope
}

var bowerGroups = []bowerGroup{
	{name: "dependencies", scope: ScopeRuntime},
	{name: "devDependencies", scope: ScopeDevelopment},
}

func newBowerParser(bowerMatcherConfig) (sourceAnalyzer, error) {
	return bowerParser{}, nil
}

func (bowerParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse bower manifest %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, group := range bowerGroups {
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
			dependencies = append(dependencies, bowerDependency(name, specifier, group.name, group.scope))
		}
	}

	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func bowerDependency(name, specifier, group string, scope DependencyScope) DependencyReference {
	dependency := DependencyReference{
		Raw:               name + "@" + specifier,
		Name:              name,
		VersionConstraint: specifier,
		SourceGroup:       group,
		Relationship:      RelationshipDirect,
		Scope:             scope,
	}

	switch {
	case isBowerGitSpecifier(specifier), isPackageJSONHostedGitShorthand(specifier):
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
	case strings.HasPrefix(specifier, "file:"):
		dependency.OriginKind = OriginPath
		dependency.VersionConstraint = ""
		dependency.Attributes = map[string]string{"path": strings.TrimPrefix(specifier, "file:")}
	case isLocalFilesystemPath(specifier), strings.HasPrefix(specifier, "~/"),
		strings.HasPrefix(specifier, ".\\"), strings.HasPrefix(specifier, "..\\"),
		strings.HasPrefix(specifier, "\\"), isWindowsAbsolutePath(specifier):
		dependency.OriginKind = OriginPath
		dependency.VersionConstraint = ""
		dependency.Attributes = map[string]string{"path": specifier}
	default:
		dependency.OriginKind = OriginRegistry
	}
	return dependency
}

func isBowerGitSpecifier(value string) bool {
	for _, prefix := range []string{"git:", "git+", "git@", "git://", "ssh:", "github:", "gitlab:", "bitbucket:"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
