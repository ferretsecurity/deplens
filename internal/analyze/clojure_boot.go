package analyze

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type clojureBootMatcherConfig struct{}

type clojureBootParser struct{}

type clojureNodeKind uint8

const (
	clojureScalar clojureNodeKind = iota
	clojureList
	clojureVector
	clojureMap
	clojureSet
	clojureReaderMacro
)

type clojureNode struct {
	kind     clojureNodeKind
	value    string
	children []clojureNode
}

type clojureTokenKind uint8

const (
	clojureTokenScalar clojureTokenKind = iota
	clojureTokenOpenList
	clojureTokenOpenVector
	clojureTokenOpenMap
	clojureTokenOpenSet
	clojureTokenCloseList
	clojureTokenCloseVector
	clojureTokenCloseMap
	clojureTokenReaderMacro
)

type clojureToken struct {
	kind  clojureTokenKind
	value string
	pos   int
}

func newClojureBootParser(clojureBootMatcherConfig) (sourceAnalyzer, error) {
	return clojureBootParser{}, nil
}

func (clojureBootParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	forms, err := parseClojureForms(content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Clojure Boot file %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seenDependencies := make(map[string]struct{})
	seenMessages := make(map[string]struct{})
	for _, form := range forms {
		collectClojureBootDependencies(form, &dependencies, &incomplete, seenDependencies, seenMessages)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func collectClojureBootDependencies(node clojureNode, dependencies *[]DependencyReference, incomplete *[]string, seenDependencies, seenMessages map[string]struct{}) {
	if node.kind == clojureList && len(node.children) > 0 && clojureNodeValue(node.children[0]) == "set-env!" {
		for index := 1; index+1 < len(node.children); index++ {
			if clojureNodeValue(node.children[index]) != ":dependencies" {
				continue
			}
			collectClojureBootDependencyVector(node.children[index+1], dependencies, incomplete, seenDependencies, seenMessages)
			index++
		}
	}
	for _, child := range node.children {
		collectClojureBootDependencies(child, dependencies, incomplete, seenDependencies, seenMessages)
	}
}

func collectClojureBootDependencyVector(node clojureNode, dependencies *[]DependencyReference, incomplete *[]string, seenDependencies, seenMessages map[string]struct{}) {
	node = unwrapClojureReaderMacros(node)
	if node.kind != clojureVector {
		*incomplete = appendUniqueMessage(*incomplete, seenMessages, "set-env! :dependencies is not a static vector")
		return
	}
	for _, entry := range node.children {
		dependency, ok, message := clojureBootDependency(entry)
		if message != "" {
			*incomplete = appendUniqueMessage(*incomplete, seenMessages, message)
		}
		if !ok {
			continue
		}
		key := dependency.Name + "\x00" + dependency.VersionConstraint + "\x00" + string(dependency.Scope)
		*dependencies = appendUniqueDependency(*dependencies, seenDependencies, key, dependency)
	}
}

func clojureBootDependency(entry clojureNode) (DependencyReference, bool, string) {
	entry = unwrapClojureReaderMacros(entry)
	if entry.kind != clojureVector || len(entry.children) == 0 {
		return DependencyReference{}, false, "set-env! :dependencies contains a non-vector declaration"
	}

	name := clojureNodeValue(entry.children[0])
	if name == "" || strings.HasPrefix(name, ":") {
		return DependencyReference{}, false, "set-env! :dependencies contains a declaration without a static coordinate"
	}

	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  "dependencies",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	index := 1
	if index < len(entry.children) && clojureNodeValue(entry.children[index]) != "" && !strings.HasPrefix(clojureNodeValue(entry.children[index]), ":") {
		version := clojureNodeValue(entry.children[index])
		dependency.Raw += "@" + version
		dependency.VersionConstraint = normalizeMavenManifestConstraint(version)
		index++
	}

	for index < len(entry.children) {
		key := clojureNodeValue(entry.children[index])
		if !strings.HasPrefix(key, ":") || index+1 >= len(entry.children) {
			return DependencyReference{}, false, fmt.Sprintf("set-env! :dependencies %s has unsupported options", name)
		}
		value := clojureNodeValue(entry.children[index+1])
		if value == "" {
			return DependencyReference{}, false, fmt.Sprintf("set-env! :dependencies %s has a non-static %s option", name, key)
		}
		switch key {
		case ":scope":
			dependency.Scope = clojureBootScope(value)
		default:
			if dependency.Attributes == nil {
				dependency.Attributes = make(map[string]string)
			}
			dependency.Attributes[strings.TrimPrefix(key, ":")] = value
		}
		index += 2
	}
	return dependency, true, ""
}

func clojureBootScope(value string) DependencyScope {
	switch value {
	case "test":
		return ScopeTest
	case "provided":
		return ScopeBuild
	default:
		return ScopeRuntime
	}
}

func clojureNodeValue(node clojureNode) string {
	if node.kind != clojureScalar {
		return ""
	}
	return node.value
}

func unwrapClojureReaderMacros(node clojureNode) clojureNode {
	for node.kind == clojureReaderMacro && len(node.children) == 1 {
		node = node.children[0]
	}
	return node
}

func parseClojureForms(content []byte) ([]clojureNode, error) {
	tokens, err := lexClojure(string(content))
	if err != nil {
		return nil, err
	}
	forms := make([]clojureNode, 0)
	for index := 0; index < len(tokens); {
		form, next, err := parseClojureForm(tokens, index)
		if err != nil {
			return nil, err
		}
		forms = append(forms, form)
		index = next
	}
	return forms, nil
}

func parseClojureForm(tokens []clojureToken, index int) (clojureNode, int, error) {
	if index >= len(tokens) {
		return clojureNode{}, index, fmt.Errorf("unexpected end of file")
	}
	token := tokens[index]
	switch token.kind {
	case clojureTokenScalar:
		return clojureNode{kind: clojureScalar, value: token.value}, index + 1, nil
	case clojureTokenReaderMacro:
		child, next, err := parseClojureForm(tokens, index+1)
		if err != nil {
			return clojureNode{}, index, fmt.Errorf("reader macro at byte %d: %w", token.pos, err)
		}
		return clojureNode{kind: clojureReaderMacro, children: []clojureNode{child}}, next, nil
	case clojureTokenOpenList, clojureTokenOpenVector, clojureTokenOpenMap, clojureTokenOpenSet:
		kind, close := clojureCollectionKinds(token.kind)
		children := make([]clojureNode, 0)
		index++
		for {
			if index >= len(tokens) {
				return clojureNode{}, index, fmt.Errorf("unterminated collection opened at byte %d", token.pos)
			}
			if tokens[index].kind == close {
				return clojureNode{kind: kind, children: children}, index + 1, nil
			}
			if isClojureClosingToken(tokens[index].kind) {
				return clojureNode{}, index, fmt.Errorf("mismatched closing delimiter at byte %d", tokens[index].pos)
			}
			child, next, err := parseClojureForm(tokens, index)
			if err != nil {
				return clojureNode{}, index, err
			}
			children = append(children, child)
			index = next
		}
	default:
		return clojureNode{}, index, fmt.Errorf("unexpected closing delimiter at byte %d", token.pos)
	}
}

func clojureCollectionKinds(token clojureTokenKind) (clojureNodeKind, clojureTokenKind) {
	switch token {
	case clojureTokenOpenList:
		return clojureList, clojureTokenCloseList
	case clojureTokenOpenVector:
		return clojureVector, clojureTokenCloseVector
	case clojureTokenOpenMap:
		return clojureMap, clojureTokenCloseMap
	default:
		return clojureSet, clojureTokenCloseMap
	}
}

func isClojureClosingToken(token clojureTokenKind) bool {
	return token == clojureTokenCloseList || token == clojureTokenCloseVector || token == clojureTokenCloseMap
}

func lexClojure(content string) ([]clojureToken, error) {
	tokens := make([]clojureToken, 0)
	for index := 0; index < len(content); {
		current := content[index]
		if unicode.IsSpace(rune(current)) || current == ',' {
			index++
			continue
		}
		if current == ';' {
			for index < len(content) && content[index] != '\n' {
				index++
			}
			continue
		}
		if current == '"' {
			start := index
			index++
			escaped := false
			for index < len(content) {
				if escaped {
					escaped = false
				} else if content[index] == '\\' {
					escaped = true
				} else if content[index] == '"' {
					index++
					value, err := unquoteClojureString(content[start:index])
					if err != nil {
						return nil, fmt.Errorf("string at byte %d: %w", start, err)
					}
					tokens = append(tokens, clojureToken{kind: clojureTokenScalar, value: value, pos: start})
					break
				}
				index++
			}
			if index == len(content) && (len(tokens) == 0 || tokens[len(tokens)-1].pos != start) {
				return nil, fmt.Errorf("unterminated string at byte %d", start)
			}
			continue
		}
		if current == '#' {
			if index+1 < len(content) && content[index+1] == '{' {
				tokens = append(tokens, clojureToken{kind: clojureTokenOpenSet, pos: index})
				index += 2
			} else {
				tokens = append(tokens, clojureToken{kind: clojureTokenReaderMacro, pos: index})
				index++
			}
			continue
		}

		kind, ok := clojurePunctuationToken(current)
		if ok {
			tokens = append(tokens, clojureToken{kind: kind, pos: index})
			index++
			continue
		}

		start := index
		for index < len(content) && !isClojureDelimiter(content[index]) {
			index++
		}
		if start == index {
			return nil, fmt.Errorf("unexpected byte %q at %d", content[index], index)
		}
		tokens = append(tokens, clojureToken{kind: clojureTokenScalar, value: content[start:index], pos: start})
	}
	return tokens, nil
}

func unquoteClojureString(value string) (string, error) {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", fmt.Errorf("expected a quoted string")
	}

	var decoded strings.Builder
	for index := 1; index < len(value)-1; index++ {
		if value[index] != '\\' {
			decoded.WriteByte(value[index])
			continue
		}
		index++
		if index >= len(value)-1 {
			return "", fmt.Errorf("unterminated escape sequence")
		}
		switch value[index] {
		case 'b':
			decoded.WriteByte('\b')
		case 'f':
			decoded.WriteByte('\f')
		case 'n':
			decoded.WriteByte('\n')
		case 'r':
			decoded.WriteByte('\r')
		case 't':
			decoded.WriteByte('\t')
		case '"', '\\':
			decoded.WriteByte(value[index])
		case 'u':
			if index+4 >= len(value)-1 {
				return "", fmt.Errorf("incomplete Unicode escape")
			}
			codePoint, err := strconv.ParseUint(value[index+1:index+5], 16, 16)
			if err != nil {
				return "", fmt.Errorf("invalid Unicode escape: %w", err)
			}
			decoded.WriteRune(rune(codePoint))
			index += 4
		default:
			return "", fmt.Errorf("unsupported escape sequence \\%c", value[index])
		}
	}
	return decoded.String(), nil
}

func clojurePunctuationToken(value byte) (clojureTokenKind, bool) {
	switch value {
	case '(':
		return clojureTokenOpenList, true
	case '[':
		return clojureTokenOpenVector, true
	case '{':
		return clojureTokenOpenMap, true
	case ')':
		return clojureTokenCloseList, true
	case ']':
		return clojureTokenCloseVector, true
	case '}':
		return clojureTokenCloseMap, true
	case '\'', '`', '@', '^', '~':
		return clojureTokenReaderMacro, true
	default:
		return 0, false
	}
}

func isClojureDelimiter(value byte) bool {
	if unicode.IsSpace(rune(value)) || value == ',' || value == ';' || value == '"' {
		return true
	}
	_, punctuation := clojurePunctuationToken(value)
	return punctuation || value == '#'
}
