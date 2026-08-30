package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type helmChartLockParser struct{}

type helmChartLockFile struct {
	Dependencies []helmChartLockDependency `yaml:"dependencies"`
	Digest       string                    `yaml:"digest"`
	Generated    string                    `yaml:"generated"`
}

type helmChartLockDependency struct {
	Name       string `yaml:"name"`
	Repository string `yaml:"repository"`
	Version    string `yaml:"version"`
}

func newHelmChartLockParser(helmChartLockMatcherConfig) (sourceAnalyzer, error) {
	return helmChartLockParser{}, nil
}

func (helmChartLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var lock helmChartLockFile
	if err := yaml.Unmarshal(content, &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Helm chart lockfile %q: %w", path, err)
	}
	if strings.TrimSpace(lock.Digest) == "" || strings.TrimSpace(lock.Generated) == "" {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(lock.Dependencies))
	incomplete := make([]string, 0)
	for index, entry := range lock.Dependencies {
		dependency, message := helmChartLockDependencyReference(entry, index)
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func helmChartLockDependencyReference(entry helmChartLockDependency, index int) (DependencyReference, string) {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d]: name is required", index)
	}
	version := strings.TrimSpace(entry.Version)
	if version == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d].%s: version is required", index, name)
	}
	repository := strings.TrimSpace(entry.Repository)
	if repository == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d].%s: repository is required", index, name)
	}

	dependency := DependencyReference{
		Raw:          name + "@" + version,
		Name:         name,
		Version:      version,
		SourceGroup:  "dependencies",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if sourcePath, ok := strings.CutPrefix(repository, "file://"); ok {
		if sourcePath == "" {
			return DependencyReference{}, fmt.Sprintf("dependencies[%d].%s.repository: local path is required", index, name)
		}
		dependency.OriginKind = OriginPath
		dependency.Attributes = map[string]string{"source_path": sourcePath}
	} else {
		dependency.Attributes = map[string]string{"source_url": repository}
	}
	return dependency, ""
}
