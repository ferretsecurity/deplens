package analyze

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const pythonSplitComma = "comma"

type pythonMatcherConfig struct {
	CDKConstruct *pythonCDKConstructConfig `yaml:"cdk_construct"`
}

type pythonCDKConstructConfig struct {
	Module          string                        `yaml:"module"`
	Construct       string                        `yaml:"construct"`
	KeywordArgument string                        `yaml:"keyword_argument"`
	Within          []string                      `yaml:"within"`
	Conditions      []pythonObjectConditionConfig `yaml:"conditions"`
	Extract         *pythonExtractConfig          `yaml:"extract"`
}

type pythonObjectConditionConfig struct {
	Key     string  `yaml:"key"`
	Equals  *string `yaml:"equals"`
	Present bool    `yaml:"present"`
}

type pythonExtractConfig struct {
	Key   string `yaml:"key"`
	Split string `yaml:"split"`
}

type pythonObjectCondition struct {
	key     string
	equals  *string
	present bool
}

type pythonExtract struct {
	key   string
	split string
}

type pythonImportTable struct {
	namespaces map[string]struct{}
	named      map[string]struct{}
}

type pythonCDKConstructMatcher struct {
	module          string
	construct       string
	keywordArgument string
	within          []string
	conditions      []pythonObjectCondition
	extract         *pythonExtract
}

var pythonIdentifierRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func newPythonMatcher(raw pythonMatcherConfig) (manifestParser, error) {
	if raw.CDKConstruct == nil {
		return nil, fmt.Errorf("python.cdk_construct: required")
	}

	cfg := raw.CDKConstruct
	if cfg.Module == "" {
		return nil, fmt.Errorf("python.cdk_construct.module: required")
	}
	if cfg.Construct == "" {
		return nil, fmt.Errorf("python.cdk_construct.construct: required")
	}
	if cfg.KeywordArgument == "" {
		return nil, fmt.Errorf("python.cdk_construct.keyword_argument: required")
	}
	if len(cfg.Conditions) == 0 {
		return nil, fmt.Errorf("python.cdk_construct.conditions: must contain at least one entry")
	}

	within := make([]string, 0, len(cfg.Within))
	for idx, segment := range cfg.Within {
		if segment == "" {
			return nil, fmt.Errorf("python.cdk_construct.within[%d]: required", idx)
		}
		within = append(within, segment)
	}

	conditions := make([]pythonObjectCondition, 0, len(cfg.Conditions))
	for idx, cond := range cfg.Conditions {
		if cond.Key == "" {
			return nil, fmt.Errorf("python.cdk_construct.conditions[%d].key: required", idx)
		}
		if cond.Equals == nil && !cond.Present {
			return nil, fmt.Errorf("python.cdk_construct.conditions[%d]: one of equals or present=true is required", idx)
		}
		conditions = append(conditions, pythonObjectCondition{
			key:     cond.Key,
			equals:  cond.Equals,
			present: cond.Present,
		})
	}

	var extract *pythonExtract
	if cfg.Extract != nil {
		if cfg.Extract.Key == "" {
			return nil, fmt.Errorf("python.cdk_construct.extract.key: required")
		}
		if cfg.Extract.Split != pythonSplitComma {
			return nil, fmt.Errorf("python.cdk_construct.extract.split: unsupported value %q", cfg.Extract.Split)
		}
		extract = &pythonExtract{
			key:   cfg.Extract.Key,
			split: cfg.Extract.Split,
		}
	}

	return pythonCDKConstructMatcher{
		module:          cfg.Module,
		construct:       cfg.Construct,
		keywordArgument: cfg.KeywordArgument,
		within:          within,
		conditions:      conditions,
		extract:         extract,
	}, nil
}

func (m pythonCDKConstructMatcher) Match(path string, content []byte) ([]string, bool, error) {
	source := string(content)
	imports := collectPythonImports(source, m.module, m.construct)
	if len(imports.namespaces) == 0 && len(imports.named) == 0 {
		return nil, false, nil
	}

	callStarts := pythonConstructorCallStarts(source, imports, m.construct)
	for _, start := range callStarts {
		args, end, ok := pythonCallArguments(source, start)
		if !ok {
			continue
		}

		kwargsValue, ok := pythonKeywordArgumentValue(args, m.keywordArgument)
		if !ok {
			continue
		}

		objectValue, ok := resolvePythonObjectValue(source, kwargsValue, start)
		if !ok {
			continue
		}

		current := objectValue
		valid := true
		for _, segment := range m.within {
			next, ok := pythonDictValue(source, current, segment, end)
			if !ok {
				valid = false
				break
			}
			resolved, ok := resolvePythonObjectValue(source, next, end)
			if !ok {
				valid = false
				break
			}
			current = resolved
		}
		if !valid {
			continue
		}

		if !m.matchesConditions(source, current, end) {
			continue
		}

		if m.extract == nil {
			return nil, true, nil
		}

		valueExpr, ok := pythonDictValue(source, current, m.extract.key, end)
		if !ok {
			continue
		}
		value, ok := resolvePythonStringValue(source, valueExpr, end)
		if !ok {
			continue
		}

		dependencies := splitExtractedValue(value, m.extract.split)
		if len(dependencies) == 0 {
			continue
		}
		return dependencies, true, nil
	}

	return nil, false, nil
}

func (m pythonCDKConstructMatcher) matchesConditions(source string, objectExpr string, before int) bool {
	for _, cond := range m.conditions {
		valueExpr, ok := pythonDictValue(source, objectExpr, cond.key, before)
		if !ok {
			return false
		}
		if cond.present {
			continue
		}
		value, ok := resolvePythonStringValue(source, valueExpr, before)
		if !ok || value != *cond.equals {
			return false
		}
	}
	return true
}

func collectPythonImports(source string, module string, construct string) pythonImportTable {
	table := pythonImportTable{
		namespaces: make(map[string]struct{}),
		named:      make(map[string]struct{}),
	}

	lines := strings.Split(source, "\n")
	moduleParts := strings.Split(module, ".")
	parentModule := strings.Join(moduleParts[:max(len(moduleParts)-1, 0)], ".")
	moduleLeaf := moduleParts[len(moduleParts)-1]

	for _, line := range lines {
		trimmed := strings.TrimSpace(stripPythonLineComment(line))
		if trimmed == "" {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "import "):
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 && fields[1] == module {
				if len(fields) >= 4 && fields[2] == "as" {
					table.namespaces[fields[3]] = struct{}{}
				} else {
					table.namespaces[module] = struct{}{}
				}
			}
		case strings.HasPrefix(trimmed, "from "):
			parts := strings.SplitN(trimmed, " import ", 2)
			if len(parts) != 2 {
				continue
			}
			fromModule := strings.TrimSpace(strings.TrimPrefix(parts[0], "from "))
			importPart := strings.TrimSpace(parts[1])
			importFields := strings.Fields(importPart)
			if len(importFields) == 0 {
				continue
			}

			name := importFields[0]
			alias := name
			if len(importFields) >= 3 && importFields[1] == "as" {
				alias = importFields[2]
			}

			if fromModule == module && name == construct {
				table.named[alias] = struct{}{}
			}
			if parentModule != "" && fromModule == parentModule && name == moduleLeaf {
				table.namespaces[alias] = struct{}{}
			}
		}
	}

	return table
}

func pythonConstructorCallStarts(source string, imports pythonImportTable, construct string) []int {
	starts := make([]int, 0)
	seen := make(map[int]struct{})

	for namespace := range imports.namespaces {
		pattern := namespace + "." + construct + "("
		for idx := strings.Index(source, pattern); idx >= 0; {
			start := idx + len(namespace) + 1 + len(construct)
			if _, ok := seen[start]; !ok {
				seen[start] = struct{}{}
				starts = append(starts, start)
			}
			next := strings.Index(source[idx+len(pattern):], pattern)
			if next < 0 {
				break
			}
			idx += len(pattern) + next
		}
	}
	for named := range imports.named {
		pattern := named + "("
		for idx := strings.Index(source, pattern); idx >= 0; {
			if idx > 0 && isPythonIdentifierByte(source[idx-1]) {
				next := strings.Index(source[idx+len(pattern):], pattern)
				if next < 0 {
					break
				}
				idx += len(pattern) + next
				continue
			}
			start := idx + len(named)
			if _, ok := seen[start]; !ok {
				seen[start] = struct{}{}
				starts = append(starts, start)
			}
			next := strings.Index(source[idx+len(pattern):], pattern)
			if next < 0 {
				break
			}
			idx += len(pattern) + next
		}
	}

	return starts
}

func pythonCallArguments(source string, start int) (string, int, bool) {
	if start < 0 || start >= len(source) || source[start] != '(' {
		return "", 0, false
	}
	end, ok := pythonMatchingDelimiter(source, start, '(', ')')
	if !ok {
		return "", 0, false
	}
	return source[start+1 : end], end, true
}

func pythonKeywordArgumentValue(args string, name string) (string, bool) {
	for _, part := range splitTopLevel(args, ',') {
		idx := topLevelAssignmentIndex(part)
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(part[:idx])
		if key != name {
			continue
		}
		value := strings.TrimSpace(part[idx+1:])
		if value == "" {
			return "", false
		}
		return value, true
	}
	return "", false
}

func resolvePythonObjectValue(source string, expr string, before int) (string, bool) {
	value := strings.TrimSpace(expr)
	if value == "" {
		return "", false
	}
	if value[0] == '{' {
		end, ok := pythonMatchingDelimiter(value, 0, '{', '}')
		if !ok {
			return "", false
		}
		return value[:end+1], true
	}
	if !pythonIdentifierRegexp.MatchString(value) {
		return "", false
	}
	assigned, ok := findPythonAssignment(source, value, before)
	if !ok {
		return "", false
	}
	return resolvePythonObjectValue(source, assigned, before)
}

func pythonDictValue(source string, dictExpr string, key string, before int) (string, bool) {
	value := strings.TrimSpace(dictExpr)
	if value == "" || value[0] != '{' {
		return "", false
	}
	end, ok := pythonMatchingDelimiter(value, 0, '{', '}')
	if !ok {
		return "", false
	}
	body := value[1:end]
	for _, part := range splitTopLevel(body, ',') {
		idx := topLevelColonIndex(part)
		if idx < 0 {
			continue
		}
		rawKey := strings.TrimSpace(part[:idx])
		resolvedKey, ok := resolvePythonStringValue(source, rawKey, before)
		if !ok || resolvedKey != key {
			continue
		}
		rawValue := strings.TrimSpace(part[idx+1:])
		if rawValue == "" {
			return "", false
		}
		return rawValue, true
	}
	return "", false
}

func resolvePythonStringValue(source string, expr string, before int) (string, bool) {
	value := strings.TrimSpace(expr)
	if value == "" {
		return "", false
	}

	if unquoted, ok := unquotePythonString(value); ok {
		return unquoted, true
	}
	if !pythonIdentifierRegexp.MatchString(value) {
		return "", false
	}

	assigned, ok := findPythonAssignment(source, value, before)
	if !ok {
		return "", false
	}
	return resolvePythonStringValue(source, assigned, before)
}

func findPythonAssignment(source string, name string, before int) (string, bool) {
	pattern := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(name) + `[ \t]*=`)
	matches := pattern.FindAllStringIndex(source[:before], -1)
	for idx := len(matches) - 1; idx >= 0; idx-- {
		start := matches[idx][1]
		start = skipPythonWhitespace(source, start)
		if start >= before {
			continue
		}
		if expr, ok := pythonExpressionAt(source, start); ok {
			return expr, true
		}
	}
	return "", false
}

func pythonExpressionAt(source string, start int) (string, bool) {
	if start < 0 || start >= len(source) {
		return "", false
	}
	switch source[start] {
	case '{':
		end, ok := pythonMatchingDelimiter(source, start, '{', '}')
		if !ok {
			return "", false
		}
		return source[start : end+1], true
	case '\'', '"':
		end, ok := pythonStringEnd(source, start)
		if !ok {
			return "", false
		}
		return source[start : end+1], true
	default:
		if isPythonStringPrefix(source, start) {
			end, ok := pythonStringEnd(source, start+1)
			if !ok {
				return "", false
			}
			return source[start : end+1], true
		}
		end := start
		for end < len(source) && source[end] != '\n' && source[end] != '\r' {
			end++
		}
		return strings.TrimSpace(source[start:end]), true
	}
}

func pythonMatchingDelimiter(source string, start int, open byte, close byte) (int, bool) {
	depth := 0
	for i := start; i < len(source); i++ {
		switch source[i] {
		case '\'', '"':
			end, ok := pythonStringEnd(source, i)
			if !ok {
				return 0, false
			}
			i = end
		case '#':
			i = pythonLineEnd(source, i)
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func pythonStringEnd(source string, start int) (int, bool) {
	quoteIdx := start
	if isPythonStringPrefix(source, start) {
		quoteIdx++
	}
	if quoteIdx >= len(source) {
		return 0, false
	}
	quote := source[quoteIdx]
	if quote != '\'' && quote != '"' {
		return 0, false
	}
	for i := quoteIdx + 1; i < len(source); i++ {
		if source[i] == '\\' {
			i++
			continue
		}
		if source[i] == quote {
			return i, true
		}
	}
	return 0, false
}

func pythonLineEnd(source string, start int) int {
	for i := start; i < len(source); i++ {
		if source[i] == '\n' {
			return i
		}
	}
	return len(source) - 1
}

func splitTopLevel(value string, sep rune) []string {
	parts := make([]string, 0)
	start := 0
	depthParen := 0
	depthBrace := 0
	depthBracket := 0

	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\'', '"':
			end, ok := pythonStringEnd(value, i)
			if !ok {
				return []string{value}
			}
			i = end
		case '#':
			i = pythonLineEnd(value, i)
		case '(':
			depthParen++
		case ')':
			depthParen--
		case '{':
			depthBrace++
		case '}':
			depthBrace--
		case '[':
			depthBracket++
		case ']':
			depthBracket--
		default:
			if rune(value[i]) == sep && depthParen == 0 && depthBrace == 0 && depthBracket == 0 {
				parts = append(parts, strings.TrimSpace(value[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(value[start:]))

	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return filtered
}

func topLevelAssignmentIndex(value string) int {
	return topLevelRuneIndex(value, '=')
}

func topLevelColonIndex(value string) int {
	return topLevelRuneIndex(value, ':')
}

func topLevelRuneIndex(value string, needle byte) int {
	depthParen := 0
	depthBrace := 0
	depthBracket := 0
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\'', '"':
			end, ok := pythonStringEnd(value, i)
			if !ok {
				return -1
			}
			i = end
		case '#':
			i = pythonLineEnd(value, i)
		case '(':
			depthParen++
		case ')':
			depthParen--
		case '{':
			depthBrace++
		case '}':
			depthBrace--
		case '[':
			depthBracket++
		case ']':
			depthBracket--
		default:
			if value[i] == needle && depthParen == 0 && depthBrace == 0 && depthBracket == 0 {
				return i
			}
		}
	}
	return -1
}

func unquotePythonString(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	start := 0
	if isPythonStringPrefix(trimmed, 0) {
		start = 1
	}
	if start >= len(trimmed) {
		return "", false
	}
	quote := trimmed[start]
	if quote != '\'' && quote != '"' {
		return "", false
	}
	if len(trimmed) < start+2 || trimmed[len(trimmed)-1] != quote {
		return "", false
	}
	body := trimmed[start+1 : len(trimmed)-1]
	body = strings.ReplaceAll(body, `\\`, `\`)
	if quote == '\'' {
		body = strings.ReplaceAll(body, `\'`, `'`)
	} else {
		body = strings.ReplaceAll(body, `\"`, `"`)
	}
	return body, true
}

func isPythonStringPrefix(source string, idx int) bool {
	if idx < 0 || idx >= len(source) {
		return false
	}
	if idx+1 >= len(source) {
		return false
	}
	prefix := unicode.ToLower(rune(source[idx]))
	return strings.ContainsRune("rubf", prefix) && (source[idx+1] == '\'' || source[idx+1] == '"')
}

func isPythonIdentifierByte(ch byte) bool {
	return ch == '_' || ('0' <= ch && ch <= '9') || ('A' <= ch && ch <= 'Z') || ('a' <= ch && ch <= 'z')
}

func skipPythonWhitespace(source string, idx int) int {
	for idx < len(source) && (source[idx] == ' ' || source[idx] == '\t' || source[idx] == '\n' || source[idx] == '\r') {
		idx++
	}
	return idx
}

func stripPythonLineComment(line string) string {
	for idx := 0; idx < len(line); idx++ {
		switch line[idx] {
		case '\'', '"':
			end, ok := pythonStringEnd(line, idx)
			if !ok {
				return line
			}
			idx = end
		case '#':
			return line[:idx]
		}
	}
	return line
}
