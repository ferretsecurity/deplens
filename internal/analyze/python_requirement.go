package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

var pythonRequirementPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(?:\[[A-Za-z0-9._-]+(?:\s*,\s*[A-Za-z0-9._-]+)*\])?(?:\s*(?:===|==|!=|~=|<=|>=|<|>)\s*[^,;\s]+(?:\s*,\s*(?:===|==|!=|~=|<=|>=|<|>)\s*[^,;\s]+)*)?(?:\s*;\s*[^\r\n]+)?$`)

func parsePythonRequirement(raw string) (DependencyReference, error) {
	spec := strings.TrimSpace(raw)
	if spec == "" || strings.HasPrefix(spec, "-") || strings.HasPrefix(spec, "@") || strings.Contains(spec, " @ ") || strings.Contains(spec, "://") || strings.Contains(spec, "${") || strings.Contains(spec, "{{") || strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "/") || !pythonRequirementPattern.MatchString(spec) {
		return DependencyReference{}, fmt.Errorf("unsupported or malformed Python requirement")
	}
	parsed := parsePEP508Dep(spec)
	if parsed.name == "" {
		return DependencyReference{}, fmt.Errorf("unsupported or malformed Python requirement")
	}
	if marker := parsed.attributes["marker"]; marker != "" {
		if err := validatePEP508Marker(marker); err != nil {
			return DependencyReference{}, fmt.Errorf("unsupported or malformed Python requirement: invalid environment marker")
		}
	}
	return DependencyReference{PackageType: "pypi", Raw: spec, Name: parsed.name, VersionConstraint: parsed.versionConstraint, OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: parsed.attributes}, nil
}

var pep508MarkerVariables = map[string]struct{}{
	"implementation_name": {}, "implementation_version": {}, "os_name": {},
	"platform_machine": {}, "platform_python_implementation": {}, "platform_release": {},
	"platform_system": {}, "platform_version": {}, "python_full_version": {},
	"python_version": {}, "sys_platform": {}, "extra": {}, "extras": {},
	"dependency_groups": {},
}

type pep508MarkerParser struct {
	input string
	pos   int
}

func validatePEP508Marker(marker string) error {
	p := pep508MarkerParser{input: marker}
	if !p.expression() {
		return fmt.Errorf("invalid marker expression")
	}
	p.space()
	if p.pos != len(p.input) {
		return fmt.Errorf("unexpected marker input")
	}
	return nil
}

func (p *pep508MarkerParser) expression() bool {
	if !p.andExpression() {
		return false
	}
	for p.keyword("or") {
		if !p.andExpression() {
			return false
		}
	}
	return true
}

func (p *pep508MarkerParser) andExpression() bool {
	if !p.term() {
		return false
	}
	for p.keyword("and") {
		if !p.term() {
			return false
		}
	}
	return true
}

func (p *pep508MarkerParser) term() bool {
	p.space()
	if p.take("(") {
		if !p.expression() {
			return false
		}
		p.space()
		return p.take(")")
	}
	return p.comparison()
}

func (p *pep508MarkerParser) comparison() bool {
	if !p.value() || !p.operator() {
		return false
	}
	return p.value()
}

func (p *pep508MarkerParser) value() bool {
	p.space()
	if p.pos >= len(p.input) {
		return false
	}
	if p.input[p.pos] == '\'' || p.input[p.pos] == '"' {
		quote := p.input[p.pos]
		p.pos++
		for p.pos < len(p.input) {
			if p.input[p.pos] == '\\' && p.pos+1 < len(p.input) {
				p.pos += 2
				continue
			}
			if p.input[p.pos] == quote {
				p.pos++
				return true
			}
			p.pos++
		}
		return false
	}
	start := p.pos
	for p.pos < len(p.input) && (p.input[p.pos] == '_' || p.input[p.pos] >= 'a' && p.input[p.pos] <= 'z') {
		p.pos++
	}
	_, ok := pep508MarkerVariables[p.input[start:p.pos]]
	return ok
}

func (p *pep508MarkerParser) operator() bool {
	p.space()
	for _, operator := range []string{"===", "~=", "==", "!=", "<=", ">=", "<", ">"} {
		if p.take(operator) {
			return true
		}
	}
	if p.keyword("in") {
		return true
	}
	start := p.pos
	if p.keyword("not") && p.keyword("in") {
		return true
	}
	p.pos = start
	return false
}

func (p *pep508MarkerParser) keyword(keyword string) bool {
	p.space()
	end := p.pos + len(keyword)
	if end > len(p.input) || p.input[p.pos:end] != keyword {
		return false
	}
	if end < len(p.input) && (p.input[end] == '_' || p.input[end] >= 'a' && p.input[end] <= 'z') {
		return false
	}
	p.pos = end
	return true
}

func (p *pep508MarkerParser) take(value string) bool {
	if strings.HasPrefix(p.input[p.pos:], value) {
		p.pos += len(value)
		return true
	}
	return false
}

func (p *pep508MarkerParser) space() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t') {
		p.pos++
	}
}
