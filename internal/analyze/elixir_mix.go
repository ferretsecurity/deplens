package analyze

import (
	"fmt"
	"strings"
	"unicode"
)

type elixirMixMatcherConfig struct{}

type elixirMixParser struct{}

func newElixirMixParser(elixirMixMatcherConfig) (sourceAnalyzer, error) {
	return elixirMixParser{}, nil
}

func (elixirMixParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	tokens, err := lexElixirMix(content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Mix manifest %q: %w", path, err)
	}
	if !isMixProject(tokens) {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0)
	for _, list := range mixDependencyLists(tokens) {
		dependencies = append(dependencies, mixDependenciesInList(list)...)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

type elixirMixTokenKind uint8

const (
	elixirMixIdentifier elixirMixTokenKind = iota
	elixirMixAtom
	elixirMixStringToken
	elixirMixOpenList
	elixirMixCloseList
	elixirMixOpenTuple
	elixirMixCloseTuple
	elixirMixOpenParen
	elixirMixCloseParen
	elixirMixComma
	elixirMixColon
	elixirMixDot
	elixirMixNewline
)

type elixirMixToken struct {
	kind  elixirMixTokenKind
	value string
}

func lexElixirMix(content []byte) ([]elixirMixToken, error) {
	tokens := make([]elixirMixToken, 0)
	for index := 0; index < len(content); {
		char := content[index]
		switch {
		case char == '#':
			for index < len(content) && content[index] != '\n' {
				index++
			}
		case char == '\n':
			tokens = append(tokens, elixirMixToken{kind: elixirMixNewline})
			index++
		case char == ' ' || char == '\t' || char == '\r':
			index++
		case char == '"':
			value, next, err := elixirMixString(content, index)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, elixirMixToken{kind: elixirMixStringToken, value: value})
			index = next
		case char == ':' && index+1 < len(content) && isElixirMixIdentifierStart(rune(content[index+1])):
			start := index + 1
			index = start + 1
			for index < len(content) && isElixirMixIdentifierPart(rune(content[index])) {
				index++
			}
			tokens = append(tokens, elixirMixToken{kind: elixirMixAtom, value: string(content[start:index])})
		case isElixirMixIdentifierStart(rune(char)):
			start := index
			index++
			for index < len(content) && isElixirMixIdentifierPart(rune(content[index])) {
				index++
			}
			tokens = append(tokens, elixirMixToken{kind: elixirMixIdentifier, value: string(content[start:index])})
		default:
			kind, ok := elixirMixPunctuation(char)
			if ok {
				tokens = append(tokens, elixirMixToken{kind: kind})
			}
			index++
		}
	}
	return tokens, nil
}

func elixirMixString(content []byte, start int) (string, int, error) {
	var value strings.Builder
	for index := start + 1; index < len(content); index++ {
		switch content[index] {
		case '\\':
			if index+1 >= len(content) {
				return "", 0, fmt.Errorf("unterminated string")
			}
			index++
			value.WriteByte(content[index])
		case '"':
			return value.String(), index + 1, nil
		default:
			value.WriteByte(content[index])
		}
	}
	return "", 0, fmt.Errorf("unterminated string")
}

func isElixirMixIdentifierStart(char rune) bool {
	return char == '_' || unicode.IsLetter(char)
}

func isElixirMixIdentifierPart(char rune) bool {
	return isElixirMixIdentifierStart(char) || unicode.IsDigit(char) || char == '?' || char == '!'
}

func elixirMixPunctuation(char byte) (elixirMixTokenKind, bool) {
	switch char {
	case '[':
		return elixirMixOpenList, true
	case ']':
		return elixirMixCloseList, true
	case '{':
		return elixirMixOpenTuple, true
	case '}':
		return elixirMixCloseTuple, true
	case '(':
		return elixirMixOpenParen, true
	case ')':
		return elixirMixCloseParen, true
	case ',':
		return elixirMixComma, true
	case ':':
		return elixirMixColon, true
	case '.':
		return elixirMixDot, true
	default:
		return 0, false
	}
}

func isMixProject(tokens []elixirMixToken) bool {
	for index := 0; index+3 < len(tokens); index++ {
		if tokens[index].kind == elixirMixIdentifier && tokens[index].value == "use" &&
			tokens[index+1].kind == elixirMixIdentifier && tokens[index+1].value == "Mix" &&
			tokens[index+2].kind == elixirMixDot &&
			tokens[index+3].kind == elixirMixIdentifier && tokens[index+3].value == "Project" {
			return true
		}
	}
	return false
}

func mixDependencyLists(tokens []elixirMixToken) [][]elixirMixToken {
	lists := make([][]elixirMixToken, 0)
	for index := 0; index+2 < len(tokens); index++ {
		if !isMixDepsDefinition(tokens, index) {
			continue
		}
		for next := index + 2; next < len(tokens); next++ {
			if tokens[next].kind == elixirMixIdentifier && tokens[next].value == "end" {
				break
			}
			if tokens[next].kind != elixirMixOpenList {
				continue
			}
			if end, ok := elixirMixDelimitedEnd(tokens, next, elixirMixOpenList, elixirMixCloseList); ok {
				lists = append(lists, tokens[next+1:end])
			}
			break
		}
	}
	return lists
}

func isMixDepsDefinition(tokens []elixirMixToken, index int) bool {
	if tokens[index].kind != elixirMixIdentifier || (tokens[index].value != "def" && tokens[index].value != "defp") {
		return false
	}
	for next := index + 1; next < len(tokens) && tokens[next].kind != elixirMixNewline; next++ {
		if tokens[next].kind == elixirMixIdentifier && tokens[next].value == "deps" {
			return true
		}
	}
	return false
}

func elixirMixDelimitedEnd(tokens []elixirMixToken, start int, open, close elixirMixTokenKind) (int, bool) {
	depth := 0
	for index := start; index < len(tokens); index++ {
		switch tokens[index].kind {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return index, true
			}
		}
	}
	return 0, false
}

func mixDependenciesInList(tokens []elixirMixToken) []DependencyReference {
	dependencies := make([]DependencyReference, 0)
	for index := 0; index < len(tokens); index++ {
		if tokens[index].kind != elixirMixOpenTuple {
			continue
		}
		end, ok := elixirMixDelimitedEnd(tokens, index, elixirMixOpenTuple, elixirMixCloseTuple)
		if !ok {
			return dependencies
		}
		if dependency, ok := mixDependency(tokens[index+1 : end]); ok {
			dependencies = append(dependencies, dependency)
		}
		index = end
	}
	return dependencies
}

func mixDependency(tokens []elixirMixToken) (DependencyReference, bool) {
	parts := mixTupleParts(tokens)
	if len(parts) == 0 || len(parts[0]) != 1 || parts[0][0].kind != elixirMixAtom {
		return DependencyReference{}, false
	}

	name := parts[0][0].value
	if name == "" {
		return DependencyReference{}, false
	}
	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  "deps",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        mixDependencyScope(parts),
	}
	if len(parts) > 1 && len(parts[1]) == 1 && parts[1][0].kind == elixirMixStringToken {
		dependency.VersionConstraint = parts[1][0].value
		dependency.Raw += "@" + dependency.VersionConstraint
	}
	return dependency, true
}

func mixTupleParts(tokens []elixirMixToken) [][]elixirMixToken {
	parts := make([][]elixirMixToken, 0)
	start, depth := 0, 0
	for index, token := range tokens {
		switch token.kind {
		case elixirMixOpenList, elixirMixOpenTuple, elixirMixOpenParen:
			depth++
		case elixirMixCloseList, elixirMixCloseTuple, elixirMixCloseParen:
			depth--
		case elixirMixComma:
			if depth == 0 {
				parts = append(parts, tokens[start:index])
				start = index + 1
			}
		}
	}
	parts = append(parts, tokens[start:])
	return parts
}

func mixDependencyScope(parts [][]elixirMixToken) DependencyScope {
	development := false
	for _, part := range parts[2:] {
		for index := 0; index+2 < len(part); index++ {
			if part[index].kind != elixirMixIdentifier || part[index].value != "only" || part[index+1].kind != elixirMixColon {
				continue
			}
			for _, token := range part[index+2:] {
				if token.kind != elixirMixAtom {
					continue
				}
				switch token.value {
				case "test":
					return ScopeTest
				case "dev", "bench":
					development = true
				}
			}
		}
	}
	if development {
		return ScopeDevelopment
	}
	return ScopeRuntime
}
