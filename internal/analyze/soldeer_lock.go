package analyze

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type soldeerLockParser struct{}

type soldeerLockFile struct {
	Dependencies []soldeerLockDependency `toml:"dependencies"`
}

type soldeerLockDependency struct {
	Name      string `toml:"name"`
	Version   string `toml:"version"`
	Source    string `toml:"source"`
	URL       string `toml:"url"`
	Git       string `toml:"git"`
	Rev       string `toml:"rev"`
	Checksum  string `toml:"checksum"`
	Integrity string `toml:"integrity"`
}

func newSoldeerLockParser(soldeerLockMatcherConfig) (sourceAnalyzer, error) {
	return soldeerLockParser{}, nil
}

func (soldeerLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var lock soldeerLockFile
	if err := toml.Unmarshal(content, &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Soldeer lockfile %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0, len(lock.Dependencies))
	incomplete := make([]string, 0)
	for index, entry := range lock.Dependencies {
		dependency, message := soldeerLockDependencyReference(entry, index)
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func soldeerLockDependencyReference(entry soldeerLockDependency, index int) (DependencyReference, string) {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d].name: required", index)
	}
	version := strings.TrimSpace(entry.Version)
	if version == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d].%s.version: required", index, name)
	}

	gitURL := strings.TrimSpace(entry.Git)
	archiveURL := strings.TrimSpace(entry.URL)
	if archiveURL == "" {
		archiveURL = strings.TrimSpace(entry.Source)
	}
	if gitURL == "" && archiveURL == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d].%s: expected git, url, or source", index, name)
	}

	attributes := make(map[string]string)
	dependency := DependencyReference{
		Raw:          name + "@" + version,
		Name:         name,
		Version:      version,
		SourceGroup:  "dependencies",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if gitURL != "" {
		dependency.OriginKind = OriginGit
		attributes["source_url"] = gitURL
		if revision := strings.TrimSpace(entry.Rev); revision != "" {
			attributes["source_ref"] = revision
			attributes["source_ref_kind"] = "revision"
		}
	} else {
		attributes["source_url"] = archiveURL
	}
	if checksum := strings.TrimSpace(entry.Checksum); checksum != "" {
		attributes["checksum"] = checksum
	}
	if integrity := strings.TrimSpace(entry.Integrity); integrity != "" {
		attributes["integrity"] = integrity
	}
	dependency.Attributes = attributes
	return dependency, ""
}
