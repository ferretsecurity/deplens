package analyze

import (
	"fmt"
	"strings"
	"unicode"
)

type luaRocksParser struct{}

type luaRocksTokenKind uint8

const (
	luaRocksTokenIdentifier luaRocksTokenKind = iota
	luaRocksTokenString
	luaRocksTokenAssignment
	luaRocksTokenOpenTable
	luaRocksTokenCloseTable
	luaRocksTokenOther
)

type luaRocksToken struct {
	kind  luaRocksTokenKind
	value string
}

func newLuaRocksParser(luaRocksMatcherConfig) (sourceAnalyzer, error) {
	return luaRocksParser{}, nil
}

func (luaRocksParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	tokens, err := luaRocksTokens(string(content))
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse LuaRocks rockspec %q: %w", path, err)
	}

	packageDeclared, versionDeclared := false, false
	dependencies := make([]DependencyReference, 0)
	depth := 0
	for index := 0; index+1 < len(tokens); index++ {
		if tokens[index].kind == luaRocksTokenOpenTable {
			depth++
			continue
		}
		if tokens[index].kind == luaRocksTokenCloseTable {
			depth--
			continue
		}
		if depth != 0 || tokens[index].kind != luaRocksTokenIdentifier || tokens[index+1].kind != luaRocksTokenAssignment {
			continue
		}
		switch tokens[index].value {
		case "package":
			packageDeclared = true
		case "version":
			versionDeclared = true
		case "dependencies":
			if index+2 >= len(tokens) || tokens[index+2].kind != luaRocksTokenOpenTable {
				continue
			}
			values, next, ok := luaRocksTableStrings(tokens, index+2)
			if !ok {
				return sourceAnalyzerResult{}, fmt.Errorf("parse LuaRocks rockspec %q: unterminated dependencies table", path)
			}
			for _, value := range values {
				if dependency, ok := luaRocksDependency(value); ok {
					dependencies = append(dependencies, dependency)
				}
			}
			index = next - 1
		}
	}

	if !packageDeclared || !versionDeclared {
		return sourceAnalyzerResult{}, nil
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func luaRocksTableStrings(tokens []luaRocksToken, start int) ([]string, int, bool) {
	values := make([]string, 0)
	depth := 0
	for index := start; index < len(tokens); index++ {
		switch tokens[index].kind {
		case luaRocksTokenOpenTable:
			depth++
		case luaRocksTokenCloseTable:
			depth--
			if depth == 0 {
				return values, index + 1, true
			}
		case luaRocksTokenString:
			if depth == 1 {
				values = append(values, tokens[index].value)
			}
		}
	}
	return nil, 0, false
}

func luaRocksDependency(value string) (DependencyReference, bool) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return DependencyReference{}, false
	}
	dependency := DependencyReference{
		Raw:          fields[0],
		Name:         fields[0],
		SourceGroup:  "dependencies",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if constraint := strings.TrimSpace(strings.TrimPrefix(value, fields[0])); constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency, true
}

func luaRocksTokens(content string) ([]luaRocksToken, error) {
	tokens := make([]luaRocksToken, 0)
	for index := 0; index < len(content); {
		character := content[index]
		if unicode.IsSpace(rune(character)) {
			index++
			continue
		}
		if character == '-' && index+1 < len(content) && content[index+1] == '-' {
			index += 2
			if equals, start, ok := luaRocksLongBracketStart(content, index); ok {
				end, found := luaRocksLongBracketEnd(content, start, equals)
				if !found {
					return nil, fmt.Errorf("unterminated block comment")
				}
				index = end
				continue
			}
			for index < len(content) && content[index] != '\n' {
				index++
			}
			continue
		}
		if character == '\'' || character == '"' {
			value, next, err := luaRocksQuotedString(content, index)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, luaRocksToken{kind: luaRocksTokenString, value: value})
			index = next
			continue
		}
		if equals, start, ok := luaRocksLongBracketStart(content, index); ok {
			end, found := luaRocksLongBracketEnd(content, start, equals)
			if !found {
				return nil, fmt.Errorf("unterminated long string")
			}
			valueStart := start + 1
			valueEnd := end - (equals + 2)
			tokens = append(tokens, luaRocksToken{kind: luaRocksTokenString, value: content[valueStart:valueEnd]})
			index = end
			continue
		}
		if isLuaRocksIdentifierStart(character) {
			end := index + 1
			for end < len(content) && isLuaRocksIdentifierPart(content[end]) {
				end++
			}
			tokens = append(tokens, luaRocksToken{kind: luaRocksTokenIdentifier, value: content[index:end]})
			index = end
			continue
		}
		switch character {
		case '=':
			tokens = append(tokens, luaRocksToken{kind: luaRocksTokenAssignment})
		case '{':
			tokens = append(tokens, luaRocksToken{kind: luaRocksTokenOpenTable})
		case '}':
			tokens = append(tokens, luaRocksToken{kind: luaRocksTokenCloseTable})
		default:
			tokens = append(tokens, luaRocksToken{kind: luaRocksTokenOther})
		}
		index++
	}
	return tokens, nil
}

func isLuaRocksIdentifierStart(character byte) bool {
	return character == '_' || ('a' <= character && character <= 'z') || ('A' <= character && character <= 'Z')
}

func isLuaRocksIdentifierPart(character byte) bool {
	return isLuaRocksIdentifierStart(character) || ('0' <= character && character <= '9')
}

func luaRocksLongBracketStart(content string, index int) (int, int, bool) {
	if index >= len(content) || content[index] != '[' {
		return 0, 0, false
	}
	equals := 0
	for start := index + 1; start < len(content) && content[start] == '='; start++ {
		equals++
		if start+1 < len(content) && content[start+1] == '[' {
			return equals, start + 1, true
		}
	}
	if index+1 < len(content) && content[index+1] == '[' {
		return 0, index + 1, true
	}
	return 0, 0, false
}

func luaRocksLongBracketEnd(content string, openingBracket, equals int) (int, bool) {
	closing := "]" + strings.Repeat("=", equals) + "]"
	end := strings.Index(content[openingBracket+1:], closing)
	if end < 0 {
		return 0, false
	}
	return openingBracket + 1 + end + len(closing), true
}

func luaRocksQuotedString(content string, index int) (string, int, error) {
	quote := content[index]
	var value strings.Builder
	for index++; index < len(content); index++ {
		if content[index] == quote {
			return value.String(), index + 1, nil
		}
		if content[index] != '\\' {
			value.WriteByte(content[index])
			continue
		}
		index++
		if index >= len(content) {
			break
		}
		switch content[index] {
		case 'a':
			value.WriteByte('\a')
		case 'b':
			value.WriteByte('\b')
		case 'f':
			value.WriteByte('\f')
		case 'n':
			value.WriteByte('\n')
		case 'r':
			value.WriteByte('\r')
		case 't':
			value.WriteByte('\t')
		case 'v':
			value.WriteByte('\v')
		default:
			value.WriteByte(content[index])
		}
	}
	return "", 0, fmt.Errorf("unterminated quoted string")
}
