package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type glideLockMatcherConfig struct{}

type glideLockParser struct{}

type glideLockFile struct {
	Hash        string                `yaml:"hash"`
	Imports     []glideLockDependency `yaml:"imports"`
	TestImports []glideLockDependency `yaml:"testImports"`
}

type glideLockDependency struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Repo    string `yaml:"repo"`
	VCS     string `yaml:"vcs"`
}

func newGlideLockParser(glideLockMatcherConfig) (sourceAnalyzer, error) {
	return glideLockParser{}, nil
}

func (glideLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var lock glideLockFile
	if err := yaml.Unmarshal(content, &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Glide lockfile %q: %w", path, err)
	}
	if strings.TrimSpace(lock.Hash) == "" {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(lock.Imports)+len(lock.TestImports))
	incomplete := make([]string, 0)
	dependencies, incomplete = appendGlideLockDependencies(dependencies, incomplete, "imports", ScopeRuntime, lock.Imports)
	dependencies, incomplete = appendGlideLockDependencies(dependencies, incomplete, "testImports", ScopeTest, lock.TestImports)
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func appendGlideLockDependencies(dependencies []DependencyReference, incomplete []string, group string, scope DependencyScope, entries []glideLockDependency) ([]DependencyReference, []string) {
	for index, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		version := strings.TrimSpace(entry.Version)
		if name == "" || version == "" {
			incomplete = append(incomplete, fmt.Sprintf("%s dependency %d requires name and version", group, index+1))
			continue
		}

		dependency := DependencyReference{
			Raw:          name + "@" + version,
			Name:         name,
			Version:      version,
			SourceGroup:  group,
			Relationship: RelationshipInconclusive,
			Scope:        scope,
		}
		repo := strings.TrimSpace(entry.Repo)
		vcs := strings.TrimSpace(entry.VCS)
		if repo != "" || vcs != "" {
			dependency.Attributes = make(map[string]string)
			if repo != "" {
				dependency.Attributes["source_url"] = repo
			}
			if vcs != "" {
				dependency.Attributes["vcs"] = vcs
				if vcs == "git" {
					dependency.OriginKind = OriginGit
				}
			}
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies, incomplete
}
