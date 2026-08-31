package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type condaEnvironmentParser struct{}

func newCondaEnvironmentParser(struct{}) (sourceAnalyzer, error) {
	return condaEnvironmentParser{}, nil
}

func (condaEnvironmentParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var manifest map[string]any
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Conda environment %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	entries, exists := manifest["dependencies"]
	if !exists || entries == nil {
		return semanticAnalyzerResult(dependencies, incomplete), nil
	}

	values, ok := entries.([]any)
	if !ok {
		return semanticAnalyzerResult(dependencies, []string{"dependencies: expected a list of package declarations"}), nil
	}
	for index, entry := range values {
		switch value := entry.(type) {
		case string:
			dependency, message := condaEnvironmentDependency(value)
			if message != "" {
				incomplete = append(incomplete, fmt.Sprintf("dependencies[%d]: %s", index, message))
				continue
			}
			dependencies = append(dependencies, dependency)
		case map[string]any:
			pipDependencies, message := condaEnvironmentPipDependencies(value)
			if message != "" {
				incomplete = append(incomplete, fmt.Sprintf("dependencies[%d]: %s", index, message))
				continue
			}
			dependencies = append(dependencies, pipDependencies...)
		default:
			incomplete = append(incomplete, fmt.Sprintf("dependencies[%d]: expected a package string or pip mapping", index))
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func condaEnvironmentDependency(raw string) (DependencyReference, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DependencyReference{}, "package declaration must not be empty"
	}

	nameAndConstraint := raw
	attributes := map[string]string(nil)
	if channel, remainder, found := strings.Cut(raw, "::"); found {
		channel = strings.TrimSpace(channel)
		nameAndConstraint = strings.TrimSpace(remainder)
		if channel == "" || nameAndConstraint == "" {
			return DependencyReference{}, "invalid channel-qualified package declaration"
		}
		attributes = map[string]string{"channel": channel}
	}

	nameEnd := strings.IndexAny(nameAndConstraint, "=<>!")
	if nameEnd < 0 {
		nameEnd = strings.IndexAny(nameAndConstraint, " \t")
	}
	name := nameAndConstraint
	constraint := ""
	if nameEnd >= 0 {
		name = strings.TrimSpace(nameAndConstraint[:nameEnd])
		constraint = strings.TrimSpace(nameAndConstraint[nameEnd:])
	}
	if name == "" {
		return DependencyReference{}, "package name is required"
	}

	return DependencyReference{
		Raw:               raw,
		Name:              name,
		VersionConstraint: constraint,
		SourceGroup:       "dependencies",
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             ScopeRuntime,
		Attributes:        attributes,
	}, ""
}

func condaEnvironmentPipDependencies(value map[string]any) ([]DependencyReference, string) {
	pip, exists := value["pip"]
	if !exists || len(value) != 1 {
		return nil, "expected a mapping with only a pip dependency list"
	}
	values, ok := pip.([]any)
	if !ok {
		return nil, "pip: expected a list of package declarations"
	}

	dependencies := make([]DependencyReference, 0, len(values))
	for index, rawValue := range values {
		raw, ok := rawValue.(string)
		raw = strings.TrimSpace(raw)
		if !ok || raw == "" {
			return nil, fmt.Sprintf("pip[%d]: package declaration must be a non-empty string", index)
		}
		if target, included := parsePyRequirementsInclude(raw); included {
			dependencies = append(dependencies, condaEnvironmentPipRequirementsInclude(raw, target))
			continue
		}
		parsed := parsePEP508Dep(raw)
		if parsed.name == "" {
			return nil, fmt.Sprintf("pip[%d]: package name is required", index)
		}
		dependencies = append(dependencies, DependencyReference{
			PackageType:       "pypi",
			Raw:               raw,
			Name:              parsed.name,
			VersionConstraint: parsed.versionConstraint,
			SourceGroup:       "dependencies.pip",
			OriginKind:        OriginRegistry,
			Relationship:      RelationshipDirect,
			Scope:             ScopeRuntime,
			Attributes:        parsed.attributes,
		})
	}
	return dependencies, ""
}

func condaEnvironmentPipRequirementsInclude(raw, target string) DependencyReference {
	path := strings.TrimPrefix(target, "file:")
	return DependencyReference{
		PackageType:  "generic",
		Raw:          raw,
		Name:         path,
		SourceGroup:  "dependencies.pip",
		OriginKind:   OriginPath,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
		Attributes:   map[string]string{"path": path},
	}
}
