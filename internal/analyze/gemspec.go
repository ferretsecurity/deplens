package analyze

import (
	"fmt"
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
	incomplete := make([]string, 0)
	seen := make(map[string]struct{})
	for statementNumber, statement := range statements {
		match := gemspecDependencyCall.FindStringSubmatchIndex(statement)
		if match == nil {
			continue
		}
		dependency, found, complete := gemspecDependency(statement[match[1]:], statement[match[2]:match[3]])
		if found {
			key := dependency.SourceGroup + "\x00" + dependency.Raw
			dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
		}
		if !complete {
			incomplete = append(incomplete, fmt.Sprintf("gemspec statement %d has a dynamic dependency declaration that could not be fully extracted", statementNumber+1))
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func gemspecDependency(arguments, method string) (DependencyReference, bool, bool) {
	arguments = strings.TrimLeft(arguments, " \t")
	quoted := gemfileQuotedString.FindAllStringSubmatchIndex(arguments, -1)
	if len(quoted) == 0 || quoted[0][0] != 0 {
		return DependencyReference{}, false, false
	}

	name := arguments[quoted[0][2]:quoted[0][3]]
	if name == "" || strings.Contains(name, "#{") {
		return DependencyReference{}, false, false
	}

	complete := true
	staticSyntax := strings.Builder{}
	last := 0
	for _, match := range quoted {
		value := arguments[match[2]:match[3]]
		if strings.Contains(value, "#{") {
			complete = false
		}
		staticSyntax.WriteString(arguments[last:match[0]])
		last = match[1]
	}
	staticSyntax.WriteString(arguments[last:])
	if strings.Trim(staticSyntax.String(), " \t\r\n,()[]") != "" {
		complete = false
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
	if complete && len(quoted) > 1 {
		constraints := make([]string, 0, len(quoted)-1)
		for _, match := range quoted[1:] {
			constraints = append(constraints, arguments[match[2]:match[3]])
		}
		dependency.VersionConstraint = strings.Join(constraints, ", ")
		dependency.Raw += "@" + dependency.VersionConstraint
	}
	return dependency, true, complete
}
