package analyze

import (
	"regexp"
	"strings"
)

type scalaMillParser struct{}

var (
	scalaMillVersionDeclaration = regexp.MustCompile(`(?m)\bval\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*"([^"\r\n]*)"`)
	scalaMillIvyDeclaration     = regexp.MustCompile(`\bivy"([^"\r\n]+)"`)
	scalaMillIvyImport          = regexp.MustCompile("(?m)\\bimport\\s+\\$ivy\\.\\`([^\\`]*)\\`")
	scalaMillIvyDepsDefinition  = regexp.MustCompile(`\b(?:override\s+)?def\s+([A-Za-z_][A-Za-z0-9_]*vyDeps)\b`)
	scalaMillInterpolation      = regexp.MustCompile(`\$\{([^{}]+)\}`)
)

func newScalaMillParser(scalaMillMatcherConfig) (sourceAnalyzer, error) {
	return scalaMillParser{}, nil
}

func (scalaMillParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	cleaned, err := stripExecutableDSLComments(path, string(content))
	if err != nil {
		return sourceAnalyzerResult{}, err
	}

	versions := scalaMillVersions(cleaned)
	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})
	for _, match := range scalaMillIvyImport.FindAllStringSubmatchIndex(cleaned, -1) {
		dependency, ok := scalaMillCoordinateDependency(cleaned[match[2]:match[3]], versions, "ivy-imports", ScopeBuild)
		if ok {
			key := dependency.SourceGroup + "\x00" + string(dependency.Scope) + "\x00" + dependency.Raw
			dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
		}
	}
	for _, match := range scalaMillIvyDeclaration.FindAllStringSubmatchIndex(cleaned, -1) {
		position := match[0]
		group := scalaMillDeclarationGroup(cleaned, position)
		dependency, ok := scalaMillCoordinateDependency(cleaned[match[2]:match[3]], versions, group, scalaMillDeclarationScope(cleaned, position, group))
		if ok {
			key := dependency.SourceGroup + "\x00" + string(dependency.Scope) + "\x00" + dependency.Raw
			dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func scalaMillVersions(content string) map[string]string {
	versions := make(map[string]string)
	for _, match := range scalaMillVersionDeclaration.FindAllStringSubmatch(content, -1) {
		versions[match[1]] = match[2]
		versions["versions."+match[1]] = match[2]
	}
	return versions
}

func scalaMillCoordinateDependency(coordinate string, versions map[string]string, group string, scope DependencyScope) (DependencyReference, bool) {
	coordinate = strings.TrimSpace(coordinate)
	versionIndex := strings.LastIndex(coordinate, ":")
	if versionIndex < 1 || versionIndex == len(coordinate)-1 {
		return DependencyReference{}, false
	}
	prefix := strings.TrimRight(coordinate[:versionIndex], ":")
	parts := strings.FieldsFunc(prefix, func(value rune) bool { return value == ':' })
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return DependencyReference{}, false
	}
	version := scalaMillResolveVersion(coordinate[versionIndex+1:], versions)
	return DependencyReference{
		Raw:               coordinate,
		Name:              parts[0] + ":" + parts[1],
		VersionConstraint: normalizeMavenManifestConstraint(version),
		SourceGroup:       group,
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             scope,
	}, true
}

func scalaMillResolveVersion(value string, versions map[string]string) string {
	return scalaMillInterpolation.ReplaceAllStringFunc(value, func(expression string) string {
		match := scalaMillInterpolation.FindStringSubmatch(expression)
		if len(match) == 2 {
			if resolved, ok := versions[strings.TrimSpace(match[1])]; ok {
				return resolved
			}
		}
		return expression
	})
}

func scalaMillDeclarationGroup(content string, position int) string {
	matches := scalaMillIvyDepsDefinition.FindAllStringSubmatch(content[:position], -1)
	if len(matches) == 0 {
		return "ivyDeps"
	}
	return matches[len(matches)-1][1]
}

func scalaMillDeclarationScope(content string, position int, group string) DependencyScope {
	if scalaMillInsideTestModule(content, position) {
		return ScopeTest
	}
	if strings.Contains(strings.ToLower(group), "plugin") {
		return ScopeBuild
	}
	return ScopeRuntime
}

func scalaMillInsideTestModule(content string, position int) bool {
	stack := make([]bool, 0)
	lineStart := 0
	var quote byte
	escaped := false
	for index := 0; index < position; index++ {
		current := content[index]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == quote {
				quote = 0
			}
			continue
		}
		if current == '\'' || current == '"' {
			quote = current
			continue
		}
		if current == '\n' {
			lineStart = index + 1
			continue
		}
		switch current {
		case '{':
			header := strings.ToLower(content[lineStart:index])
			stack = append(stack, strings.Contains(header, "object test") || strings.Contains(header, "commontestmodule"))
		case '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	for _, isTest := range stack {
		if isTest {
			return true
		}
	}
	return false
}
