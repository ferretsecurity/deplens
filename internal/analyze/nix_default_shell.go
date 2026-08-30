package analyze

import (
	"net/url"
	"strings"
)

type nixDefaultShellMatcherConfig struct{}

type nixDefaultShellParser struct{}

type nixTokenKind uint8

const (
	nixTokenOther nixTokenKind = iota
	nixTokenIdentifier
	nixTokenString
	nixTokenSearchPath
	nixTokenOpenBrace
	nixTokenCloseBrace
	nixTokenDot
	nixTokenEquals
	nixTokenSemicolon
)

type nixToken struct {
	kind  nixTokenKind
	value string
}

func newNixDefaultShellParser(nixDefaultShellMatcherConfig) (sourceAnalyzer, error) {
	return nixDefaultShellParser{}, nil
}

func (nixDefaultShellParser) Analyze(_ string, content []byte) (sourceAnalyzerResult, error) {
	tokens := lexNix(content)
	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})

	for index, token := range tokens {
		switch token.kind {
		case nixTokenSearchPath:
			dependency := nixSearchPathDependency(token.value)
			dependencies = appendUniqueDependency(dependencies, seen, dependency.Raw, dependency)
		case nixTokenIdentifier:
			if token.value != "fetchTarball" {
				continue
			}
			if dependency, ok := nixFetchTarballDependency(tokens[index+1:]); ok {
				dependencies = appendUniqueDependency(dependencies, seen, dependency.Raw, dependency)
			}
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func nixSearchPathDependency(name string) DependencyReference {
	return DependencyReference{
		PackageType:  "generic",
		Raw:          "<" + name + ">",
		Name:         name,
		SourceGroup:  "nix-path",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
}

func nixFetchTarballDependency(tokens []nixToken) (DependencyReference, bool) {
	if len(tokens) == 0 || tokens[0].kind != nixTokenOpenBrace {
		return DependencyReference{}, false
	}

	depth := 0
	for index := range tokens {
		switch tokens[index].kind {
		case nixTokenOpenBrace:
			depth++
		case nixTokenCloseBrace:
			depth--
			if depth == 0 {
				return DependencyReference{}, false
			}
		}
		if depth != 1 || tokens[index].kind != nixTokenIdentifier || tokens[index].value != "url" {
			continue
		}
		if index+2 < len(tokens) && tokens[index+1].kind == nixTokenEquals && tokens[index+2].kind == nixTokenString {
			return nixTarballURLDependency(tokens[index+2].value)
		}
	}
	return DependencyReference{}, false
}

func nixTarballURLDependency(source string) (DependencyReference, bool) {
	parsed, err := url.Parse(source)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return DependencyReference{}, false
	}

	name := source
	packageType := PackageType("generic")
	origin := OriginURL
	attributes := map[string]string{"source_url": source}
	if strings.EqualFold(parsed.Host, "github.com") {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			name = parts[0] + "/" + parts[1]
			packageType = "github"
			origin = OriginGit
			attributes["source_url"] = "https://github.com/" + name
		}
	}

	return DependencyReference{
		PackageType:  packageType,
		Raw:          source,
		Name:         name,
		SourceGroup:  "fetchTarball",
		OriginKind:   origin,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
		Attributes:   attributes,
	}, true
}

func lexNix(content []byte) []nixToken {
	tokens := make([]nixToken, 0)
	for index := 0; index < len(content); {
		if isNixWhitespace(content[index]) {
			index++
			continue
		}
		if content[index] == '#' {
			for index < len(content) && content[index] != '\n' {
				index++
			}
			continue
		}
		if index+1 < len(content) && content[index] == '/' && content[index+1] == '*' {
			index = skipNixBlockComment(content, index+2)
			continue
		}

		switch content[index] {
		case '{':
			tokens = append(tokens, nixToken{kind: nixTokenOpenBrace})
			index++
		case '}':
			tokens = append(tokens, nixToken{kind: nixTokenCloseBrace})
			index++
		case '.':
			tokens = append(tokens, nixToken{kind: nixTokenDot})
			index++
		case '=':
			tokens = append(tokens, nixToken{kind: nixTokenEquals})
			index++
		case ';':
			tokens = append(tokens, nixToken{kind: nixTokenSemicolon})
			index++
		case '"':
			value, next := lexNixString(content, index+1)
			tokens = append(tokens, nixToken{kind: nixTokenString, value: value})
			index = next
		case '<':
			if end := strings.IndexByte(string(content[index+1:]), '>'); end >= 0 {
				value := string(content[index+1 : index+1+end])
				if value != "" && !strings.ContainsAny(value, " \t\r\n") {
					tokens = append(tokens, nixToken{kind: nixTokenSearchPath, value: value})
					index += end + 2
					continue
				}
			}
			tokens = append(tokens, nixToken{kind: nixTokenOther})
			index++
		default:
			if isNixIdentifierStart(content[index]) {
				end := index + 1
				for end < len(content) && isNixIdentifierPart(content[end]) {
					end++
				}
				tokens = append(tokens, nixToken{kind: nixTokenIdentifier, value: string(content[index:end])})
				index = end
				continue
			}
			tokens = append(tokens, nixToken{kind: nixTokenOther})
			index++
		}
	}
	return tokens
}

func isNixWhitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func skipNixBlockComment(content []byte, index int) int {
	for index+1 < len(content) {
		if content[index] == '*' && content[index+1] == '/' {
			return index + 2
		}
		index++
	}
	return len(content)
}

func lexNixString(content []byte, index int) (string, int) {
	var value strings.Builder
	for index < len(content) {
		if content[index] == '\\' && index+1 < len(content) {
			value.WriteByte(content[index])
			value.WriteByte(content[index+1])
			index += 2
			continue
		}
		if content[index] == '"' {
			return value.String(), index + 1
		}
		value.WriteByte(content[index])
		index++
	}
	return value.String(), index
}

func isNixIdentifierStart(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func isNixIdentifierPart(value byte) bool {
	return isNixIdentifierStart(value) || value >= '0' && value <= '9' || value == '-' || value == '\''
}
