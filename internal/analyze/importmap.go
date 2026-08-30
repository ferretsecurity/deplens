package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
)

type importMapParserConfig struct{}

type importMapParser struct{}

func newImportMapParser(importMapParserConfig) (sourceAnalyzer, error) {
	return importMapParser{}, nil
}

func (importMapParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse import map %q: %w", path, err)
	}

	dependencies, incomplete := importMapGroupDependencies(root["imports"], "imports")
	scopedDependencies, scopedIncomplete := importMapScopeDependencies(root["scopes"])
	dependencies = append(dependencies, scopedDependencies...)
	incomplete = append(incomplete, scopedIncomplete...)
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func importMapScopeDependencies(raw json.RawMessage) ([]DependencyReference, []string) {
	if len(raw) == 0 || isJSONNull(raw) {
		return nil, nil
	}

	var scopes map[string]json.RawMessage
	if err := json.Unmarshal(raw, &scopes); err != nil {
		return nil, []string{"scopes: expected an object of scoped import mappings"}
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, scopePrefix := range sortedStringKeys(scopes) {
		group := "scopes." + scopePrefix
		scopeDependencies, scopeIncomplete := importMapGroupDependencies(scopes[scopePrefix], group)
		for i := range scopeDependencies {
			scopeDependencies[i].SourceGroup = "scopes"
			scopeDependencies[i].Relationship = RelationshipInconclusive
			if scopeDependencies[i].Attributes == nil {
				scopeDependencies[i].Attributes = make(map[string]string)
			}
			scopeDependencies[i].Attributes["scope_prefix"] = scopePrefix
		}
		dependencies = append(dependencies, scopeDependencies...)
		incomplete = append(incomplete, scopeIncomplete...)
	}

	return dependencies, incomplete
}

func importMapGroupDependencies(raw json.RawMessage, group string) ([]DependencyReference, []string) {
	if len(raw) == 0 || isJSONNull(raw) {
		return nil, nil
	}

	var imports map[string]json.RawMessage
	if err := json.Unmarshal(raw, &imports); err != nil {
		return nil, []string{fmt.Sprintf("%s: expected an object of import mappings", group)}
	}

	dependencies := make([]DependencyReference, 0, len(imports))
	incomplete := make([]string, 0)
	for _, declaredName := range sortedStringKeys(imports) {
		if declaredName == "" {
			incomplete = append(incomplete, fmt.Sprintf("%s: import specifier must not be empty", group))
			continue
		}
		if isJSONNull(imports[declaredName]) {
			continue
		}

		var target string
		if err := json.Unmarshal(imports[declaredName], &target); err != nil {
			incomplete = append(incomplete, fmt.Sprintf("%s.%s: expected a string import target", group, declaredName))
			continue
		}
		target = strings.TrimSpace(target)
		if target == "" {
			incomplete = append(incomplete, fmt.Sprintf("%s.%s: import target must not be empty", group, declaredName))
			continue
		}

		dependencies = append(dependencies, importMapDependency(declaredName, target, group))
	}

	sortDependencyReferences(dependencies)
	return dependencies, incomplete
}

func importMapDependency(declaredName, target, group string) DependencyReference {
	dependency := DependencyReference{
		Raw:          declaredName + "@" + target,
		Name:         declaredName,
		SourceGroup:  group,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}

	switch {
	case strings.HasPrefix(target, "npm:"):
		return importMapRegistryDependency(dependency, "npm", strings.TrimPrefix(target, "npm:"))
	case strings.HasPrefix(target, "jsr:"):
		return importMapRegistryDependency(dependency, "jsr", strings.TrimPrefix(target, "jsr:"))
	case strings.HasPrefix(target, "http://"), strings.HasPrefix(target, "https://"):
		dependency.OriginKind = OriginURL
		dependency.Attributes = map[string]string{"source_url": target}
	case strings.HasPrefix(target, "file:"):
		dependency.OriginKind = OriginPath
		dependency.Attributes = map[string]string{"path": strings.TrimPrefix(target, "file:")}
	case isLocalFilesystemPath(target), strings.HasPrefix(target, "~/"), isWindowsAbsolutePath(target):
		dependency.OriginKind = OriginPath
		dependency.Attributes = map[string]string{"path": target}
	}

	return dependency
}

func importMapRegistryDependency(dependency DependencyReference, registry, rawPackage string) DependencyReference {
	name, constraint := splitDenoJSONPackageSpecifier(rawPackage)
	if name == "" {
		return dependency
	}

	declaredName := dependency.Name
	dependency.Name = name
	dependency.VersionConstraint = constraint
	dependency.OriginKind = OriginRegistry
	dependency.Attributes = map[string]string{"registry": registry}
	if declaredName != name {
		dependency.Attributes["declared_name"] = declaredName
	}
	if registry == "npm" {
		dependency.PackageType = "npm"
	}
	return dependency
}
