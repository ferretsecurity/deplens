package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type crystalShardLockMatcherConfig struct{}

type crystalShardLockParser struct{}

func newCrystalShardLockParser(crystalShardLockMatcherConfig) (sourceAnalyzer, error) {
	return crystalShardLockParser{}, nil
}

func (crystalShardLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var lock struct {
		Version string         `yaml:"version"`
		Shards  map[string]any `yaml:"shards"`
	}
	if err := yaml.Unmarshal(content, &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Crystal Shard lockfile %q: %w", path, err)
	}
	if version := strings.TrimSpace(lock.Version); version != "1.0" && version != "2.0" {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(lock.Shards))
	incomplete := make([]string, 0)
	for _, name := range sortedAnyMapKeys(lock.Shards) {
		dependency, message := crystalShardLockDependency(name, lock.Shards[name])
		if message != "" {
			incomplete = append(incomplete, message)
		}
		if dependency.Name != "" {
			dependencies = append(dependencies, dependency)
		}
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func crystalShardLockDependency(declaredName string, raw any) (DependencyReference, string) {
	name := strings.TrimSpace(declaredName)
	if name == "" {
		return DependencyReference{}, "shards: dependency name must not be empty"
	}
	declaration, ok := anyStringMap(raw)
	if !ok {
		return DependencyReference{}, fmt.Sprintf("shards.%s: expected an object of dependency fields", name)
	}

	dependency := DependencyReference{
		Name:         name,
		SourceGroup:  "shards",
		Relationship: RelationshipInconclusive,
		Scope:        ScopeRuntime,
	}
	github := crystalShardString(declaration, "github")
	gitURL := crystalShardString(declaration, "git")
	sourcePath := crystalShardString(declaration, "path")
	switch {
	case github != "":
		dependency.OriginKind = OriginGit
		ensureDependencyAttribute(&dependency, "source_url", "github:"+github)
	case gitURL != "":
		dependency.OriginKind = OriginGit
		ensureDependencyAttribute(&dependency, "source_url", gitURL)
	case sourcePath != "":
		dependency.OriginKind = OriginPath
		ensureDependencyAttribute(&dependency, "path", sourcePath)
	default:
		return DependencyReference{}, fmt.Sprintf("shards.%s: expected github, git, or path source", name)
	}

	version := crystalShardString(declaration, "version")
	commit := crystalShardString(declaration, "commit")
	if version == "" {
		version = commit
	}
	if version == "" {
		return DependencyReference{}, fmt.Sprintf("shards.%s: expected resolved version or commit", name)
	}
	dependency.Version = version
	dependency.Raw = name + "@" + version
	if commit != "" {
		ensureDependencyAttribute(&dependency, "source_ref", commit)
		ensureDependencyAttribute(&dependency, "source_ref_kind", "commit")
	}
	return dependency, ""
}
