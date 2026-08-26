package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

type vlangMatcherConfig struct{}

type vlangParser struct{}

var (
	vlangDependenciesField = regexp.MustCompile(`(?m)^[\t ]*dependencies[\t ]*:[\t ]*\[`)
	vlangNameField         = regexp.MustCompile(`(?m)^[\t ]*name[\t ]*:[\t ]*['\"][^'\"]+['\"]`)
)

func newVLangParser(vlangMatcherConfig) (sourceAnalyzer, error) {
	return vlangParser{}, nil
}

func (vlangParser) Analyze(_ string, content []byte) (sourceAnalyzerResult, error) {
	body, ok := vlangModuleBody(string(content))
	if !ok || !vlangNameField.MatchString(body) {
		return sourceAnalyzerResult{}, nil
	}

	match := vlangDependenciesField.FindStringIndex(body)
	if match == nil {
		return semanticAnalyzerResult([]DependencyReference{}, nil), nil
	}

	values, err := vlangDependencyValues(body[match[1]-1:])
	if err != nil {
		return semanticAnalyzerResult(nil, []string{fmt.Sprintf("dependencies: %v", err)}), nil
	}

	dependencies := make([]DependencyReference, 0, len(values))
	incomplete := make([]string, 0)
	for _, value := range values {
		dependency, message := vlangDependency(value)
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func vlangModuleBody(content string) (string, bool) {
	withoutComments := vlangWithoutComments(content)
	for start := 0; start < len(withoutComments); {
		index := strings.Index(withoutComments[start:], "Module")
		if index < 0 {
			return "", false
		}
		index += start
		afterName := index + len("Module")
		if (index > 0 && vlangIdentifierByte(withoutComments[index-1])) ||
			(afterName < len(withoutComments) && vlangIdentifierByte(withoutComments[afterName])) {
			start = afterName
			continue
		}
		open := afterName
		for open < len(withoutComments) && (withoutComments[open] == ' ' || withoutComments[open] == '\t' || withoutComments[open] == '\n' || withoutComments[open] == '\r') {
			open++
		}
		if open >= len(withoutComments) || withoutComments[open] != '{' {
			start = afterName
			continue
		}
		if close, ok := vlangMatchingBrace(withoutComments, open); ok {
			return withoutComments[open+1 : close], true
		}
		return "", false
	}
	return "", false
}

func vlangWithoutComments(content string) string {
	var result strings.Builder
	result.Grow(len(content))
	for index := 0; index < len(content); {
		if index+1 < len(content) && content[index] == '/' && content[index+1] == '/' {
			for index < len(content) && content[index] != '\n' {
				result.WriteByte(' ')
				index++
			}
			continue
		}
		if index+1 < len(content) && content[index] == '/' && content[index+1] == '*' {
			result.WriteString("  ")
			index += 2
			for index+1 < len(content) && !(content[index] == '*' && content[index+1] == '/') {
				if content[index] == '\n' {
					result.WriteByte('\n')
				} else {
					result.WriteByte(' ')
				}
				index++
			}
			if index+1 < len(content) {
				result.WriteString("  ")
				index += 2
			}
			continue
		}
		quote := content[index]
		result.WriteByte(quote)
		index++
		if quote != '\'' && quote != '"' && quote != '`' {
			continue
		}
		for index < len(content) {
			character := content[index]
			result.WriteByte(character)
			index++
			if character == '\\' && index < len(content) {
				result.WriteByte(content[index])
				index++
				continue
			}
			if character == quote {
				break
			}
		}
	}
	return result.String()
}

func vlangMatchingBrace(content string, open int) (int, bool) {
	depth := 0
	for index := open; index < len(content); index++ {
		if content[index] == '\'' || content[index] == '"' || content[index] == '`' {
			quote := content[index]
			index++
			for index < len(content) && content[index] != quote {
				if content[index] == '\\' {
					index++
				}
				index++
			}
			continue
		}
		switch content[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index, true
			}
		}
	}
	return 0, false
}

func vlangIdentifierByte(character byte) bool {
	return character == '_' || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9')
}

func vlangDependencyValues(content string) ([]string, error) {
	if len(content) == 0 || content[0] != '[' {
		return nil, fmt.Errorf("expected a list")
	}

	values := make([]string, 0)
	for index := 1; ; {
		for index < len(content) && strings.ContainsRune(" \t\r\n", rune(content[index])) {
			index++
		}
		if index >= len(content) {
			return nil, fmt.Errorf("unterminated list")
		}
		if content[index] == ']' {
			return values, nil
		}
		if content[index] != '\'' && content[index] != '"' {
			return nil, fmt.Errorf("expected a quoted dependency")
		}

		value, next, err := vlangQuotedValue(content, index)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
		index = next
		for index < len(content) && strings.ContainsRune(" \t\r\n", rune(content[index])) {
			index++
		}
		if index >= len(content) {
			return nil, fmt.Errorf("unterminated list")
		}
		if content[index] == ']' {
			return values, nil
		}
		if content[index] != ',' {
			return nil, fmt.Errorf("expected a comma or closing bracket")
		}
		index++
	}
}

func vlangQuotedValue(content string, start int) (string, int, error) {
	quote := content[start]
	var value strings.Builder
	for index := start + 1; index < len(content); index++ {
		character := content[index]
		if character == '\\' && index+1 < len(content) {
			index++
			value.WriteByte(content[index])
			continue
		}
		if character == quote {
			return value.String(), index + 1, nil
		}
		value.WriteByte(character)
	}
	return "", 0, fmt.Errorf("unterminated quoted dependency")
}

func vlangDependency(raw string) (DependencyReference, string) {
	coordinate := strings.TrimSpace(raw)
	if coordinate == "" {
		return DependencyReference{}, "dependencies: dependency must not be empty"
	}

	name, constraint, hasConstraint := strings.Cut(coordinate, "@")
	name = strings.TrimSpace(name)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies.%q: dependency name must not be empty", coordinate)
	}

	dependency := DependencyReference{
		Raw:          coordinate,
		Name:         name,
		SourceGroup:  "dependencies",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if hasConstraint {
		constraint = strings.TrimSpace(constraint)
		if constraint == "" {
			return DependencyReference{}, fmt.Sprintf("dependencies.%s: version constraint must not be empty", name)
		}
		dependency.VersionConstraint = constraint
	}
	return dependency, ""
}
