package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type haskellStackLockMatcherConfig struct{}

type haskellStackLockParser struct{}

type haskellStackLockFile struct {
	Packages *[]haskellStackLockPackage `yaml:"packages"`
}

type haskellStackLockPackage struct {
	Completed haskellStackLockPackageSource `yaml:"completed"`
	Original  haskellStackLockPackageSource `yaml:"original"`
}

type haskellStackLockPackageSource struct {
	Hackage string `yaml:"hackage"`
	URL     string `yaml:"url"`
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

func newHaskellStackLockParser(haskellStackLockMatcherConfig) (sourceAnalyzer, error) {
	return haskellStackLockParser{}, nil
}

func (haskellStackLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var lock haskellStackLockFile
	if err := yaml.Unmarshal(content, &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Stack lockfile %q: %w", path, err)
	}
	if lock.Packages == nil {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(*lock.Packages))
	incomplete := make([]string, 0)
	for index, entry := range *lock.Packages {
		dependency, message := haskellStackLockDependency(entry)
		if message != "" {
			incomplete = append(incomplete, fmt.Sprintf("packages dependency %d: %s", index+1, message))
			continue
		}
		dependencies = append(dependencies, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func haskellStackLockDependency(entry haskellStackLockPackage) (DependencyReference, string) {
	if hackage := strings.TrimSpace(entry.Original.Hackage); hackage != "" {
		return haskellStackLockHackageDependency(hackage)
	}

	url := strings.TrimSpace(entry.Original.URL)
	if url == "" {
		return DependencyReference{}, "expected original hackage package or URL"
	}

	name := strings.TrimSpace(entry.Completed.Name)
	version := strings.TrimSpace(entry.Completed.Version)
	if !isHaskellCabalPackageName(name) || !isHaskellStackVersion(version) {
		return DependencyReference{}, "URL package requires a valid completed name and version"
	}
	return DependencyReference{
		PackageType:  "generic",
		Raw:          name + "@" + version,
		Name:         name,
		Version:      version,
		SourceGroup:  "packages",
		OriginKind:   OriginURL,
		Relationship: RelationshipInconclusive,
		Scope:        ScopeRuntime,
		Attributes:   map[string]string{"source_url": url},
	}, ""
}

func haskellStackLockHackageDependency(value string) (DependencyReference, string) {
	dependency, message := haskellStackHackageDependency(value)
	if message != "" {
		return DependencyReference{}, message
	}
	dependency.Version = dependency.VersionConstraint
	dependency.VersionConstraint = ""
	dependency.Relationship = RelationshipInconclusive
	dependency.SourceGroup = "packages"
	return dependency, ""
}
