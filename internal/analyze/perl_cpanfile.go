package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

type perlCpanfileParser struct{}

type perlCpanfileContext struct {
	depth int
	group string
	scope DependencyScope
}

var (
	perlCpanfileDeclaration  = regexp.MustCompile(`^\s*(requires|recommends)\s*(?:\(\s*)?(?:'((?:\\.|[^'])*)'|"((?:\\.|[^"])*)")(?:\s*(?:,|=>)\s*(?:'((?:\\.|[^'])*)'|"((?:\\.|[^"])*)"|([vV]?\d[A-Za-z0-9._+\-]*)))?\s*\)?\s*;?\s*$`)
	perlCpanfileCall         = regexp.MustCompile(`^\s*(?:requires|recommends)(?:\s+|\s*\()`) //nolint:lll // DSL grammar
	perlCpanfileOnBlock      = regexp.MustCompile(`^\s*on\s+(?:'([^']*)'|"([^"]*)"|([A-Za-z_][A-Za-z0-9_]*))\s*=>\s*sub\s*\{`)
	perlCpanfileFeatureBlock = regexp.MustCompile(`^\s*feature\s+(?:'([^']*)'|"([^"]*)")(?:\s*,\s*(?:'[^']*'|"[^"]*"))?\s*=>\s*sub\s*\{`)
)

func newPerlCpanfileParser(perlCpanfileMatcherConfig) (sourceAnalyzer, error) {
	return perlCpanfileParser{}, nil
}

func (perlCpanfileParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	cleaned, err := stripExecutableDSLComments(path, string(content))
	if err != nil {
		return sourceAnalyzerResult{}, err
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seen := make(map[string]struct{})
	contexts := make([]perlCpanfileContext, 0)
	depth := 0
	for lineNumber, line := range strings.Split(cleaned, "\n") {
		if match := perlCpanfileOnBlock.FindStringSubmatch(line); match != nil {
			name := firstNonEmpty(match[1], match[2], match[3])
			contexts = append(contexts, perlCpanfileOnContext(name, depth+perlCpanfileOpeningBraces(line)))
		} else if match := perlCpanfileFeatureBlock.FindStringSubmatch(line); match != nil {
			name := firstNonEmpty(match[1], match[2])
			contexts = append(contexts, perlCpanfileContext{
				depth: depth + perlCpanfileOpeningBraces(line),
				group: "feature." + name,
				scope: ScopeOptional,
			})
		}

		if match := perlCpanfileDeclaration.FindStringSubmatch(line); match != nil {
			declaration := match[1]
			name := firstNonEmpty(match[2], match[3])
			constraint := firstNonEmpty(match[4], match[5], match[6])
			context := perlCpanfileCurrentContext(contexts)
			dependency := perlCpanfileDependency(name, constraint, declaration, context)
			key := dependency.SourceGroup + "\x00" + dependency.Raw
			dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
		} else if perlCpanfileCall.MatchString(line) {
			incomplete = append(incomplete, fmt.Sprintf("cpanfile line %d has a dynamic dependency declaration that could not be extracted", lineNumber+1))
		}

		depth += perlCpanfileOpeningBraces(line) - perlCpanfileClosingBraces(line)
		for len(contexts) > 0 && contexts[len(contexts)-1].depth > depth {
			contexts = contexts[:len(contexts)-1]
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func perlCpanfileOnContext(name string, depth int) perlCpanfileContext {
	group := "on." + name
	scope := ScopeRuntime
	switch strings.ToLower(name) {
	case "test":
		scope = ScopeTest
	case "develop", "development":
		group = "on.develop"
		scope = ScopeDevelopment
	}
	return perlCpanfileContext{depth: depth, group: group, scope: scope}
}

func perlCpanfileCurrentContext(contexts []perlCpanfileContext) perlCpanfileContext {
	if len(contexts) == 0 {
		return perlCpanfileContext{group: "", scope: ScopeRuntime}
	}
	return contexts[len(contexts)-1]
}

func perlCpanfileDependency(name, constraint, declaration string, context perlCpanfileContext) DependencyReference {
	group := declaration
	if context.group != "" {
		group = context.group + "." + declaration
	}
	scope := context.scope
	if declaration == "recommends" {
		scope = ScopeOptional
	}

	dependency := DependencyReference{
		PackageType:  "cpan",
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        scope,
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency
}

func perlCpanfileOpeningBraces(line string) int {
	return perlCpanfileBraces(line, '{')
}

func perlCpanfileClosingBraces(line string) int {
	return perlCpanfileBraces(line, '}')
}

func perlCpanfileBraces(line string, wanted rune) int {
	count := 0
	var quote rune
	escaped := false
	for _, character := range line {
		if quote != 0 {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == quote {
				quote = 0
			}
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			continue
		}
		if character == wanted {
			count++
		}
	}
	return count
}
