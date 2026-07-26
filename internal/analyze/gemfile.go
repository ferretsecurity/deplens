package analyze

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type gemfileParser struct{}
type gemfileLockParser struct{}

var (
	gemfileCall          = regexp.MustCompile(`^\s*gem(?:\s+|\s*\()`)
	gemfileQuotedString  = regexp.MustCompile(`["']((?:\\.|[^"'\\])*)["']`)
	gemfileOption        = regexp.MustCompile(`\b(git|github|path|source)\s*:\s*["']([^"']+)["']`)
	gemfileAnyOption     = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*\s*:`)
	gemfileGroupBlock    = regexp.MustCompile(`^\s*group\s+(.+?)\s+do\s*$`)
	gemfileBlockStart    = regexp.MustCompile(`\bdo\s*(?:\|[^|]*\|)?\s*$`)
	gemfileGroupOption   = regexp.MustCompile(`\bgroups?\s*:\s*(\[[^\]]*\]|:[A-Za-z_][A-Za-z0-9_]*|["'][^"']+["'])`)
	gemfileSymbol        = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)
	gemfileSourceCall    = regexp.MustCompile(`^\s*source\s*(?:\(\s*)?["']([^"']+)["']`)
	gemfileGemspecCall   = regexp.MustCompile(`^\s*gemspec(?:\s|\(|$)`)
	gemfileStaticCall    = regexp.MustCompile(`^\s*gem(?:\s+|\s*\()\s*["'][^"']+["']`)
	gemfileLockSpec      = regexp.MustCompile(`^\s{4}([^\s(]+)\s+\(([^)]+)\)\s*$`)
	gemfileLockDirect    = regexp.MustCompile(`^\s{2}([^\s(!]+)(?:\s+\(([^)]+)\))?!?\s*$`)
	gemfileLockAttribute = regexp.MustCompile(`^\s{2}(remote|revision|branch|ref|tag):\s*(.+?)\s*$`)
)

type gemfileParseResult struct {
	dependencies []DependencyReference
	incomplete   []string
	hasGemspec   bool
}

func newGemfileParser(gemfileMatcherConfig) (sourceAnalyzer, error) {
	return gemfileParser{}, nil
}

func newGemfileLockParser(gemfileLockMatcherConfig) (sourceAnalyzer, error) {
	return gemfileLockParser{}, nil
}

func (gemfileParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	parsed, err := parseGemfile(path, content)
	if err != nil {
		return sourceAnalyzerResult{}, err
	}
	return semanticAnalyzerResult(parsed.dependencies, parsed.incomplete), nil
}

func parseGemfile(path string, content []byte) (gemfileParseResult, error) {
	cleaned, err := stripExecutableDSLComments(path, string(content))
	if err != nil {
		return gemfileParseResult{}, err
	}

	result := gemfileParseResult{}
	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})
	blockStack := make([][]string, 0)
	globalSource := ""

	statements, err := executableDSLStatements(path, cleaned)
	if err != nil {
		return gemfileParseResult{}, err
	}
	for lineNumber, line := range statements {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if match := gemfileSourceCall.FindStringSubmatch(line); match != nil {
			globalSource = match[1]
			if gemfileBlockStart.MatchString(line) {
				blockStack = append(blockStack, nil)
			}
			continue
		}
		if gemfileGemspecCall.MatchString(line) {
			result.hasGemspec = true
			continue
		}
		if match := gemfileGroupBlock.FindStringSubmatch(line); match != nil {
			groups := gemfileGroups(match[1])
			if len(groups) == 0 {
				result.incomplete = append(result.incomplete, fmt.Sprintf("Gemfile line %d has a dynamic group declaration", lineNumber+1))
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
		if gemfileBlockStart.MatchString(line) {
			blockStack = append(blockStack, nil)
		}
		if !gemfileCall.MatchString(line) {
			continue
		}

		quoted := gemfileQuotedString.FindAllStringSubmatch(line, -1)
		if len(quoted) == 0 || strings.Contains(quoted[0][1], "#{") {
			result.incomplete = append(result.incomplete, fmt.Sprintf("dynamic gem declaration on line %d could not be extracted", lineNumber+1))
			continue
		}
		name := quoted[0][1]
		constraints := make([]string, 0)
		optionStart := len(line)
		if match := gemfileAnyOption.FindStringIndex(line); match != nil {
			optionStart = match[0]
		}
		for _, match := range gemfileQuotedString.FindAllStringSubmatchIndex(line, -1) {
			if match[0] >= optionStart {
				break
			}
			value := line[match[2]:match[3]]
			if value == name {
				continue
			}
			constraints = append(constraints, value)
		}

		groups := flattenGemfileGroups(blockStack)
		if match := gemfileGroupOption.FindStringSubmatch(line); match != nil {
			groups = append(groups, gemfileGroups(match[1])...)
		}
		groups = uniqueSortedStrings(groups)
		sourceGroup := "default"
		if len(groups) > 0 {
			sourceGroup = strings.Join(groups, ",")
		}

		dependency := DependencyReference{
			Raw:          name,
			Name:         name,
			SourceGroup:  sourceGroup,
			Relationship: RelationshipDirect,
			Scope:        gemfileScope(groups),
		}
		if len(constraints) > 0 {
			dependency.VersionConstraint = strings.Join(constraints, ", ")
			dependency.Raw += "@" + dependency.VersionConstraint
		}
		sourceURL := globalSource
		if match := gemfileOption.FindStringSubmatch(line); match != nil {
			switch match[1] {
			case "path":
				dependency.OriginKind = OriginPath
				dependency.Attributes = map[string]string{"source_path": match[2]}
			case "git":
				dependency.OriginKind = OriginGit
				dependency.Attributes = map[string]string{"source_url": match[2]}
			case "github":
				dependency.OriginKind = OriginGit
				sourceURL = "https://github.com/" + strings.TrimSuffix(match[2], ".git") + ".git"
				dependency.Attributes = map[string]string{"source_url": sourceURL}
			case "source":
				sourceURL = match[2]
			}
		}
		if dependency.OriginKind == "" && sourceURL != "" {
			dependency.OriginKind = OriginRegistry
			dependency.Attributes = map[string]string{"source_url": sourceURL}
		}
		key := dependency.SourceGroup + "\x00" + dependency.Raw
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	sortDependencyReferences(dependencies)
	result.dependencies = dependencies
	return result, nil
}

func executableDSLStatements(path, content string) ([]string, error) {
	statements := make([]string, 0)
	var current strings.Builder
	depth := 0
	var quote rune
	escaped := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" && current.Len() == 0 {
			continue
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(trimmed)
		for _, char := range line {
			if quote != 0 {
				if escaped {
					escaped = false
				} else if char == '\\' {
					escaped = true
				} else if char == quote {
					quote = 0
				}
				continue
			}
			if char == '"' || char == '\'' {
				quote = char
			} else if char == '(' || char == '[' {
				depth++
			} else if char == ')' || char == ']' {
				depth--
				if depth < 0 {
					return nil, fmt.Errorf("parse executable dependency file %q: unmatched closing delimiter", path)
				}
			}
		}
		if depth == 0 && !strings.HasSuffix(trimmed, ",") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("parse executable dependency file %q: unterminated delimiter", path)
	}
	if current.Len() > 0 {
		statements = append(statements, current.String())
	}
	return statements, nil
}

func gemfileGroups(value string) []string {
	groups := make([]string, 0)
	for _, match := range gemfileSymbol.FindAllStringSubmatch(value, -1) {
		groups = append(groups, match[1])
	}
	if len(groups) == 0 {
		for _, match := range gemfileQuotedString.FindAllStringSubmatch(value, -1) {
			groups = append(groups, match[1])
		}
	}
	return uniqueSortedStrings(groups)
}

func flattenGemfileGroups(stack [][]string) []string {
	groups := make([]string, 0)
	for _, level := range stack {
		groups = append(groups, level...)
	}
	return groups
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}

func gemfileScope(groups []string) DependencyScope {
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

type gemfileLockSection struct {
	kind       string
	attributes map[string]string
	specs      []DependencyReference
}

func (gemfileLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	sections := make([]gemfileLockSection, 0)
	direct := make(map[string]string)
	incomplete := make([]string, 0)
	var current *gemfileLockSection
	inSpecs := false
	inDependencies := false
	recognized := false

	flush := func() {
		if current != nil {
			sections = append(sections, *current)
			current = nil
		}
	}

	for lineNumber, line := range lines {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			switch strings.TrimSpace(line) {
			case "GEM", "GIT", "PATH":
				flush()
				current = &gemfileLockSection{kind: strings.TrimSpace(line), attributes: make(map[string]string)}
				inSpecs = false
				inDependencies = false
				recognized = true
			case "DEPENDENCIES":
				flush()
				inDependencies = true
				inSpecs = false
				recognized = true
			default:
				flush()
				inDependencies = false
				inSpecs = false
			}
			continue
		}
		if inDependencies {
			if match := gemfileLockDirect.FindStringSubmatch(line); match != nil {
				direct[match[1]] = strings.TrimSpace(match[2])
			} else if strings.TrimSpace(line) != "" {
				incomplete = append(incomplete, fmt.Sprintf("Gemfile.lock line %d has an unsupported dependency entry", lineNumber+1))
			}
			continue
		}
		if current == nil {
			continue
		}
		if strings.TrimSpace(line) == "specs:" {
			inSpecs = true
			continue
		}
		if !inSpecs {
			if match := gemfileLockAttribute.FindStringSubmatch(line); match != nil {
				current.attributes[match[1]] = match[2]
			}
			continue
		}
		if match := gemfileLockSpec.FindStringSubmatch(line); match != nil {
			dependency := DependencyReference{
				Raw:         match[1] + "@" + match[2],
				Name:        match[1],
				Version:     match[2],
				SourceGroup: current.kind,
			}
			switch current.kind {
			case "GEM":
				dependency.OriginKind = OriginRegistry
			case "GIT":
				dependency.OriginKind = OriginGit
			case "PATH":
				dependency.OriginKind = OriginPath
			}
			dependency.Attributes = gemfileLockAttributes(current.kind, current.attributes)
			current.specs = append(current.specs, dependency)
		}
	}
	flush()

	if !recognized {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Gemfile.lock %q: no recognized lockfile sections", path)
	}

	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})
	foundDirect := make(map[string]struct{})
	for _, section := range sections {
		for _, dependency := range section.specs {
			if constraint, exists := direct[dependency.Name]; exists {
				dependency.Relationship = RelationshipDirect
				dependency.VersionConstraint = constraint
				foundDirect[dependency.Name] = struct{}{}
			} else {
				dependency.Relationship = RelationshipTransitive
			}
			key := dependency.SourceGroup + "\x00" + dependency.Raw
			dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
		}
	}
	for name := range direct {
		if _, exists := foundDirect[name]; !exists {
			incomplete = append(incomplete, fmt.Sprintf("direct Gemfile.lock dependency %s has no resolved spec", name))
		}
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func gemfileLockAttributes(kind string, raw map[string]string) map[string]string {
	attributes := make(map[string]string)
	for key, value := range raw {
		switch {
		case kind == "PATH" && key == "remote":
			attributes["source_path"] = value
		case key == "remote":
			attributes["source_url"] = value
		case key == "revision":
			attributes["source_ref"] = value
		default:
			attributes[key] = value
		}
	}
	if len(attributes) == 0 {
		return nil
	}
	return attributes
}
