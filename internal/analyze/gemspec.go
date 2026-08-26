package analyze

import (
	"regexp"
	"strings"
)

type gemspecParser struct{}

var gemspecDependencyCall = regexp.MustCompile(`^\s*(?:[A-Za-z_][A-Za-z0-9_]*\.)+(add_dependency|add_runtime_dependency|add_development_dependency)(?:\s+|\s*\()`) //nolint:lll // Ruby DSL grammar

func newGemspecParser(struct{}) (sourceAnalyzer, error) {
	return gemspecParser{}, nil
}

func (gemspecParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	cleaned, err := stripExecutableDSLComments(path, string(content))
	if err != nil {
		return sourceAnalyzerResult{}, err
	}
	statements, err := executableDSLStatements(path, cleaned)
	if err != nil {
		return sourceAnalyzerResult{}, err
	}

	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})
	for _, statement := range statements {
		match := gemspecDependencyCall.FindStringSubmatch(statement)
		if match == nil {
			continue
		}
		dependency, ok := gemspecDependency(statement, match[1])
		if !ok {
			continue
		}
		key := dependency.SourceGroup + "\x00" + dependency.Raw
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func gemspecDependency(statement, method string) (DependencyReference, bool) {
	quoted := gemfileQuotedString.FindAllStringSubmatchIndex(statement, -1)
	if len(quoted) == 0 {
		return DependencyReference{}, false
	}

	name := statement[quoted[0][2]:quoted[0][3]]
	if name == "" || strings.Contains(name, "#{") {
		return DependencyReference{}, false
	}
	constraints := make([]string, 0, len(quoted)-1)
	for _, match := range quoted[1:] {
		constraint := statement[match[2]:match[3]]
		if strings.Contains(constraint, "#{") {
			return DependencyReference{}, false
		}
		constraints = append(constraints, constraint)
	}

	group := "dependencies"
	scope := ScopeRuntime
	if method == "add_development_dependency" {
		group = "development_dependencies"
		scope = ScopeDevelopment
	}
	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        scope,
	}
	if len(constraints) > 0 {
		dependency.VersionConstraint = strings.Join(constraints, ", ")
		dependency.Raw += "@" + dependency.VersionConstraint
	}
	return dependency, true
}
