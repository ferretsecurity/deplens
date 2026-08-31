package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

type iosPodspecParser struct{}

var (
	iosPodspecDependencyCall = regexp.MustCompile(`^\s*(?:[A-Za-z_][A-Za-z0-9_]*\.)+dependency(?:\s+|\s*\()`) //nolint:lll // DSL grammar
	iosPodspecQuoted         = regexp.MustCompile(`["']((?:\\.|[^"'\\])*)["']`)
)

func newIOSPodspecParser(iosPodspecMatcherConfig) (sourceAnalyzer, error) {
	return iosPodspecParser{}, nil
}

func (iosPodspecParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	cleaned, err := stripExecutableDSLComments(path, string(content))
	if err != nil {
		return sourceAnalyzerResult{}, err
	}
	statements, err := executableDSLStatements(path, cleaned)
	if err != nil {
		return sourceAnalyzerResult{}, err
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seen := make(map[string]struct{})
	for statementNumber, statement := range statements {
		if !iosPodspecDependencyCall.MatchString(statement) {
			continue
		}

		dependency, ok := iosPodspecDependency(statement)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("podspec statement %d has a dynamic dependency declaration that could not be extracted", statementNumber+1))
			continue
		}
		key := dependency.SourceGroup + "\x00" + dependency.Raw
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func iosPodspecDependency(statement string) (DependencyReference, bool) {
	quoted := iosPodspecQuoted.FindAllStringSubmatchIndex(statement, -1)
	if len(quoted) == 0 {
		return DependencyReference{}, false
	}

	name := statement[quoted[0][2]:quoted[0][3]]
	if name == "" || strings.Contains(name, "#{") {
		return DependencyReference{}, false
	}
	optionStart := len(statement)
	if match := iosPodfileOption.FindStringIndex(statement); match != nil {
		optionStart = match[0]
	}
	constraints := make([]string, 0)
	for _, match := range quoted[1:] {
		if match[0] >= optionStart {
			break
		}
		value := statement[match[2]:match[3]]
		if strings.Contains(value, "#{") {
			return DependencyReference{}, false
		}
		constraints = append(constraints, value)
	}

	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  "default",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if len(constraints) > 0 {
		dependency.VersionConstraint = strings.Join(constraints, ", ")
		dependency.Raw += "@" + dependency.VersionConstraint
	}
	return dependency, true
}
