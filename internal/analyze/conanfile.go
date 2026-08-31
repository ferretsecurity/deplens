package analyze

import (
	"fmt"
	"strings"
)

type conanfileParser struct{}

type conanfileDependencyGroup struct {
	name  string
	scope DependencyScope
}

var conanfileDependencyGroups = map[string]conanfileDependencyGroup{
	"requires":        {name: "requires", scope: ScopeRuntime},
	"build_requires":  {name: "build_requires", scope: ScopeBuild},
	"tool_requires":   {name: "tool_requires", scope: ScopeBuild},
	"test_requires":   {name: "test_requires", scope: ScopeTest},
	"python_requires": {name: "python_requires", scope: ScopeDevelopment},
}

func newConanfileParser(conanfileMatcherConfig) (sourceAnalyzer, error) {
	return conanfileParser{}, nil
}

func (conanfileParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	currentGroup := conanfileDependencyGroup{}
	recognized := false

	for lineNumber, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if section, ok := conanfileSection(trimmed); ok {
			recognized = true
			currentGroup = conanfileDependencyGroups[section]
			continue
		}
		if currentGroup.name == "" {
			continue
		}

		dependency, ok := conanfileReference(trimmed)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("conanfile.txt %s reference on line %d is invalid", currentGroup.name, lineNumber+1))
			continue
		}
		dependency.SourceGroup = currentGroup.name
		dependency.Scope = currentGroup.scope
		dependencies = append(dependencies, dependency)
	}

	if !recognized {
		return sourceAnalyzerResult{}, nil
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func conanfileSection(line string) (string, bool) {
	if len(line) < 3 || line[0] != '[' || line[len(line)-1] != ']' {
		return "", false
	}
	section := strings.TrimSpace(line[1 : len(line)-1])
	if section == "" || strings.ContainsAny(section, " \t") {
		return "", false
	}
	return strings.ToLower(section), true
}

func conanfileReference(raw string) (DependencyReference, bool) {
	name, remainder, ok := strings.Cut(raw, "/")
	if !ok || name == "" || remainder == "" || strings.ContainsAny(name, " \t") {
		return DependencyReference{}, false
	}

	version, userChannel, hasUserChannel := strings.Cut(remainder, "@")
	version = strings.TrimSpace(version)
	if version == "" {
		return DependencyReference{}, false
	}

	dependency := DependencyReference{
		Raw:               raw,
		Name:              name,
		VersionConstraint: version,
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
	}
	if !hasUserChannel {
		return dependency, true
	}

	user, channel, ok := strings.Cut(userChannel, "/")
	if !ok || user == "" || channel == "" || strings.ContainsAny(userChannel, " \t") {
		return DependencyReference{}, false
	}
	dependency.Attributes = map[string]string{"user": user, "channel": channel}
	return dependency, true
}
