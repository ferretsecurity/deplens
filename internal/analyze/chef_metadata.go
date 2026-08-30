package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

type chefMetadataParser struct{}

var (
	chefMetadataIdentity           = regexp.MustCompile(`^\s*(?:name|version)\s*(?:\(\s*)?["'][^"']+["']`)
	chefMetadataDepends            = regexp.MustCompile(`^\s*depends\s*(?:\(\s*)?["']([^"']+)["'](?:\s*,\s*["']([^"']+)["'])?\s*\)?\s*$`)
	chefMetadataDependsVariable    = regexp.MustCompile(`^\s*depends\s*(?:\(\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\)?\s*$`)
	chefMetadataStaticWordListLoop = regexp.MustCompile(`^\s*%w(?:\{([^}]*)\}|\(([^)]*)\)|\[([^]]*)\])\.each\s+do\s+\|([A-Za-z_][A-Za-z0-9_]*)\|\s*$`)
	chefMetadataDependsCall        = regexp.MustCompile(`^\s*depends(?:\s+|\s*\()`) //nolint:lll // DSL grammar
	chefMetadataLoopEnd            = regexp.MustCompile(`^\s*end\s*$`)
)

func newChefMetadataParser(chefMetadataMatcherConfig) (sourceAnalyzer, error) {
	return chefMetadataParser{}, nil
}

func (chefMetadataParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	cleaned, err := stripExecutableDSLComments(path, string(content))
	if err != nil {
		return sourceAnalyzerResult{}, err
	}

	statements, err := executableDSLStatements(path, cleaned)
	if err != nil {
		return sourceAnalyzerResult{}, err
	}

	recognized := false
	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seen := make(map[string]struct{})
	staticLoopVariables := chefMetadataStaticLoopDependencies(cleaned, &dependencies, seen)
	if len(staticLoopVariables) > 0 {
		recognized = true
	}

	for lineNumber, statement := range statements {
		if chefMetadataIdentity.MatchString(statement) {
			recognized = true
		}
		if match := chefMetadataDepends.FindStringSubmatch(statement); match != nil {
			recognized = true
			dependency := chefMetadataDependency(match[1], match[2])
			dependencies = appendUniqueDependency(dependencies, seen, dependency.Raw, dependency)
			continue
		}
		if match := chefMetadataDependsVariable.FindStringSubmatch(statement); match != nil {
			recognized = true
			if staticLoopVariables[match[1]] {
				continue
			}
			incomplete = append(incomplete, fmt.Sprintf("metadata.rb line %d has a dynamic cookbook declaration that could not be extracted", lineNumber+1))
			continue
		}
		if chefMetadataDependsCall.MatchString(statement) {
			recognized = true
			incomplete = append(incomplete, fmt.Sprintf("metadata.rb line %d has a cookbook declaration that could not be extracted", lineNumber+1))
		}
	}

	if !recognized {
		return sourceAnalyzerResult{}, nil
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func chefMetadataStaticLoopDependencies(content string, dependencies *[]DependencyReference, seen map[string]struct{}) map[string]bool {
	variables := make(map[string]bool)
	lines := strings.Split(content, "\n")
	for lineNumber, line := range lines {
		match := chefMetadataStaticWordListLoop.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		words := firstNonEmpty(match[1], match[2], match[3])
		variable := match[4]
		for index := lineNumber + 1; index < len(lines) && !chefMetadataLoopEnd.MatchString(lines[index]); index++ {
			depends := chefMetadataDependsVariable.FindStringSubmatch(lines[index])
			if depends == nil || depends[1] != variable {
				continue
			}
			variables[variable] = true
			for _, name := range strings.Fields(words) {
				dependency := chefMetadataDependency(name, "")
				*dependencies = appendUniqueDependency(*dependencies, seen, dependency.Raw, dependency)
			}
		}
	}
	return variables
}

func chefMetadataDependency(name, constraint string) DependencyReference {
	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  "default",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
