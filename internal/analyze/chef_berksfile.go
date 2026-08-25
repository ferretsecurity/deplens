package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

type chefBerksfileParser struct{}

var (
	chefBerksfileCookbookCall = regexp.MustCompile(`^\s*cookbook(?:\s+|\s*\()`) //nolint:lll // DSL grammar
	chefBerksfileQuotedString = regexp.MustCompile(`["']((?:\\.|[^"'\\])*)["']`)
	chefBerksfileOption       = regexp.MustCompile(`\b(git|github|path|source|site|supermarket|branch|tag|ref|revision)\s*:\s*["']([^"']+)["']`)
	chefBerksfileAnyOption    = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*\s*:`)
	chefBerksfileGroupBlock   = regexp.MustCompile(`^\s*group\s+(.+?)\s+do\s*$`)
	chefBerksfileBlockStart   = regexp.MustCompile(`\bdo\s*(?:\|[^|]*\|)?\s*$`)
	chefBerksfileSourceCall   = regexp.MustCompile(`^\s*source\s*(?:\(\s*)?["']([^"']+)["']`)
	chefBerksfileGroupSymbol  = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)
)

func newChefBerksfileParser(chefBerksfileMatcherConfig) (sourceAnalyzer, error) {
	return chefBerksfileParser{}, nil
}

func (chefBerksfileParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
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
	blockStack := make([][]string, 0)
	globalSource := ""

	for lineNumber, statement := range statements {
		trimmed := strings.TrimSpace(statement)
		if trimmed == "" {
			continue
		}
		if match := chefBerksfileSourceCall.FindStringSubmatch(statement); match != nil {
			globalSource = match[1]
			if chefBerksfileBlockStart.MatchString(statement) {
				blockStack = append(blockStack, nil)
			}
			continue
		}
		if match := chefBerksfileGroupBlock.FindStringSubmatch(statement); match != nil {
			groups := chefBerksfileGroups(match[1])
			if len(groups) == 0 {
				incomplete = append(incomplete, fmt.Sprintf("Berksfile line %d has a dynamic group declaration", lineNumber+1))
			}
			blockStack = append(blockStack, groups)
			continue
		}
		if trimmed == "end" {
			if len(blockStack) > 0 {
				blockStack = blockStack[:len(blockStack)-1]
			}
			continue
		}
		if chefBerksfileBlockStart.MatchString(statement) {
			blockStack = append(blockStack, nil)
		}
		if !chefBerksfileCookbookCall.MatchString(statement) {
			continue
		}

		dependency, ok, message := chefBerksfileDependency(statement, chefBerksfileGroupsFromStack(blockStack), globalSource)
		if message != "" {
			incomplete = append(incomplete, fmt.Sprintf("Berksfile line %d %s", lineNumber+1, message))
		}
		if !ok {
			continue
		}
		key := dependency.SourceGroup + "\x00" + dependency.Raw
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func chefBerksfileDependency(statement string, groups []string, globalSource string) (DependencyReference, bool, string) {
	quoted := chefBerksfileQuotedString.FindAllStringSubmatch(statement, -1)
	if len(quoted) == 0 || strings.Contains(quoted[0][1], "#{") {
		return DependencyReference{}, false, "has a dynamic cookbook declaration that could not be extracted"
	}

	name := quoted[0][1]
	optionStart := len(statement)
	if match := chefBerksfileAnyOption.FindStringIndex(statement); match != nil {
		optionStart = match[0]
	}
	constraints := make([]string, 0)
	for _, match := range chefBerksfileQuotedString.FindAllStringSubmatchIndex(statement, -1) {
		if match[0] >= optionStart {
			break
		}
		value := statement[match[2]:match[3]]
		if value != name {
			constraints = append(constraints, value)
		}
	}

	group := "default"
	if len(groups) > 0 {
		group = strings.Join(groups, ",")
	}
	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		Relationship: RelationshipDirect,
		Scope:        chefBerksfileScope(groups),
	}
	if len(constraints) > 0 {
		dependency.VersionConstraint = strings.Join(constraints, ", ")
		dependency.Raw += "@" + dependency.VersionConstraint
	}

	sourceURL := globalSource
	for _, match := range chefBerksfileOption.FindAllStringSubmatch(statement, -1) {
		kind, value := match[1], match[2]
		switch kind {
		case "path":
			dependency.OriginKind = OriginPath
			dependency.Attributes = map[string]string{"source_path": value}
		case "git":
			dependency.OriginKind = OriginGit
			dependency.Attributes = map[string]string{"source_url": value}
		case "github":
			dependency.OriginKind = OriginGit
			sourceURL = "https://github.com/" + strings.TrimSuffix(value, ".git") + ".git"
			dependency.Attributes = map[string]string{"source_url": sourceURL}
		case "source", "site", "supermarket":
			sourceURL = value
		case "branch", "tag", "ref", "revision":
			if dependency.Attributes == nil {
				dependency.Attributes = make(map[string]string)
			}
			dependency.Attributes["source_ref"] = value
			dependency.Attributes["source_ref_kind"] = kind
		}
	}
	if dependency.OriginKind == "" {
		dependency.OriginKind = OriginRegistry
		if sourceURL != "" {
			if dependency.Attributes == nil {
				dependency.Attributes = make(map[string]string)
			}
			dependency.Attributes["source_url"] = sourceURL
		}
	}

	return dependency, true, ""
}

func chefBerksfileGroups(value string) []string {
	groups := make([]string, 0)
	for _, match := range chefBerksfileGroupSymbol.FindAllStringSubmatch(value, -1) {
		groups = append(groups, match[1])
	}
	if len(groups) == 0 {
		for _, match := range chefBerksfileQuotedString.FindAllStringSubmatch(value, -1) {
			groups = append(groups, match[1])
		}
	}
	return uniqueSortedStrings(groups)
}

func chefBerksfileGroupsFromStack(stack [][]string) []string {
	groups := make([]string, 0)
	for _, level := range stack {
		groups = append(groups, level...)
	}
	return uniqueSortedStrings(groups)
}

func chefBerksfileScope(groups []string) DependencyScope {
	for _, group := range groups {
		if strings.Contains(strings.ToLower(group), "test") {
			return ScopeTest
		}
	}
	for _, group := range groups {
		if strings.Contains(strings.ToLower(group), "development") || strings.Contains(strings.ToLower(group), "dev") {
			return ScopeDevelopment
		}
	}
	return ScopeRuntime
}
