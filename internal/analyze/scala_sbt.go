package analyze

import (
	"regexp"
	"strings"
)

type scalaSBTBuildMatcherConfig struct{}

type scalaSBTBuildParser struct{}

var scalaSBTDependencyDeclaration = regexp.MustCompile(`"([^"\r\n]+)"\s*(%%%|%%|%)\s*"([^"\r\n]+)"\s*%\s*"([^"\r\n]+)"(?:\s*%\s*"([^"\r\n]+)")?`)

func newScalaSBTBuildParser(scalaSBTBuildMatcherConfig) (sourceAnalyzer, error) {
	return scalaSBTBuildParser{}, nil
}

func (scalaSBTBuildParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	cleaned, err := stripExecutableDSLComments(path, string(content))
	if err != nil {
		return sourceAnalyzerResult{}, err
	}

	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})
	for _, match := range scalaSBTDependencyDeclaration.FindAllStringSubmatch(cleaned, -1) {
		dependency := scalaSBTDependency(match[1], match[2], match[3], match[4], match[5])
		key := dependency.Raw + "\x00" + string(dependency.Scope)
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func scalaSBTDependency(group, crossVersion, artifact, version, configuration string) DependencyReference {
	scope := ScopeRuntime
	if strings.Contains(strings.ToLower(configuration), "test") {
		scope = ScopeTest
	}

	return DependencyReference{
		Raw:               group + crossVersion + artifact + "@" + version,
		Name:              group + ":" + artifact,
		VersionConstraint: normalizeMavenManifestConstraint(version),
		SourceGroup:       "libraryDependencies",
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             scope,
	}
}
