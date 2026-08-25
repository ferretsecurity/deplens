package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type haskellStackMatcherConfig struct{}

type haskellStackParser struct{}

type haskellStackFile struct {
	Resolver  string `yaml:"resolver"`
	ExtraDeps []any  `yaml:"extra-deps"`
}

func newHaskellStackParser(haskellStackMatcherConfig) (sourceAnalyzer, error) {
	return haskellStackParser{}, nil
}

func (haskellStackParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var manifest haskellStackFile
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Stack manifest %q: %w", path, err)
	}
	if strings.TrimSpace(manifest.Resolver) == "" {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(manifest.ExtraDeps))
	incomplete := make([]string, 0)
	for index, value := range manifest.ExtraDeps {
		dependency, message := haskellStackExtraDependency(value)
		if message != "" {
			incomplete = append(incomplete, fmt.Sprintf("extra-deps dependency %d: %s", index+1, message))
			continue
		}
		dependencies = append(dependencies, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func haskellStackExtraDependency(value any) (DependencyReference, string) {
	switch declaration := value.(type) {
	case string:
		return haskellStackHackageDependency(declaration)
	case map[string]any:
		return haskellStackGitDependency(declaration)
	default:
		return DependencyReference{}, "expected a package version or Git dependency mapping"
	}
}

func haskellStackHackageDependency(value string) (DependencyReference, string) {
	value = strings.TrimSpace(value)
	separator := -1
	for index := len(value) - 1; index >= 0; index-- {
		if value[index] == '-' && index+1 < len(value) && value[index+1] >= '0' && value[index+1] <= '9' {
			separator = index
			break
		}
	}
	if separator < 1 {
		return DependencyReference{}, fmt.Sprintf("invalid package version declaration %q", value)
	}

	name, version := value[:separator], value[separator+1:]
	if !isHaskellCabalPackageName(name) || !isHaskellStackVersion(version) {
		return DependencyReference{}, fmt.Sprintf("invalid package version declaration %q", value)
	}
	return DependencyReference{
		Raw:               name + "@" + version,
		Name:              name,
		VersionConstraint: version,
		SourceGroup:       "extra-deps",
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             ScopeRuntime,
	}, ""
}

func isHaskellStackVersion(value string) bool {
	for _, part := range strings.Split(value, ".") {
		if part == "" {
			return false
		}
		for _, character := range part {
			if character < '0' || character > '9' {
				return false
			}
		}
	}
	return true
}

func haskellStackGitDependency(values map[string]any) (DependencyReference, string) {
	gitURL, ok := values["git"].(string)
	gitURL = strings.TrimSpace(gitURL)
	if !ok || gitURL == "" {
		return DependencyReference{}, "git dependency requires a non-empty git URL"
	}

	name := haskellStackGitDependencyName(gitURL)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("cannot derive package name from Git URL %q", gitURL)
	}
	attributes := map[string]string{"source_url": gitURL}
	if commit, ok := values["commit"].(string); ok && strings.TrimSpace(commit) != "" {
		attributes["source_ref"] = strings.TrimSpace(commit)
		attributes["source_ref_kind"] = "commit"
	}
	return DependencyReference{
		PackageType:  "generic",
		Raw:          name + "@" + gitURL,
		Name:         name,
		SourceGroup:  "extra-deps",
		OriginKind:   OriginGit,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
		Attributes:   attributes,
	}, ""
}

func haskellStackGitDependencyName(gitURL string) string {
	path := strings.TrimSuffix(strings.TrimRight(gitURL, "/"), ".git")
	if index := strings.LastIndexAny(path, "/:"); index >= 0 {
		path = path[index+1:]
	}
	if isHaskellCabalPackageName(path) {
		return path
	}
	return ""
}
