package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type glideYAMLMatcherConfig struct{}

type glideYAMLParser struct{}

type glideYAMLFile struct {
	Package     string                `yaml:"package"`
	Imports     []glideYAMLDependency `yaml:"import"`
	TestImport  []glideYAMLDependency `yaml:"testImport"`
	TestImports []glideYAMLDependency `yaml:"testImports"`
}

type glideYAMLDependency struct {
	Package string `yaml:"package"`
	Version string `yaml:"version"`
	Repo    string `yaml:"repo"`
	VCS     string `yaml:"vcs"`
}

func newGlideYAMLParser(glideYAMLMatcherConfig) (sourceAnalyzer, error) {
	return glideYAMLParser{}, nil
}

func (glideYAMLParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var manifest glideYAMLFile
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Glide manifest %q: %w", path, err)
	}
	if strings.TrimSpace(manifest.Package) == "" {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(manifest.Imports)+len(manifest.TestImport)+len(manifest.TestImports))
	incomplete := make([]string, 0)
	dependencies, incomplete = appendGlideYAMLDependencies(dependencies, incomplete, "import", ScopeRuntime, manifest.Imports)
	dependencies, incomplete = appendGlideYAMLDependencies(dependencies, incomplete, "testImport", ScopeTest, manifest.TestImport)
	dependencies, incomplete = appendGlideYAMLDependencies(dependencies, incomplete, "testImports", ScopeTest, manifest.TestImports)
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func appendGlideYAMLDependencies(dependencies []DependencyReference, incomplete []string, group string, scope DependencyScope, entries []glideYAMLDependency) ([]DependencyReference, []string) {
	for index, entry := range entries {
		name := strings.TrimSpace(entry.Package)
		if name == "" {
			incomplete = append(incomplete, fmt.Sprintf("%s dependency %d requires package", group, index+1))
			continue
		}

		dependency := DependencyReference{
			Raw:          name,
			Name:         name,
			SourceGroup:  group,
			Relationship: RelationshipDirect,
			Scope:        scope,
		}
		if version := strings.TrimSpace(entry.Version); version != "" {
			dependency.Raw += "@" + version
			dependency.VersionConstraint = version
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
