package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

type iosCartfileResolvedParser struct{}

var (
	iosCartfileResolvedDeclaration       = regexp.MustCompile(`^\s*(github|git|binary)\s+"((?:\\.|[^"\\])*)"\s+"((?:\\.|[^"\\])*)"\s*$`)
	iosCartfileResolvedDeclarationPrefix = regexp.MustCompile(`^\s*(?:github|git|binary)\b`)
	iosCartfileResolvedCommit            = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)
)

func newIOSCartfileResolvedParser(iosCartfileResolvedMatcherConfig) (sourceAnalyzer, error) {
	return iosCartfileResolvedParser{}, nil
}

func (iosCartfileResolvedParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	cleaned, err := stripExecutableDSLComments(path, string(content))
	if err != nil {
		return sourceAnalyzerResult{}, err
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seen := make(map[string]struct{})
	for lineNumber, line := range strings.Split(cleaned, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		match := iosCartfileResolvedDeclaration.FindStringSubmatch(line)
		if match == nil {
			if iosCartfileResolvedDeclarationPrefix.MatchString(line) {
				incomplete = append(incomplete, fmt.Sprintf("Cartfile.resolved line %d has a resolved dependency that could not be extracted", lineNumber+1))
			}
			continue
		}

		source := strings.TrimSpace(match[2])
		version := strings.TrimSpace(match[3])
		if source == "" || version == "" || strings.Contains(source, "#{") || strings.Contains(version, "#{") {
			incomplete = append(incomplete, fmt.Sprintf("Cartfile.resolved line %d has a dynamic or incomplete resolved dependency", lineNumber+1))
			continue
		}

		dependency := iosCartfileResolvedDependency(match[1], source, version)
		key := dependency.SourceGroup + "\x00" + dependency.Raw
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func iosCartfileResolvedDependency(kind, source, version string) DependencyReference {
	dependency := DependencyReference{
		Raw:          source + "@" + version,
		Name:         source,
		Version:      version,
		SourceGroup:  "default",
		Relationship: RelationshipInconclusive,
		Scope:        ScopeRuntime,
	}

	switch kind {
	case "github":
		dependency.PackageType = "github"
		dependency.OriginKind = OriginGit
		dependency.Attributes = map[string]string{"source_url": "https://github.com/" + strings.TrimSuffix(source, ".git") + ".git"}
	case "git":
		dependency.PackageType = "generic"
		dependency.OriginKind = OriginGit
		dependency.Name = iosCartfileRepositoryName(source)
		dependency.Attributes = map[string]string{"source_url": source}
	case "binary":
		dependency.PackageType = "generic"
		dependency.OriginKind = OriginURL
		dependency.Attributes = map[string]string{"source_url": source}
	}
	if dependency.OriginKind == OriginGit && iosCartfileResolvedCommit.MatchString(version) {
		dependency.Attributes["source_ref"] = version
		dependency.Attributes["source_ref_kind"] = "commit"
	}
	return dependency
}
