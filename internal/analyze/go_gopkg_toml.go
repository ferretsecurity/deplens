package analyze

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type gopkgTOMLMatcherConfig struct{}

type gopkgTOMLParser struct{}

type gopkgTOMLFile struct {
	Constraints []gopkgTOMLDependency `toml:"constraint"`
	Overrides   []gopkgTOMLDependency `toml:"override"`
}

type gopkgTOMLDependency struct {
	Name     string `toml:"name"`
	Version  string `toml:"version"`
	Branch   string `toml:"branch"`
	Revision string `toml:"revision"`
	Source   string `toml:"source"`
}

func newGopkgTOMLParser(gopkgTOMLMatcherConfig) (sourceAnalyzer, error) {
	return gopkgTOMLParser{}, nil
}

func (gopkgTOMLParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var manifest gopkgTOMLFile
	if _, err := toml.Decode(string(content), &manifest); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Gopkg manifest %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0, len(manifest.Constraints)+len(manifest.Overrides))
	incomplete := make([]string, 0)
	dependencies, incomplete = appendGopkgTOMLDependencies(dependencies, incomplete, "constraint", RelationshipDirect, manifest.Constraints)
	dependencies, incomplete = appendGopkgTOMLDependencies(dependencies, incomplete, "override", RelationshipInconclusive, manifest.Overrides)
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func appendGopkgTOMLDependencies(dependencies []DependencyReference, incomplete []string, group string, relationship Relationship, entries []gopkgTOMLDependency) ([]DependencyReference, []string) {
	for index, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			incomplete = append(incomplete, fmt.Sprintf("%s dependency %d requires a name", group, index+1))
			continue
		}

		dependency := DependencyReference{
			Raw:          name,
			Name:         name,
			SourceGroup:  group,
			Relationship: relationship,
			Scope:        ScopeRuntime,
		}
		if constraint := strings.TrimSpace(entry.Version); constraint != "" {
			dependency.Raw += "@" + constraint
			dependency.VersionConstraint = constraint
		}

		if branch := strings.TrimSpace(entry.Branch); branch != "" {
			dependency.Attributes = map[string]string{"source_branch": branch}
		}
		if revision := strings.TrimSpace(entry.Revision); revision != "" {
			if dependency.Attributes == nil {
				dependency.Attributes = make(map[string]string)
			}
			dependency.Attributes["source_revision"] = revision
		}
		if source := strings.TrimSpace(entry.Source); source != "" {
			if dependency.Attributes == nil {
				dependency.Attributes = make(map[string]string)
			}
			dependency.Attributes["source_url"] = source
		}

		dependencies = append(dependencies, dependency)
	}
	return dependencies, incomplete
}
