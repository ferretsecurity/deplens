package analyze

import (
	"fmt"
	"strings"
)

type ocamlOpamParser struct{}

type ocamlOpamTokenKind uint8

const (
	ocamlOpamAtom ocamlOpamTokenKind = iota
	ocamlOpamString
	ocamlOpamColon
	ocamlOpamOpenBracket
	ocamlOpamCloseBracket
	ocamlOpamOpenBrace
	ocamlOpamCloseBrace
	ocamlOpamOpenParen
	ocamlOpamCloseParen
)

type ocamlOpamToken struct {
	kind ocamlOpamTokenKind
	text string
}

type ocamlOpamField struct {
	name  string
	value []ocamlOpamToken
}

func newOCamlOpamParser(ocamlOpamParserConfig) (sourceAnalyzer, error) {
	return ocamlOpamParser{}, nil
}

func (ocamlOpamParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	tokens, err := tokenizeOCamlOpam(content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse OPAM manifest %q: %w", path, err)
	}
	fields, err := parseOCamlOpamFields(tokens)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse OPAM manifest %q: %w", path, err)
	}
	if !hasOCamlOpamVersion(fields) {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, field := range fields {
		if field.name != "depends" {
			continue
		}
		parsed, messages := parseOCamlOpamDependencies(field.value)
		dependencies = append(dependencies, parsed...)
		incomplete = append(incomplete, messages...)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func tokenizeOCamlOpam(content []byte) ([]ocamlOpamToken, error) {
	tokens := make([]ocamlOpamToken, 0)
	for offset := 0; offset < len(content); {
		character := content[offset]
		if character == ' ' || character == '\t' || character == '\r' || character == '\n' {
			offset++
			continue
		}
		if character == '#' {
			for offset < len(content) && content[offset] != '\n' {
				offset++
			}
			continue
		}
		if character == '"' {
			value, next, err := readOCamlOpamString(content, offset)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, ocamlOpamToken{kind: ocamlOpamString, text: value})
			offset = next
			continue
		}
		if kind, ok := ocamlOpamPunctuation(character); ok {
			tokens = append(tokens, ocamlOpamToken{kind: kind, text: string(character)})
			offset++
			continue
		}
		start := offset
		for offset < len(content) && !isOCamlOpamDelimiter(content[offset]) {
			offset++
		}
		if start == offset {
			return nil, fmt.Errorf("unexpected character %q", content[offset])
		}
		tokens = append(tokens, ocamlOpamToken{kind: ocamlOpamAtom, text: string(content[start:offset])})
	}
	return tokens, nil
}

func readOCamlOpamString(content []byte, offset int) (string, int, error) {
	var value strings.Builder
	for offset++; offset < len(content); offset++ {
		if content[offset] == '"' {
			return value.String(), offset + 1, nil
		}
		if content[offset] != '\\' {
			value.WriteByte(content[offset])
			continue
		}
		offset++
		if offset == len(content) {
			break
		}
		switch content[offset] {
		case 'n':
			value.WriteByte('\n')
		case 'r':
			value.WriteByte('\r')
		case 't':
			value.WriteByte('\t')
		default:
			value.WriteByte(content[offset])
		}
	}
	return "", 0, fmt.Errorf("unterminated string")
}

func ocamlOpamPunctuation(character byte) (ocamlOpamTokenKind, bool) {
	switch character {
	case ':':
		return ocamlOpamColon, true
	case '[':
		return ocamlOpamOpenBracket, true
	case ']':
		return ocamlOpamCloseBracket, true
	case '{':
		return ocamlOpamOpenBrace, true
	case '}':
		return ocamlOpamCloseBrace, true
	case '(':
		return ocamlOpamOpenParen, true
	case ')':
		return ocamlOpamCloseParen, true
	default:
		return 0, false
	}
}

func isOCamlOpamDelimiter(character byte) bool {
	return character == ' ' || character == '\t' || character == '\r' || character == '\n' || character == '#' || character == '"' ||
		character == ':' || character == '[' || character == ']' || character == '{' || character == '}' || character == '(' || character == ')'
}

func parseOCamlOpamFields(tokens []ocamlOpamToken) ([]ocamlOpamField, error) {
	fields := make([]ocamlOpamField, 0)
	depth := 0
	fieldStart := -1
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if depth == 0 && token.kind == ocamlOpamAtom && index+1 < len(tokens) && tokens[index+1].kind == ocamlOpamColon {
			if fieldStart >= 0 {
				fields[len(fields)-1].value = tokens[fieldStart:index]
			}
			fields = append(fields, ocamlOpamField{name: token.text})
			fieldStart = index + 2
			index++
			continue
		}
		switch token.kind {
		case ocamlOpamOpenBracket, ocamlOpamOpenBrace, ocamlOpamOpenParen:
			depth++
		case ocamlOpamCloseBracket, ocamlOpamCloseBrace, ocamlOpamCloseParen:
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unexpected closing delimiter %q", token.text)
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("unclosed delimiter")
	}
	if fieldStart >= 0 {
		fields[len(fields)-1].value = tokens[fieldStart:]
	}
	return fields, nil
}

func hasOCamlOpamVersion(fields []ocamlOpamField) bool {
	for _, field := range fields {
		if field.name == "opam-version" && len(field.value) > 0 {
			return true
		}
	}
	return false
}

func parseOCamlOpamDependencies(tokens []ocamlOpamToken) ([]DependencyReference, []string) {
	if len(tokens) == 0 || tokens[0].kind != ocamlOpamOpenBracket {
		return nil, []string{"depends: expected a dependency list"}
	}
	end, ok := ocamlOpamMatchingDelimiter(tokens, 0)
	if !ok || end != len(tokens)-1 {
		return nil, []string{"depends: unclosed or invalid dependency list"}
	}

	dependencies := make([]DependencyReference, 0)
	messages := make([]string, 0)
	for index := 1; index < end; {
		if tokens[index].kind != ocamlOpamString {
			messages = append(messages, fmt.Sprintf("depends: expected a quoted package name, got %q", tokens[index].text))
			index++
			continue
		}
		name := tokens[index].text
		index++
		filter := []ocamlOpamToken(nil)
		if index < end && tokens[index].kind == ocamlOpamOpenBrace {
			filterEnd, matched := ocamlOpamMatchingDelimiter(tokens, index)
			if !matched || filterEnd >= end {
				messages = append(messages, fmt.Sprintf("depends.%s: unclosed filter", name))
				break
			}
			filter = tokens[index+1 : filterEnd]
			index = filterEnd + 1
		}
		if name == "" {
			messages = append(messages, "depends: package name must not be empty")
			continue
		}
		constraint := ocamlOpamVersionConstraint(filter)
		dependency := DependencyReference{
			PackageType:       "opam",
			Raw:               name,
			Name:              name,
			VersionConstraint: constraint,
			SourceGroup:       "depends",
			OriginKind:        OriginRegistry,
			Relationship:      RelationshipDirect,
			Scope:             ocamlOpamDependencyScope(filter),
		}
		if constraint != "" {
			dependency.Raw += "@" + constraint
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies, messages
}

func ocamlOpamMatchingDelimiter(tokens []ocamlOpamToken, start int) (int, bool) {
	if start >= len(tokens) {
		return 0, false
	}
	open := tokens[start].kind
	close := map[ocamlOpamTokenKind]ocamlOpamTokenKind{
		ocamlOpamOpenBracket: ocamlOpamCloseBracket,
		ocamlOpamOpenBrace:   ocamlOpamCloseBrace,
		ocamlOpamOpenParen:   ocamlOpamCloseParen,
	}[open]
	if close == 0 {
		return 0, false
	}
	depth := 0
	for index := start; index < len(tokens); index++ {
		if tokens[index].kind == open {
			depth++
		}
		if tokens[index].kind == close {
			depth--
			if depth == 0 {
				return index, true
			}
		}
	}
	return 0, false
}

func ocamlOpamVersionConstraint(filter []ocamlOpamToken) string {
	constraints := make([]string, 0)
	for index := 0; index+1 < len(filter); index++ {
		if !isOCamlOpamVersionOperator(filter[index].text) {
			continue
		}
		if filter[index+1].kind != ocamlOpamAtom && filter[index+1].kind != ocamlOpamString {
			continue
		}
		if filter[index].text == "=" && index > 0 && filter[index-1].kind == ocamlOpamAtom && !isOCamlOpamBooleanOperator(filter[index-1].text) {
			continue
		}
		constraints = append(constraints, filter[index].text+" "+filter[index+1].text)
		index++
	}
	return strings.Join(constraints, " & ")
}

func isOCamlOpamVersionOperator(value string) bool {
	switch value {
	case "=", "!=", ">", ">=", "<", "<=", "~>":
		return true
	default:
		return false
	}
}

func isOCamlOpamBooleanOperator(value string) bool {
	return value == "&" || value == "|" || value == "(" || value == ")"
}

func ocamlOpamDependencyScope(filter []ocamlOpamToken) DependencyScope {
	for _, token := range filter {
		if token.text == "with-test" {
			return ScopeTest
		}
	}
	for _, token := range filter {
		if token.text == "with-doc" || token.text == "with-dev-setup" {
			return ScopeDevelopment
		}
	}
	return ScopeRuntime
}
