package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type bufLockParser struct{}

type bufLockFile struct {
	Version string              `yaml:"version"`
	Deps    []bufLockDependency `yaml:"deps"`
}

type bufLockDependency struct {
	Remote     string `yaml:"remote"`
	Owner      string `yaml:"owner"`
	Repository string `yaml:"repository"`
	Name       string `yaml:"name"`
	Commit     string `yaml:"commit"`
	Digest     string `yaml:"digest"`
}

func newBufLockParser(bufLockMatcherConfig) (sourceAnalyzer, error) {
	return bufLockParser{}, nil
}

func (bufLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var file bufLockFile
	if err := yaml.Unmarshal(content, &file); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Buf lockfile %q: %w", path, err)
	}
	if file.Version != "v1" && file.Version != "v2" {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(file.Deps))
	incomplete := make([]string, 0)
	for index, entry := range file.Deps {
		name := bufLockDependencyName(file.Version, entry)
		if name == "" {
			incomplete = append(incomplete, fmt.Sprintf("Buf lockfile dependency %d has no module name", index+1))
			continue
		}

		commit := strings.TrimSpace(entry.Commit)
		if commit == "" {
			incomplete = append(incomplete, fmt.Sprintf("Buf lockfile dependency %s has no commit", name))
			continue
		}

		dependency := DependencyReference{
			Raw:          name + "@" + commit,
			Name:         name,
			Version:      commit,
			SourceGroup:  "deps",
			OriginKind:   OriginRegistry,
			Relationship: RelationshipDirect,
			Scope:        ScopeRuntime,
		}
		if digest := strings.TrimSpace(entry.Digest); digest != "" {
			dependency.Attributes = map[string]string{"digest": digest}
		}
		dependencies = append(dependencies, dependency)
	}
	sortDependencyReferences(dependencies)

	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func bufLockDependencyName(version string, entry bufLockDependency) string {
	if version == "v2" {
		return strings.TrimSpace(entry.Name)
	}

	parts := []string{
		strings.TrimSpace(entry.Remote),
		strings.TrimSpace(entry.Owner),
		strings.TrimSpace(entry.Repository),
	}
	for _, part := range parts {
		if part == "" {
			return ""
		}
	}
	return strings.Join(parts, "/")
}
