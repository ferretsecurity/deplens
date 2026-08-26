package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

type iosPodfileParser struct{}

var (
	iosPodfileCall       = regexp.MustCompile(`^\s*pod(?:\s+|\s*\()`) //nolint:lll // DSL grammar
	iosPodfileQuoted     = regexp.MustCompile(`["']((?:\\.|[^"'\\])*)["']`)
	iosPodfileOption     = regexp.MustCompile(`(?::[A-Za-z_][A-Za-z0-9_]*\s*=>|[A-Za-z_][A-Za-z0-9_]*\s*:)`)
	iosPodfileSourceCall = regexp.MustCompile(`^\s*source\s*(?:\(\s*)?["']([^"']+)["']`)
	iosPodfilePath       = regexp.MustCompile(`(?::path\s*=>|path:)\s*["']([^"']+)["']`)
	iosPodfileGit        = regexp.MustCompile(`(?::git\s*=>|git:)\s*["']([^"']+)["']`)
	iosPodfilePathOption = regexp.MustCompile(`(?::path\s*=>|path:)`)
	iosPodfileGitOption  = regexp.MustCompile(`(?::git\s*=>|git:)`)
)

func newIOSPodfileParser(iosPodfileMatcherConfig) (sourceAnalyzer, error) {
	return iosPodfileParser{}, nil
}

func (iosPodfileParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
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
	indices := make(map[string]int)
	sourceURL := ""
	for statementNumber, statement := range statements {
		if match := iosPodfileSourceCall.FindStringSubmatch(statement); match != nil {
			sourceURL = match[1]
			continue
		}
		if !iosPodfileCall.MatchString(statement) {
			continue
		}

		dependency, ok := iosPodfileDependency(statement, sourceURL)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("Podfile statement %d has a dynamic pod declaration that could not be extracted", statementNumber+1))
			continue
		}
		key := dependency.SourceGroup + "\x00" + dependency.Raw
		if index, exists := indices[key]; exists {
			if dependencies[index].OriginKind == "" && dependency.OriginKind != "" {
				dependencies[index] = dependency
			}
			continue
		}
		indices[key] = len(dependencies)
		dependencies = append(dependencies, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func iosPodfileDependency(statement, sourceURL string) (DependencyReference, bool) {
	quoted := iosPodfileQuoted.FindAllStringSubmatchIndex(statement, -1)
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
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if len(constraints) > 0 {
		dependency.VersionConstraint = strings.Join(constraints, ", ")
		dependency.Raw += "@" + dependency.VersionConstraint
	}
	switch {
	case iosPodfilePath.MatchString(statement):
		dependency.OriginKind = OriginPath
		dependency.Attributes = map[string]string{"source_path": iosPodfilePath.FindStringSubmatch(statement)[1]}
	case iosPodfileGit.MatchString(statement):
		dependency.OriginKind = OriginGit
		dependency.Attributes = map[string]string{"source_url": iosPodfileGit.FindStringSubmatch(statement)[1]}
	case iosPodfilePathOption.MatchString(statement), iosPodfileGitOption.MatchString(statement):
		// The pod name remains useful even when its source is selected dynamically.
	case sourceURL != "":
		dependency.OriginKind = OriginRegistry
		dependency.Attributes = map[string]string{"source_url": sourceURL}
	default:
		dependency.OriginKind = OriginRegistry
	}
	return dependency, true
}
