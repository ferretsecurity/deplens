package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

type homebrewBrewfileMatcherConfig struct{}

type homebrewBrewfileParser struct{}

var (
	homebrewBrewfileDependencyCall = regexp.MustCompile(`^\s*(brew|cask|mas)(?:\s+|\s*\(\s*)`)
	homebrewBrewfileDependency     = regexp.MustCompile(`^\s*(brew|cask|mas)(?:\s+|\s*\(\s*)["']((?:\\.|[^"'\\])*)["']`)
	homebrewBrewfileMASID          = regexp.MustCompile(`\bid\s*:\s*([0-9]+)\b`)
)

func newHomebrewBrewfileParser(homebrewBrewfileMatcherConfig) (sourceAnalyzer, error) {
	return homebrewBrewfileParser{}, nil
}

func (homebrewBrewfileParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
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
		if !homebrewBrewfileDependencyCall.MatchString(statement) {
			continue
		}

		match := homebrewBrewfileDependency.FindStringSubmatch(statement)
		if match == nil || strings.TrimSpace(match[2]) == "" || strings.Contains(match[2], "#{") {
			incomplete = append(incomplete, fmt.Sprintf("Brewfile line %d has a dynamic dependency declaration that could not be extracted", lineNumber+1))
			continue
		}

		dependency := homebrewBrewfileReference(match[1], match[2], statement)
		key := dependency.SourceGroup + "\x00" + dependency.Raw
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func homebrewBrewfileReference(kind, name, statement string) DependencyReference {
	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  kind,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if kind == "mas" {
		if match := homebrewBrewfileMASID.FindStringSubmatch(statement); match != nil {
			dependency.Attributes = map[string]string{"app_store_id": match[1]}
		}
	}
	return dependency
}
