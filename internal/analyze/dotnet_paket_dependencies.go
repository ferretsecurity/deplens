package analyze

import (
	"path"
	"strings"
)

type dotnetPaketDependenciesMatcherConfig struct{}

type dotnetPaketDependenciesParser struct{}

func newDotnetPaketDependenciesParser(dotnetPaketDependenciesMatcherConfig) (sourceAnalyzer, error) {
	return dotnetPaketDependenciesParser{}, nil
}

func (dotnetPaketDependenciesParser) Analyze(_ string, content []byte) (sourceAnalyzerResult, error) {
	dependencies := make([]DependencyReference, 0)
	group := "default"
	groupSource := ""

	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "//") || strings.HasPrefix(fields[0], "#") {
			continue
		}

		switch strings.ToLower(fields[0]) {
		case "group":
			if len(fields) >= 2 {
				group = fields[1]
				groupSource = ""
			}
		case "source":
			if len(fields) >= 2 {
				groupSource = fields[1]
			}
		case "nuget", "clitool":
			if dependency, ok := paketRegistryDependency(fields, group, groupSource); ok {
				dependencies = append(dependencies, dependency)
			}
		case "git":
			if dependency, ok := paketGitDependency(fields, group); ok {
				dependencies = append(dependencies, dependency)
			}
		case "github":
			if dependency, ok := paketGitHubDependency(fields, group); ok {
				dependencies = append(dependencies, dependency)
			}
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func paketRegistryDependency(fields []string, group, sourceURL string) (DependencyReference, bool) {
	if len(fields) < 2 {
		return DependencyReference{}, false
	}
	name := fields[1]
	constraint := paketVersionConstraint(fields[2:])
	dependency := DependencyReference{
		PackageType:       "nuget",
		Raw:               name,
		Name:              name,
		VersionConstraint: constraint,
		SourceGroup:       group,
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             paketGroupScope(group),
	}
	if strings.EqualFold(fields[0], "clitool") {
		dependency.Scope = ScopeBuild
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
	}
	if sourceURL != "" {
		dependency.Attributes = map[string]string{"source_url": sourceURL}
	}
	return dependency, true
}

func paketGitDependency(fields []string, group string) (DependencyReference, bool) {
	if len(fields) < 2 {
		return DependencyReference{}, false
	}
	sourceURL := fields[1]
	name := paketRepositoryName(sourceURL)
	if name == "" {
		return DependencyReference{}, false
	}
	dependency := DependencyReference{
		PackageType:  "generic",
		Raw:          name + "@" + sourceURL,
		Name:         name,
		SourceGroup:  group,
		OriginKind:   OriginGit,
		Relationship: RelationshipDirect,
		Scope:        paketGroupScope(group),
		Attributes:   map[string]string{"source_url": sourceURL},
	}
	if len(fields) >= 3 {
		dependency.Attributes["source_ref"] = fields[2]
		dependency.Attributes["source_ref_kind"] = "ref"
	}
	return dependency, true
}

func paketGitHubDependency(fields []string, group string) (DependencyReference, bool) {
	if len(fields) < 2 || !strings.Contains(fields[1], "/") {
		return DependencyReference{}, false
	}
	repository := strings.TrimSuffix(fields[1], ".git")
	sourceURL := "https://github.com/" + repository + ".git"
	dependency := DependencyReference{
		PackageType:  "generic",
		Raw:          repository + "@" + sourceURL,
		Name:         repository,
		SourceGroup:  group,
		OriginKind:   OriginGit,
		Relationship: RelationshipDirect,
		Scope:        paketGroupScope(group),
		Attributes:   map[string]string{"source_url": sourceURL},
	}
	if len(fields) >= 3 {
		dependency.Raw += "/" + fields[2]
		dependency.Attributes["source_path"] = fields[2]
	}
	if len(fields) >= 4 {
		dependency.Attributes["source_ref"] = fields[3]
		dependency.Attributes["source_ref_kind"] = "ref"
	}
	return dependency, true
}

func paketVersionConstraint(fields []string) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.Contains(field, ":") {
			break
		}
		parts = append(parts, field)
	}
	return strings.Join(parts, " ")
}

func paketRepositoryName(sourceURL string) string {
	cleaned := strings.TrimSuffix(strings.TrimSuffix(sourceURL, "/"), ".git")
	return path.Base(cleaned)
}

func paketGroupScope(group string) DependencyScope {
	name := strings.ToLower(group)
	switch {
	case strings.Contains(name, "test"):
		return ScopeTest
	case strings.Contains(name, "build"):
		return ScopeBuild
	case strings.Contains(name, "dev"), strings.Contains(name, "format"):
		return ScopeDevelopment
	default:
		return ScopeRuntime
	}
}
