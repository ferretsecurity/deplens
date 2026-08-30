package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
)

type vcpkgParser struct{}

func newVcpkgParser(vcpkgMatcherConfig) (sourceAnalyzer, error) {
	return vcpkgParser{}, nil
}

func (vcpkgParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse vcpkg manifest %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	dependencies, incomplete = appendVcpkgDependencyGroup(dependencies, incomplete, root["dependencies"], "dependencies", ScopeRuntime)

	var features map[string]json.RawMessage
	if rawFeatures, exists := root["features"]; exists && !isJSONNull(rawFeatures) {
		if err := json.Unmarshal(rawFeatures, &features); err != nil {
			incomplete = append(incomplete, "features: expected an object")
		}
	}
	for _, featureName := range sortedStringKeys(features) {
		var feature map[string]json.RawMessage
		if err := json.Unmarshal(features[featureName], &feature); err != nil {
			incomplete = append(incomplete, fmt.Sprintf("features.%s: expected an object", featureName))
			continue
		}
		group := "features." + featureName + ".dependencies"
		dependencies, incomplete = appendVcpkgDependencyGroup(dependencies, incomplete, feature["dependencies"], group, ScopeOptional)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func appendVcpkgDependencyGroup(dependencies []DependencyReference, incomplete []string, raw json.RawMessage, group string, scope DependencyScope) ([]DependencyReference, []string) {
	if len(raw) == 0 || isJSONNull(raw) {
		return dependencies, incomplete
	}

	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return dependencies, append(incomplete, fmt.Sprintf("%s: expected an array", group))
	}
	for _, entry := range entries {
		dependency, message := vcpkgDependency(entry, group, scope)
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies, incomplete
}

func vcpkgDependency(raw json.RawMessage, group string, scope DependencyScope) (DependencyReference, string) {
	var name string
	if err := json.Unmarshal(raw, &name); err == nil {
		return newVcpkgDependency(name, group, scope, false)
	}

	var declaration struct {
		Name string `json:"name"`
		Host bool   `json:"host"`
	}
	if err := json.Unmarshal(raw, &declaration); err != nil {
		return DependencyReference{}, fmt.Sprintf("%s: dependency must be a string or object", group)
	}
	return newVcpkgDependency(declaration.Name, group, scope, declaration.Host)
}

func newVcpkgDependency(rawName, group string, scope DependencyScope, host bool) (DependencyReference, string) {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("%s: dependency name is required", group)
	}
	if host {
		scope = ScopeBuild
	}
	return DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        scope,
	}, ""
}
