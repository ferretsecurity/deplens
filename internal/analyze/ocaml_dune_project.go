package analyze

import (
	"fmt"
	"strings"
)

type ocamlDuneProjectParserConfig struct{}

type ocamlDuneProjectParser struct{}

type duneProjectTerm struct {
	atom     string
	children []duneProjectTerm
}

func newOCamlDuneProjectParser(ocamlDuneProjectParserConfig) (sourceAnalyzer, error) {
	return ocamlDuneProjectParser{}, nil
}

func (ocamlDuneProjectParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	forms, err := parseDuneProjectForms(content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Dune project %q: %w", path, err)
	}

	recognized := false
	dependencies := make([]DependencyReference, 0)
	for _, form := range forms {
		if duneProjectFormHead(form) == "lang" && len(form.children) >= 3 && duneProjectTermAtom(form.children[1]) == "dune" {
			recognized = true
		}
		if duneProjectFormHead(form) == "package" {
			dependencies = append(dependencies, duneProjectPackageDependencies(form)...)
		}
	}
	if !recognized {
		return sourceAnalyzerResult{}, nil
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func duneProjectPackageDependencies(form duneProjectTerm) []DependencyReference {
	name := ""
	for _, field := range form.children[1:] {
		if duneProjectFormHead(field) == "name" && len(field.children) >= 2 {
			name = duneProjectTermAtom(field.children[1])
			break
		}
	}
	if name == "" {
		return nil
	}

	dependencies := make([]DependencyReference, 0)
	for _, field := range form.children[1:] {
		if duneProjectFormHead(field) != "depends" {
			continue
		}
		group := "package." + name + ".depends"
		for _, declaration := range field.children[1:] {
			if dependency, ok := duneProjectDependency(declaration, group); ok {
				dependencies = append(dependencies, dependency)
			}
		}
	}
	return dependencies
}

func duneProjectDependency(declaration duneProjectTerm, group string) (DependencyReference, bool) {
	name := duneProjectTermAtom(declaration)
	conditions := []duneProjectTerm(nil)
	if name == "" && len(declaration.children) > 0 {
		name = duneProjectTermAtom(declaration.children[0])
		conditions = declaration.children[1:]
	}
	if name == "" || strings.HasPrefix(name, ":") {
		return DependencyReference{}, false
	}

	constraint := strings.Join(duneProjectVersionConstraints(conditions), " && ")
	dependency := DependencyReference{
		PackageType:  "opam",
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if duneProjectHasTestFilter(conditions) {
		dependency.Scope = ScopeTest
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency, true
}

func duneProjectVersionConstraints(terms []duneProjectTerm) []string {
	constraints := make([]string, 0)
	for _, term := range terms {
		if len(term.children) > 1 && isDuneProjectVersionOperator(duneProjectTermAtom(term.children[0])) {
			values := make([]string, 0, len(term.children)-1)
			for _, value := range term.children[1:] {
				if text := duneProjectTermText(value); text != "" {
					values = append(values, text)
				}
			}
			if len(values) > 0 {
				constraints = append(constraints, duneProjectTermAtom(term.children[0])+" "+strings.Join(values, " "))
			}
			continue
		}
		constraints = append(constraints, duneProjectVersionConstraints(term.children)...)
	}
	return constraints
}

func isDuneProjectVersionOperator(value string) bool {
	switch value {
	case "=", "!=", ">", ">=", "<", "<=":
		return true
	default:
		return false
	}
}

func duneProjectHasTestFilter(terms []duneProjectTerm) bool {
	for _, term := range terms {
		if duneProjectTermAtom(term) == ":with-test" || duneProjectHasTestFilter(term.children) {
			return true
		}
	}
	return false
}

func duneProjectFormHead(term duneProjectTerm) string {
	if len(term.children) == 0 {
		return ""
	}
	return duneProjectTermAtom(term.children[0])
}

func duneProjectTermAtom(term duneProjectTerm) string {
	if len(term.children) != 0 {
		return ""
	}
	return term.atom
}

func duneProjectTermText(term duneProjectTerm) string {
	if atom := duneProjectTermAtom(term); atom != "" {
		return atom
	}
	if len(term.children) == 0 {
		return ""
	}
	values := make([]string, 0, len(term.children))
	for _, child := range term.children {
		if value := duneProjectTermText(child); value != "" {
			values = append(values, value)
		}
	}
	return "(" + strings.Join(values, " ") + ")"
}

func parseDuneProjectForms(content []byte) ([]duneProjectTerm, error) {
	parser := duneProjectFormParser{content: content}
	forms := make([]duneProjectTerm, 0)
	for {
		parser.skipSpaceAndComments()
		if parser.offset == len(parser.content) {
			return forms, nil
		}
		term, err := parser.parseTerm()
		if err != nil {
			return nil, err
		}
		forms = append(forms, term)
	}
}

type duneProjectFormParser struct {
	content []byte
	offset  int
}

func (p *duneProjectFormParser) parseTerm() (duneProjectTerm, error) {
	p.skipSpaceAndComments()
	if p.offset == len(p.content) {
		return duneProjectTerm{}, fmt.Errorf("unexpected end of file")
	}
	if p.content[p.offset] == '(' {
		p.offset++
		term := duneProjectTerm{children: make([]duneProjectTerm, 0)}
		for {
			p.skipSpaceAndComments()
			if p.offset == len(p.content) {
				return duneProjectTerm{}, fmt.Errorf("unclosed list")
			}
			if p.content[p.offset] == ')' {
				p.offset++
				return term, nil
			}
			child, err := p.parseTerm()
			if err != nil {
				return duneProjectTerm{}, err
			}
			term.children = append(term.children, child)
		}
	}
	if p.content[p.offset] == ')' {
		return duneProjectTerm{}, fmt.Errorf("unexpected closing parenthesis")
	}
	if p.content[p.offset] == '"' {
		return p.parseString()
	}
	start := p.offset
	for p.offset < len(p.content) && !isDuneProjectSpace(p.content[p.offset]) && p.content[p.offset] != '(' && p.content[p.offset] != ')' && p.content[p.offset] != ';' {
		p.offset++
	}
	if start == p.offset {
		return duneProjectTerm{}, fmt.Errorf("expected atom")
	}
	return duneProjectTerm{atom: string(p.content[start:p.offset])}, nil
}

func (p *duneProjectFormParser) parseString() (duneProjectTerm, error) {
	p.offset++
	var value strings.Builder
	for p.offset < len(p.content) {
		character := p.content[p.offset]
		p.offset++
		if character == '"' {
			return duneProjectTerm{atom: value.String()}, nil
		}
		if character == '\\' {
			if p.offset == len(p.content) {
				return duneProjectTerm{}, fmt.Errorf("unfinished string escape")
			}
			character = p.content[p.offset]
			p.offset++
		}
		value.WriteByte(character)
	}
	return duneProjectTerm{}, fmt.Errorf("unclosed string")
}

func (p *duneProjectFormParser) skipSpaceAndComments() {
	for p.offset < len(p.content) {
		if isDuneProjectSpace(p.content[p.offset]) {
			p.offset++
			continue
		}
		if p.content[p.offset] != ';' {
			return
		}
		for p.offset < len(p.content) && p.content[p.offset] != '\n' {
			p.offset++
		}
	}
}

func isDuneProjectSpace(character byte) bool {
	switch character {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}
