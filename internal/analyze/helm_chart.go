package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type helmChartParser struct{}

type helmChartFile struct {
	APIVersion   string                `yaml:"apiVersion"`
	Name         string                `yaml:"name"`
	Version      string                `yaml:"version"`
	Dependencies []helmChartDependency `yaml:"dependencies"`
}

type helmChartDependency struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	Repository string `yaml:"repository"`
}

func newHelmChartParser(helmChartMatcherConfig) (sourceAnalyzer, error) {
	return helmChartParser{}, nil
}

func (helmChartParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var chart helmChartFile
	if err := yaml.Unmarshal(content, &chart); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Helm chart %q: %w", path, err)
	}
	if strings.TrimSpace(chart.APIVersion) == "" || strings.TrimSpace(chart.Name) == "" || strings.TrimSpace(chart.Version) == "" {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(chart.Dependencies))
	incomplete := make([]string, 0)
	for index, entry := range chart.Dependencies {
		dependency, message := helmChartDependencyReference(entry, index)
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func helmChartDependencyReference(entry helmChartDependency, index int) (DependencyReference, string) {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d]: name is required", index)
	}
	version := strings.TrimSpace(entry.Version)
	if version == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d].%s: version is required", index, name)
	}

	dependency := DependencyReference{
		Raw:               name + "@" + version,
		Name:              name,
		VersionConstraint: version,
		SourceGroup:       "dependencies",
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             ScopeRuntime,
	}
	if repository := strings.TrimSpace(entry.Repository); repository != "" {
		if path, ok := strings.CutPrefix(repository, "file://"); ok {
			if path == "" {
				return DependencyReference{}, fmt.Sprintf("dependencies[%d].%s.repository: local path is required", index, name)
			}
			dependency.OriginKind = OriginPath
			dependency.Attributes = map[string]string{"source_path": path}
		} else {
			dependency.Attributes = map[string]string{"source_url": repository}
		}
	}
	return dependency, ""
}
