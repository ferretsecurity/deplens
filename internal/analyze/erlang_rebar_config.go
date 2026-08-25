package analyze

import (
	"fmt"
	"strings"
)

type erlangRebarConfigMatcherConfig struct{}

type erlangRebarConfigParser struct{}

type erlangTermKind uint8

const (
	erlangAtom erlangTermKind = iota
	erlangString
	erlangTuple
	erlangList
)

type erlangTerm struct {
	kind   erlangTermKind
	value  string
	values []erlangTerm
}

func newErlangRebarConfigParser(erlangRebarConfigMatcherConfig) (sourceAnalyzer, error) {
	return erlangRebarConfigParser{}, nil
}

func (erlangRebarConfigParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	terms, err := parseErlangTerms(content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse rebar.config file %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, term := range terms {
		dependencies, incomplete = collectRebarConfigDependencies(term, "", dependencies, incomplete)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func collectRebarConfigDependencies(term erlangTerm, prefix string, dependencies []DependencyReference, incomplete []string) ([]DependencyReference, []string) {
	if term.kind == erlangTuple && len(term.values) >= 2 {
		switch erlangAtomValue(term.values[0]) {
		case "deps", "project_plugins":
			group := joinRebarGroup(prefix, erlangAtomValue(term.values[0]))
			values := term.values[1]
			if values.kind != erlangList {
				return dependencies, append(incomplete, group+": expected a dependency list")
			}
			for _, value := range values.values {
				dependency, ok, message := rebarConfigDependency(value, group)
				if message != "" {
					incomplete = append(incomplete, message)
				}
				if ok {
					dependencies = append(dependencies, dependency)
				}
			}
			return dependencies, incomplete
		case "profiles":
			if term.values[1].kind != erlangList {
				return dependencies, append(incomplete, "profiles: expected a profile list")
			}
			for _, profile := range term.values[1].values {
				if profile.kind != erlangTuple || len(profile.values) < 2 || !isErlangName(profile.values[0]) || profile.values[1].kind != erlangList {
					incomplete = append(incomplete, "profiles: expected {name, configuration} entries")
					continue
				}
				profilePrefix := joinRebarGroup(prefix, "profiles."+profile.values[0].value)
				for _, config := range profile.values[1].values {
					dependencies, incomplete = collectRebarConfigDependencies(config, profilePrefix, dependencies, incomplete)
				}
			}
			return dependencies, incomplete
		}
	}

	for _, child := range term.values {
		dependencies, incomplete = collectRebarConfigDependencies(child, prefix, dependencies, incomplete)
	}
	return dependencies, incomplete
}

func rebarConfigDependency(term erlangTerm, group string) (DependencyReference, bool, string) {
	if isErlangName(term) {
		return rebarRegistryDependency(term.value, "", group), true, ""
	}
	if term.kind != erlangTuple || len(term.values) == 0 || !isErlangName(term.values[0]) {
		return DependencyReference{}, false, group + ": expected an atom or dependency tuple"
	}

	declaredName := term.values[0].value
	if len(term.values) == 1 {
		return rebarRegistryDependency(declaredName, "", group), true, ""
	}
	if source, ok := rebarGitSource(term.values[1]); ok {
		return rebarGitDependency(declaredName, source, group), true, ""
	}

	version, ok := erlangTextValue(term.values[1])
	if !ok {
		return DependencyReference{}, false, fmt.Sprintf("%s.%s: unsupported dependency source", group, declaredName)
	}
	dependency := rebarRegistryDependency(declaredName, version, group)
	if len(term.values) >= 3 {
		pkg, ok := rebarPackageAlias(term.values[2])
		if !ok {
			return DependencyReference{}, false, fmt.Sprintf("%s.%s: unsupported dependency options", group, declaredName)
		}
		dependency.Name = pkg
		dependency.Attributes = map[string]string{"declared_name": declaredName}
	}
	if len(term.values) > 3 {
		return DependencyReference{}, false, fmt.Sprintf("%s.%s: unsupported dependency tuple", group, declaredName)
	}
	return dependency, true, ""
}

func rebarRegistryDependency(name, version, group string) DependencyReference {
	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        rebarDependencyScope(group),
	}
	if version != "" {
		dependency.Raw += "@" + version
		dependency.VersionConstraint = version
	}
	return dependency
}

type rebarGitDependencySource struct {
	url     string
	ref     string
	refKind string
	subdir  string
}

func rebarGitSource(term erlangTerm) (rebarGitDependencySource, bool) {
	if term.kind != erlangTuple || len(term.values) < 2 {
		return rebarGitDependencySource{}, false
	}
	kind := erlangAtomValue(term.values[0])
	if kind != "git" && kind != "git_subdir" {
		return rebarGitDependencySource{}, false
	}
	url, ok := erlangTextValue(term.values[1])
	if !ok || strings.TrimSpace(url) == "" {
		return rebarGitDependencySource{}, false
	}
	source := rebarGitDependencySource{url: url}
	if len(term.values) >= 3 {
		if term.values[2].kind == erlangTuple && len(term.values[2].values) == 2 && isErlangName(term.values[2].values[0]) {
			if ref, ok := erlangTextValue(term.values[2].values[1]); ok {
				source.refKind = term.values[2].values[0].value
				source.ref = ref
			} else {
				return rebarGitDependencySource{}, false
			}
		} else if ref, ok := erlangTextValue(term.values[2]); ok {
			source.ref = ref
		} else {
			return rebarGitDependencySource{}, false
		}
	}
	if kind == "git_subdir" {
		if len(term.values) != 4 {
			return rebarGitDependencySource{}, false
		}
		subdir, ok := erlangTextValue(term.values[3])
		if !ok || strings.TrimSpace(subdir) == "" {
			return rebarGitDependencySource{}, false
		}
		source.subdir = subdir
	} else if len(term.values) != 2 && len(term.values) != 3 {
		return rebarGitDependencySource{}, false
	}
	return source, true
}

func rebarGitDependency(name string, source rebarGitDependencySource, group string) DependencyReference {
	attributes := map[string]string{"source_url": source.url}
	if source.ref != "" {
		attributes["source_ref"] = source.ref
	}
	if source.refKind != "" {
		attributes["source_ref_kind"] = source.refKind
	}
	if source.subdir != "" {
		attributes["subdir"] = source.subdir
	}
	return DependencyReference{
		Raw:          name + "@" + source.url,
		Name:         name,
		SourceGroup:  group,
		OriginKind:   OriginGit,
		Relationship: RelationshipDirect,
		Scope:        rebarDependencyScope(group),
		Attributes:   attributes,
	}
}

func rebarPackageAlias(term erlangTerm) (string, bool) {
	if term.kind != erlangTuple || len(term.values) != 2 || erlangAtomValue(term.values[0]) != "pkg" {
		return "", false
	}
	return erlangTextValue(term.values[1])
}

func rebarDependencyScope(group string) DependencyScope {
	if strings.Contains(group, ".profiles.test.") || strings.HasPrefix(group, "profiles.test.") {
		return ScopeTest
	}
	if strings.HasSuffix(group, "project_plugins") {
		return ScopeBuild
	}
	return ScopeRuntime
}

func joinRebarGroup(prefix, group string) string {
	if prefix == "" {
		return group
	}
	return prefix + "." + group
}

func isErlangName(term erlangTerm) bool {
	return term.kind == erlangAtom || term.kind == erlangString
}

func erlangAtomValue(term erlangTerm) string {
	if term.kind != erlangAtom {
		return ""
	}
	return term.value
}

func erlangTextValue(term erlangTerm) (string, bool) {
	if !isErlangName(term) {
		return "", false
	}
	return term.value, true
}

type erlangTermParser struct {
	content []byte
	pos     int
}

func parseErlangTerms(content []byte) ([]erlangTerm, error) {
	parser := erlangTermParser{content: content}
	terms := make([]erlangTerm, 0)
	for {
		parser.skipIgnored()
		if parser.pos == len(parser.content) {
			return terms, nil
		}
		term, err := parser.parseTerm()
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
		parser.skipIgnored()
		if parser.pos == len(parser.content) {
			return terms, nil
		}
		if parser.content[parser.pos] != '.' {
			return nil, parser.errorf("expected '.' after term")
		}
		parser.pos++
	}
}

func (p *erlangTermParser) parseTerm() (erlangTerm, error) {
	p.skipIgnored()
	if p.pos == len(p.content) {
		return erlangTerm{}, p.errorf("expected term")
	}
	switch p.content[p.pos] {
	case '{':
		return p.parseCollection('{', '}', erlangTuple)
	case '[':
		return p.parseCollection('[', ']', erlangList)
	case '"':
		value, err := p.parseQuoted('"')
		return erlangTerm{kind: erlangString, value: value}, err
	case '\'':
		value, err := p.parseQuoted('\'')
		return erlangTerm{kind: erlangAtom, value: value}, err
	default:
		start := p.pos
		for p.pos < len(p.content) && !strings.ContainsRune("{}[],.%\"'", rune(p.content[p.pos])) && !isErlangSpace(p.content[p.pos]) {
			p.pos++
		}
		if start == p.pos {
			return erlangTerm{}, p.errorf("unexpected character %q", p.content[p.pos])
		}
		return erlangTerm{kind: erlangAtom, value: string(p.content[start:p.pos])}, nil
	}
}

func (p *erlangTermParser) parseCollection(open, close byte, kind erlangTermKind) (erlangTerm, error) {
	p.pos++
	values := make([]erlangTerm, 0)
	for {
		p.skipIgnored()
		if p.pos == len(p.content) {
			return erlangTerm{}, p.errorf("unterminated collection starting with %q", open)
		}
		if p.content[p.pos] == close {
			p.pos++
			return erlangTerm{kind: kind, values: values}, nil
		}
		if p.content[p.pos] == '|' && kind == erlangList {
			p.pos++
			p.skipIgnored()
			value, err := p.parseTerm()
			if err != nil {
				return erlangTerm{}, err
			}
			values = append(values, value)
			p.skipIgnored()
			if p.pos == len(p.content) || p.content[p.pos] != close {
				return erlangTerm{}, p.errorf("expected %q after list tail", close)
			}
			p.pos++
			return erlangTerm{kind: kind, values: values}, nil
		}
		value, err := p.parseTerm()
		if err != nil {
			return erlangTerm{}, err
		}
		values = append(values, value)
		p.skipIgnored()
		if p.pos == len(p.content) {
			return erlangTerm{}, p.errorf("unterminated collection starting with %q", open)
		}
		if p.content[p.pos] == close {
			continue
		}
		if p.content[p.pos] != ',' {
			return erlangTerm{}, p.errorf("expected ',' or %q", close)
		}
		p.pos++
	}
}

func (p *erlangTermParser) parseQuoted(quote byte) (string, error) {
	p.pos++
	var value strings.Builder
	for p.pos < len(p.content) {
		character := p.content[p.pos]
		p.pos++
		if character == quote {
			return value.String(), nil
		}
		if character != '\\' {
			value.WriteByte(character)
			continue
		}
		if p.pos == len(p.content) {
			return "", p.errorf("unterminated escape sequence")
		}
		escaped := p.content[p.pos]
		p.pos++
		switch escaped {
		case 'n':
			value.WriteByte('\n')
		case 'r':
			value.WriteByte('\r')
		case 't':
			value.WriteByte('\t')
		default:
			value.WriteByte(escaped)
		}
	}
	return "", p.errorf("unterminated quoted value")
}

func (p *erlangTermParser) skipIgnored() {
	for p.pos < len(p.content) {
		if isErlangSpace(p.content[p.pos]) {
			p.pos++
			continue
		}
		if p.content[p.pos] == '%' {
			for p.pos < len(p.content) && p.content[p.pos] != '\n' {
				p.pos++
			}
			continue
		}
		return
	}
}

func (p *erlangTermParser) errorf(format string, args ...any) error {
	return fmt.Errorf("byte %d: %s", p.pos, fmt.Sprintf(format, args...))
}

func isErlangSpace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\n' || character == '\r'
}
