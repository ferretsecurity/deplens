package analyze

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

type iosCartfileParser struct{}

var (
	iosCartfileDeclaration       = regexp.MustCompile(`^\s*(github|git|binary)\s+"((?:\\.|[^"\\])*)"(?:\s+(.+?))?\s*$`)
	iosCartfileDeclarationPrefix = regexp.MustCompile(`^\s*(?:github|git|binary)\b`)
)

func newIOSCartfileParser(iosCartfileMatcherConfig) (sourceAnalyzer, error) {
	return iosCartfileParser{}, nil
}

func (iosCartfileParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
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

		match := iosCartfileDeclaration.FindStringSubmatch(line)
		if match == nil {
			if iosCartfileDeclarationPrefix.MatchString(line) {
				incomplete = append(incomplete, fmt.Sprintf("Cartfile line %d has a dependency declaration that could not be extracted", lineNumber+1))
			}
			continue
		}
		if strings.Contains(match[2], "#{") {
			incomplete = append(incomplete, fmt.Sprintf("Cartfile line %d has a dynamic dependency declaration that could not be extracted", lineNumber+1))
			continue
		}

		dependency := iosCartfileDependency(match[1], match[2], match[3])
		key := dependency.SourceGroup + "\x00" + dependency.Raw
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func iosCartfileDependency(kind, source, rawConstraint string) DependencyReference {
	constraint := strings.TrimSpace(rawConstraint)
	if len(constraint) >= 2 && constraint[0] == '"' && constraint[len(constraint)-1] == '"' {
		constraint = constraint[1 : len(constraint)-1]
	}

	dependency := DependencyReference{
		Raw:          source,
		Name:         source,
		SourceGroup:  "default",
		Relationship: RelationshipDirect,
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
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency
}

func iosCartfileRepositoryName(source string) string {
	name := path.Base(strings.TrimSuffix(strings.TrimSuffix(source, "/"), ".git"))
	if name == "." || name == "/" || name == "" {
		return source
	}
	return name
}
