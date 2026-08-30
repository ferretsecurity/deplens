package analyze

import (
	"fmt"
	"strconv"
	"strings"
)

type zigBuildZONParser struct{}

func newZigBuildZONParser(zigBuildZONMatcherConfig) (sourceAnalyzer, error) {
	return zigBuildZONParser{}, nil
}

func (zigBuildZONParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	root, err := parseZON(content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Zig build manifest %q: %w", path, err)
	}
	if root.kind != zonObject || root.fields["name"].kind == zonInvalid {
		return sourceAnalyzerResult{}, nil
	}

	dependenciesValue, exists := root.fields["dependencies"]
	if !exists {
		return semanticAnalyzerResult([]DependencyReference{}, nil), nil
	}
	if dependenciesValue.kind != zonObject {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Zig build manifest %q: dependencies must be an object", path)
	}

	dependencies := make([]DependencyReference, 0, len(dependenciesValue.fields))
	incomplete := make([]string, 0)
	for _, declaredName := range dependenciesValue.order {
		dependency, message := zigBuildZONDependency(declaredName, dependenciesValue.fields[declaredName])
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func zigBuildZONDependency(name string, value zonValue) (DependencyReference, string) {
	if value.kind != zonObject {
		return DependencyReference{}, fmt.Sprintf("dependencies.%s: expected an object", name)
	}

	dependency := DependencyReference{
		Name:         name,
		SourceGroup:  "dependencies",
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if path, ok := zonStringField(value, "path"); ok {
		dependency.Raw = name + "@" + path
		dependency.OriginKind = OriginPath
		dependency.Attributes = map[string]string{"path": path}
		return dependency, ""
	}
	if sourceURL, ok := zonStringField(value, "url"); ok {
		dependency.Raw = name + "@" + sourceURL
		dependency.OriginKind = OriginURL
		dependency.Attributes = map[string]string{"source_url": sourceURL}
		if strings.HasPrefix(sourceURL, "git+") {
			dependency.OriginKind = OriginGit
			dependency.Attributes["source_url"] = strings.TrimPrefix(sourceURL, "git+")
			if url, revision, found := strings.Cut(dependency.Attributes["source_url"], "#"); found {
				dependency.Attributes["source_url"] = url
				dependency.Attributes["source_ref"] = revision
				dependency.Attributes["source_ref_kind"] = "revision"
			}
		}
		return dependency, ""
	}
	return DependencyReference{}, fmt.Sprintf("dependencies.%s: expected a string url or path", name)
}

func zonStringField(value zonValue, field string) (string, bool) {
	child, exists := value.fields[field]
	return child.text, exists && child.kind == zonString && child.text != ""
}

type zonValueKind uint8

const (
	zonInvalid zonValueKind = iota
	zonAtom
	zonString
	zonObject
	zonList
)

type zonValue struct {
	kind   zonValueKind
	text   string
	fields map[string]zonValue
	order  []string
}

type zonTokenKind uint8

const (
	zonTokenEOF zonTokenKind = iota
	zonTokenDot
	zonTokenOpenBrace
	zonTokenCloseBrace
	zonTokenEqual
	zonTokenComma
	zonTokenAtom
	zonTokenString
)

type zonToken struct {
	kind zonTokenKind
	text string
}

type zonParser struct {
	content string
	offset  int
	current zonToken
}

func parseZON(content []byte) (zonValue, error) {
	parser := zonParser{content: string(content)}
	if err := parser.advance(); err != nil {
		return zonValue{}, err
	}
	root, err := parser.parseValue()
	if err != nil {
		return zonValue{}, err
	}
	if parser.current.kind != zonTokenEOF {
		return zonValue{}, fmt.Errorf("unexpected trailing content")
	}
	return root, nil
}

func (p *zonParser) parseValue() (zonValue, error) {
	switch p.current.kind {
	case zonTokenString:
		value := zonValue{kind: zonString, text: p.current.text}
		return value, p.advance()
	case zonTokenAtom:
		value := zonValue{kind: zonAtom, text: p.current.text}
		return value, p.advance()
	case zonTokenDot:
		if err := p.advance(); err != nil {
			return zonValue{}, err
		}
		if p.current.kind == zonTokenOpenBrace {
			return p.parseComposite()
		}
		if p.current.kind != zonTokenAtom && p.current.kind != zonTokenString {
			return zonValue{}, fmt.Errorf("expected an enum literal or object after dot")
		}
		value := zonValue{kind: zonAtom, text: p.current.text}
		return value, p.advance()
	default:
		return zonValue{}, fmt.Errorf("expected a value")
	}
}

func (p *zonParser) parseComposite() (zonValue, error) {
	lookahead := *p
	if err := lookahead.advance(); err != nil {
		return zonValue{}, err
	}
	if lookahead.current.kind == zonTokenCloseBrace {
		return p.parseObject()
	}
	if lookahead.current.kind == zonTokenDot {
		if err := lookahead.advance(); err != nil {
			return zonValue{}, err
		}
		if lookahead.current.kind == zonTokenAtom || lookahead.current.kind == zonTokenString {
			if err := lookahead.advance(); err != nil {
				return zonValue{}, err
			}
			if lookahead.current.kind == zonTokenEqual {
				return p.parseObject()
			}
		}
	}
	return p.parseList()
}

func (p *zonParser) parseObject() (zonValue, error) {
	if p.current.kind != zonTokenOpenBrace {
		return zonValue{}, fmt.Errorf("expected an object")
	}
	if err := p.advance(); err != nil {
		return zonValue{}, err
	}
	value := zonValue{kind: zonObject, fields: make(map[string]zonValue)}
	for p.current.kind != zonTokenCloseBrace {
		if p.current.kind == zonTokenEOF {
			return zonValue{}, fmt.Errorf("unterminated object")
		}
		if p.current.kind != zonTokenDot {
			return zonValue{}, fmt.Errorf("expected an object field")
		}
		if err := p.advance(); err != nil {
			return zonValue{}, err
		}
		if p.current.kind != zonTokenAtom && p.current.kind != zonTokenString {
			return zonValue{}, fmt.Errorf("expected an object field name")
		}
		name := p.current.text
		if err := p.advance(); err != nil {
			return zonValue{}, err
		}
		if p.current.kind != zonTokenEqual {
			return zonValue{}, fmt.Errorf("expected equals after field %q", name)
		}
		if err := p.advance(); err != nil {
			return zonValue{}, err
		}
		fieldValue, err := p.parseValue()
		if err != nil {
			return zonValue{}, fmt.Errorf("field %q: %w", name, err)
		}
		if _, exists := value.fields[name]; !exists {
			value.order = append(value.order, name)
		}
		value.fields[name] = fieldValue
		if p.current.kind == zonTokenComma {
			if err := p.advance(); err != nil {
				return zonValue{}, err
			}
			continue
		}
		if p.current.kind != zonTokenCloseBrace {
			return zonValue{}, fmt.Errorf("expected a comma or closing brace after field %q", name)
		}
	}
	return value, p.advance()
}

func (p *zonParser) parseList() (zonValue, error) {
	if p.current.kind != zonTokenOpenBrace {
		return zonValue{}, fmt.Errorf("expected a list")
	}
	if err := p.advance(); err != nil {
		return zonValue{}, err
	}
	for p.current.kind != zonTokenCloseBrace {
		if p.current.kind == zonTokenEOF {
			return zonValue{}, fmt.Errorf("unterminated list")
		}
		if _, err := p.parseValue(); err != nil {
			return zonValue{}, err
		}
		if p.current.kind == zonTokenComma {
			if err := p.advance(); err != nil {
				return zonValue{}, err
			}
			continue
		}
		if p.current.kind != zonTokenCloseBrace {
			return zonValue{}, fmt.Errorf("expected a comma or closing brace in list")
		}
	}
	return zonValue{kind: zonList}, p.advance()
}

func (p *zonParser) advance() error {
	for p.offset < len(p.content) {
		if strings.ContainsRune(" \t\r\n", rune(p.content[p.offset])) {
			p.offset++
			continue
		}
		if strings.HasPrefix(p.content[p.offset:], "//") {
			p.offset += 2
			for p.offset < len(p.content) && p.content[p.offset] != '\n' {
				p.offset++
			}
			continue
		}
		break
	}
	if p.offset == len(p.content) {
		p.current = zonToken{kind: zonTokenEOF}
		return nil
	}

	character := p.content[p.offset]
	switch character {
	case '.':
		p.offset++
		p.current = zonToken{kind: zonTokenDot}
	case '{':
		p.offset++
		p.current = zonToken{kind: zonTokenOpenBrace}
	case '}':
		p.offset++
		p.current = zonToken{kind: zonTokenCloseBrace}
	case '=':
		p.offset++
		p.current = zonToken{kind: zonTokenEqual}
	case ',':
		p.offset++
		p.current = zonToken{kind: zonTokenComma}
	case '"':
		return p.readString(false)
	case '@':
		if p.offset+1 < len(p.content) && p.content[p.offset+1] == '"' {
			p.offset++
			return p.readString(true)
		}
		fallthrough
	default:
		start := p.offset
		for p.offset < len(p.content) && !strings.ContainsRune(" \t\r\n.{}=,\"", rune(p.content[p.offset])) {
			p.offset++
		}
		if start == p.offset {
			return fmt.Errorf("unexpected character %q", character)
		}
		p.current = zonToken{kind: zonTokenAtom, text: p.content[start:p.offset]}
	}
	return nil
}

func (p *zonParser) readString(identifier bool) error {
	start := p.offset
	p.offset++
	for p.offset < len(p.content) {
		if p.content[p.offset] == '\\' {
			p.offset += 2
			continue
		}
		if p.content[p.offset] == '"' {
			p.offset++
			value, err := strconv.Unquote(p.content[start:p.offset])
			if err != nil {
				return fmt.Errorf("decode string: %w", err)
			}
			kind := zonTokenString
			if identifier {
				kind = zonTokenAtom
			}
			p.current = zonToken{kind: kind, text: value}
			return nil
		}
		p.offset++
	}
	return fmt.Errorf("unterminated string")
}
