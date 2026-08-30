package analyze

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type gopkgLockMatcherConfig struct{}

type gopkgLockParser struct{}

type gopkgLockFile struct {
	Projects []gopkgLockProject `toml:"projects"`
}

type gopkgLockProject struct {
	Name     string `toml:"name"`
	Revision string `toml:"revision"`
	Version  string `toml:"version"`
	Branch   string `toml:"branch"`
}

func newGopkgLockParser(gopkgLockMatcherConfig) (sourceAnalyzer, error) {
	return gopkgLockParser{}, nil
}

func (gopkgLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var lock gopkgLockFile
	if _, err := toml.Decode(string(content), &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Gopkg lockfile %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0, len(lock.Projects))
	incomplete := make([]string, 0)
	for index, project := range lock.Projects {
		name := strings.TrimSpace(project.Name)
		if name == "" {
			incomplete = append(incomplete, fmt.Sprintf("projects dependency %d requires a name", index+1))
			continue
		}

		version := strings.TrimSpace(project.Revision)
		dependency := DependencyReference{
			Raw:          name,
			Name:         name,
			SourceGroup:  "projects",
			Relationship: RelationshipInconclusive,
			Scope:        ScopeRuntime,
		}
		if version != "" {
			dependency.Raw = name + "@" + version
			dependency.Version = version
		}

		if tag := strings.TrimSpace(project.Version); tag != "" {
			dependency.Attributes = map[string]string{"source_tag": tag}
		}
		if branch := strings.TrimSpace(project.Branch); branch != "" {
			if dependency.Attributes == nil {
				dependency.Attributes = make(map[string]string)
			}
			dependency.Attributes["source_branch"] = branch
		}
		dependencies = append(dependencies, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}
