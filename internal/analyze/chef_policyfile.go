package analyze

import (
	"fmt"
	"regexp"
)

type chefPolicyfileParser struct{}

var chefPolicyfileCookbookCall = regexp.MustCompile(`^\s*cookbook(?:\s+|\s*\()`) //nolint:lll // DSL grammar

func newChefPolicyfileParser(chefPolicyfileMatcherConfig) (sourceAnalyzer, error) {
	return chefPolicyfileParser{}, nil
}

func (chefPolicyfileParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
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
	for lineNumber, statement := range statements {
		if !chefPolicyfileCookbookCall.MatchString(statement) {
			continue
		}

		dependency, ok, message := chefBerksfileDependency(statement, nil, "")
		if message != "" {
			incomplete = append(incomplete, fmt.Sprintf("Policyfile.rb line %d %s", lineNumber+1, message))
		}
		if !ok {
			continue
		}
		dependencies = appendUniqueDependency(dependencies, seen, dependency.Raw, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}
