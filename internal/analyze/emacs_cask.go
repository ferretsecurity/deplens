package analyze

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type emacsCaskMatcherConfig struct{}

type emacsCaskParser struct{}

type emacsCaskNodeKind uint8

const (
	emacsCaskAtom emacsCaskNodeKind = iota
	emacsCaskString
	emacsCaskList
	emacsCaskQuoted
)

type emacsCaskNode struct {
	kind     emacsCaskNodeKind
	value    string
	children []emacsCaskNode
}

type emacsCaskTokenKind uint8

const (
	emacsCaskTokenAtom emacsCaskTokenKind = iota
	emacsCaskTokenString
	emacsCaskTokenOpenList
	emacsCaskTokenCloseList
	emacsCaskTokenQuote
)

type emacsCaskToken struct {
	kind  emacsCaskTokenKind
	value string
	pos   int
}

func newEmacsCaskParser(emacsCaskMatcherConfig) (sourceAnalyzer, error) {
	return emacsCaskParser{}, nil
}

func (emacsCaskParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	forms, err := parseEmacsCaskForms(content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Cask manifest %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seenDependencies := make(map[string]struct{})
	recognized := collectEmacsCaskDependencies(forms, false, &dependencies, &incomplete, seenDependencies)
	if !recognized {
		return sourceAnalyzerResult{}, nil
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func collectEmacsCaskDependencies(nodes []emacsCaskNode, development bool, dependencies *[]DependencyReference, incomplete *[]string, seen map[string]struct{}) bool {
	recognized := false
	for _, node := range nodes {
		if node.kind != emacsCaskList || len(node.children) == 0 || node.children[0].kind != emacsCaskAtom {
			continue
		}

		switch node.children[0].value {
		case "source", "package", "package-file", "files":
			recognized = true
		case "development":
			recognized = true
			if collectEmacsCaskDependencies(node.children[1:], true, dependencies, incomplete, seen) {
				recognized = true
			}
		case "depends-on":
			recognized = true
			dependency, ok, message := emacsCaskDependency(node, development)
			if message != "" {
				*incomplete = append(*incomplete, message)
			}
			if !ok {
				continue
			}
			key := dependency.SourceGroup + "\x00" + dependency.Name + "\x00" + dependency.VersionConstraint
			*dependencies = appendUniqueDependency(*dependencies, seen, key, dependency)
		}
	}
	return recognized
}

func emacsCaskDependency(form emacsCaskNode, development bool) (DependencyReference, bool, string) {
	if len(form.children) < 2 || form.children[1].kind != emacsCaskString || strings.TrimSpace(form.children[1].value) == "" {
		return DependencyReference{}, false, "depends-on declaration has no static package name"
	}
	if len(form.children) > 3 || (len(form.children) == 3 && form.children[2].kind != emacsCaskString) {
		return DependencyReference{}, false, fmt.Sprintf("depends-on %q has an unsupported declaration", form.children[1].value)
	}

	name := form.children[1].value
	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  "dependencies",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if development {
		dependency.SourceGroup = "development"
		dependency.Scope = ScopeDevelopment
	}
	if len(form.children) == 3 {
		dependency.VersionConstraint = form.children[2].value
		dependency.Raw += "@" + dependency.VersionConstraint
	}
	return dependency, true, ""
}

func parseEmacsCaskForms(content []byte) ([]emacsCaskNode, error) {
	tokens, err := lexEmacsCask(content)
	if err != nil {
		return nil, err
	}

	forms := make([]emacsCaskNode, 0)
	for index := 0; index < len(tokens); {
		form, next, err := parseEmacsCaskForm(tokens, index)
		if err != nil {
			return nil, err
		}
		forms = append(forms, form)
		index = next
	}
	return forms, nil
}

func parseEmacsCaskForm(tokens []emacsCaskToken, index int) (emacsCaskNode, int, error) {
	if index >= len(tokens) {
		return emacsCaskNode{}, index, fmt.Errorf("unexpected end of file")
	}
	token := tokens[index]
	switch token.kind {
	case emacsCaskTokenAtom:
		return emacsCaskNode{kind: emacsCaskAtom, value: token.value}, index + 1, nil
	case emacsCaskTokenString:
		return emacsCaskNode{kind: emacsCaskString, value: token.value}, index + 1, nil
	case emacsCaskTokenQuote:
		child, next, err := parseEmacsCaskForm(tokens, index+1)
		if err != nil {
			return emacsCaskNode{}, index, fmt.Errorf("quote at byte %d: %w", token.pos, err)
		}
		return emacsCaskNode{kind: emacsCaskQuoted, children: []emacsCaskNode{child}}, next, nil
	case emacsCaskTokenOpenList:
		children := make([]emacsCaskNode, 0)
		for index = index + 1; ; {
			if index >= len(tokens) {
				return emacsCaskNode{}, index, fmt.Errorf("unterminated list opened at byte %d", token.pos)
			}
			if tokens[index].kind == emacsCaskTokenCloseList {
				return emacsCaskNode{kind: emacsCaskList, children: children}, index + 1, nil
			}
			child, next, err := parseEmacsCaskForm(tokens, index)
			if err != nil {
				return emacsCaskNode{}, index, err
			}
			children = append(children, child)
			index = next
		}
	default:
		return emacsCaskNode{}, index, fmt.Errorf("unexpected closing parenthesis at byte %d", token.pos)
	}
}

func lexEmacsCask(content []byte) ([]emacsCaskToken, error) {
	tokens := make([]emacsCaskToken, 0)
	for index := 0; index < len(content); {
		current := content[index]
		switch {
		case unicode.IsSpace(rune(current)) || current == ',':
			index++
		case current == ';':
			for index < len(content) && content[index] != '\n' {
				index++
			}
		case current == '(':
			tokens = append(tokens, emacsCaskToken{kind: emacsCaskTokenOpenList, pos: index})
			index++
		case current == ')':
			tokens = append(tokens, emacsCaskToken{kind: emacsCaskTokenCloseList, pos: index})
			index++
		case current == '\'', current == '`', current == '@':
			tokens = append(tokens, emacsCaskToken{kind: emacsCaskTokenQuote, pos: index})
			index++
		case current == '"':
			value, next, err := lexEmacsCaskString(content, index)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, emacsCaskToken{kind: emacsCaskTokenString, value: value, pos: index})
			index = next
		default:
			start := index
			for index < len(content) && !isEmacsCaskDelimiter(content[index]) {
				index++
			}
			if start == index {
				return nil, fmt.Errorf("unexpected byte %q at %d", content[index], index)
			}
			tokens = append(tokens, emacsCaskToken{kind: emacsCaskTokenAtom, value: string(content[start:index]), pos: start})
		}
	}
	return tokens, nil
}

func lexEmacsCaskString(content []byte, start int) (string, int, error) {
	for index := start + 1; index < len(content); index++ {
		if content[index] == '\\' {
			index++
			continue
		}
		if content[index] != '"' {
			continue
		}
		value, err := strconv.Unquote(string(content[start : index+1]))
		if err != nil {
			return "", 0, fmt.Errorf("string at byte %d: %w", start, err)
		}
		return value, index + 1, nil
	}
	return "", 0, fmt.Errorf("unterminated string at byte %d", start)
}

func isEmacsCaskDelimiter(value byte) bool {
	return unicode.IsSpace(rune(value)) || value == ',' || value == ';' || value == '(' || value == ')' || value == '\'' || value == '`' || value == '@' || value == '"'
}
