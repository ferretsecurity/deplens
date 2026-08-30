package analyze

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type foundryTOMLParser struct{}

func newFoundryTOMLParser(foundryTOMLMatcherConfig) (sourceAnalyzer, error) {
	return foundryTOMLParser{}, nil
}

func (foundryTOMLParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var config map[string]any
	if err := toml.Unmarshal(content, &config); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Foundry configuration %q: %w", path, err)
	}

	rawDependencies, exists := config["dependencies"]
	if !exists || rawDependencies == nil {
		return semanticAnalyzerResult([]DependencyReference{}, nil), nil
	}
	values, ok := anyStringMap(rawDependencies)
	if !ok {
		return semanticAnalyzerResult(nil, []string{"dependencies: expected a table of package declarations"}), nil
	}

	dependencies := make([]DependencyReference, 0, len(values))
	incomplete := make([]string, 0)
	for _, declaredName := range sortedAnyMapKeys(values) {
		dependency, message := foundryTOMLDependency(declaredName, values[declaredName])
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func foundryTOMLDependency(declaredName string, rawValue any) (DependencyReference, string) {
	name := strings.TrimSpace(declaredName)
	if name == "" {
		return DependencyReference{}, "dependencies: package name must not be empty"
	}
	constraint, ok := rawValue.(string)
	if !ok {
		return DependencyReference{}, fmt.Sprintf("dependencies.%s: version constraint must be a string", name)
	}
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies.%s: version constraint must not be empty", name)
	}
	return DependencyReference{
		Raw:               name + "@" + constraint,
		Name:              name,
		VersionConstraint: constraint,
		SourceGroup:       "dependencies",
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             ScopeRuntime,
	}, ""
}
