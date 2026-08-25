package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
)

type denoJSONParserConfig struct{}

type denoJSONParser struct{}

func newDenoJSONParser(denoJSONParserConfig) (sourceAnalyzer, error) {
	return denoJSONParser{}, nil
}

func (denoJSONParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Deno configuration %q: %w", path, err)
	}

	rawImports, exists := root["imports"]
	if !exists || isJSONNull(rawImports) {
		return semanticAnalyzerResult(nil, nil), nil
	}

	var imports map[string]json.RawMessage
	if err := json.Unmarshal(rawImports, &imports); err != nil {
		return semanticAnalyzerResult(nil, []string{"imports: expected an object of import mappings"}), nil
	}

	dependencies := make([]DependencyReference, 0, len(imports))
	incomplete := make([]string, 0)
	for _, declaredName := range sortedStringKeys(imports) {
		if declaredName == "" {
			incomplete = append(incomplete, "imports: import specifier must not be empty")
			continue
		}

		var target string
		if err := json.Unmarshal(imports[declaredName], &target); err != nil {
			incomplete = append(incomplete, fmt.Sprintf("imports.%s: expected a string import target", declaredName))
			continue
		}
		target = strings.TrimSpace(target)
		if target == "" {
			incomplete = append(incomplete, fmt.Sprintf("imports.%s: import target must not be empty", declaredName))
			continue
		}

		dependency, message := denoJSONImportDependency(declaredName, target)
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func denoJSONImportDependency(declaredName, target string) (DependencyReference, string) {
	dependency := DependencyReference{
		Raw:          declaredName + "@" + target,
		Name:         declaredName,
		SourceGroup:  "imports",
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}

	switch {
	case strings.HasPrefix(target, "npm:"):
		return denoJSONRegistryDependency(dependency, "npm", strings.TrimPrefix(target, "npm:"))
	case strings.HasPrefix(target, "jsr:"):
		return denoJSONRegistryDependency(dependency, "jsr", strings.TrimPrefix(target, "jsr:"))
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

	return dependency, ""
}

func denoJSONRegistryDependency(dependency DependencyReference, registry, rawPackage string) (DependencyReference, string) {
	name, constraint := splitDenoJSONPackageSpecifier(rawPackage)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("imports.%s: %s package name is required", dependency.Name, registry)
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
	return dependency, ""
}

func splitDenoJSONPackageSpecifier(value string) (string, string) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "/")
	if value == "" {
		return "", ""
	}

	nameEnd := 0
	if strings.HasPrefix(value, "@") {
		scopeEnd := strings.Index(value, "/")
		if scopeEnd <= 1 {
			return "", ""
		}
		packageEnd := strings.IndexAny(value[scopeEnd+1:], "@/")
		if packageEnd == -1 {
			return value, ""
		}
		nameEnd = scopeEnd + 1 + packageEnd
	} else {
		nameEnd = strings.IndexAny(value, "@/")
		if nameEnd == -1 {
			return value, ""
		}
	}

	name := value[:nameEnd]
	if value[nameEnd] != '@' {
		return name, ""
	}
	constraint, _, _ := strings.Cut(value[nameEnd+1:], "/")
	return name, constraint
}
