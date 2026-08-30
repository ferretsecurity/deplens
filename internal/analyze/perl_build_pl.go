package analyze

import (
	"regexp"
	"strings"
)

type perlBuildPLMatcherConfig struct{}

type perlBuildPLParser struct{}

type perlBuildPLPrerequisiteGroup struct {
	name  string
	scope DependencyScope
}

var (
	perlBuildPLUsesModuleBuild = regexp.MustCompile(`(?m)^\s*use\s+Module::Build\b`)
	perlBuildPLNewCall         = regexp.MustCompile(`->\s*new\s*\(`)
	perlBuildPLHashAssignment  = regexp.MustCompile(`(?m)(?:\b(?:my|our|local)\s+)?%([A-Za-z_][A-Za-z0-9_]*)\s*=\s*\(`)
	perlBuildPLHashReference   = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)`)
	perlBuildPLGroup           = regexp.MustCompile(`(?m)(?:["']([^"']+)["']|([A-Za-z_][A-Za-z0-9_]*))\s*=>\s*\{`)
	perlBuildPLPrerequisite    = regexp.MustCompile(`(?m)(?:["']([^"']+)["']|([A-Za-z_][A-Za-z0-9_:]*))\s*=>\s*(?:["']([^"']*)["']|([vV]?\d[A-Za-z0-9._+\-]*))\s*,?`)
)

var perlBuildPLPrerequisiteGroups = []perlBuildPLPrerequisiteGroup{
	{name: "requires", scope: ScopeRuntime},
	{name: "recommends", scope: ScopeOptional},
	{name: "build_requires", scope: ScopeBuild},
	{name: "configure_requires", scope: ScopeBuild},
	{name: "test_requires", scope: ScopeTest},
}

func newPerlBuildPLParser(perlBuildPLMatcherConfig) (sourceAnalyzer, error) {
	return perlBuildPLParser{}, nil
}

func (perlBuildPLParser) Analyze(_ string, content []byte) (sourceAnalyzerResult, error) {
	cleaned := perlBuildPLWithoutComments(string(content))
	if !perlBuildPLUsesModuleBuild.MatchString(cleaned) || !perlBuildPLNewCall.MatchString(cleaned) {
		return sourceAnalyzerResult{}, nil
	}

	argumentMaps := perlBuildPLArgumentMaps(cleaned)
	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})
	for _, argumentMap := range argumentMaps {
		for _, group := range perlBuildPLPrerequisiteGroups {
			for _, prerequisites := range perlBuildPLPrerequisiteMaps(argumentMap, group.name) {
				for _, match := range perlBuildPLPrerequisite.FindAllStringSubmatchIndex(prerequisites, -1) {
					name := firstNonEmpty(perlBuildPLSubmatch(prerequisites, match, 1), perlBuildPLSubmatch(prerequisites, match, 2))
					constraint := firstNonEmpty(perlBuildPLSubmatch(prerequisites, match, 3), perlBuildPLSubmatch(prerequisites, match, 4))
					if name == "" {
						continue
					}
					dependency := perlBuildPLDependency(name, constraint, group)
					key := dependency.SourceGroup + "\x00" + dependency.Name + "\x00" + dependency.VersionConstraint
					dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
				}
			}
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func perlBuildPLArgumentMaps(content string) []string {
	assignments := make(map[string]string)
	for _, match := range perlBuildPLHashAssignment.FindAllStringSubmatchIndex(content, -1) {
		if body, ok := perlBuildPLDelimitedBody(content, match[1]-1, '(', ')'); ok {
			assignments[perlBuildPLSubmatch(content, match, 1)] = body
		}
	}

	maps := make([]string, 0)
	for _, match := range perlBuildPLNewCall.FindAllStringIndex(content, -1) {
		arguments, ok := perlBuildPLDelimitedBody(content, match[1]-1, '(', ')')
		if !ok {
			continue
		}
		maps = append(maps, arguments)
		for _, reference := range perlBuildPLHashReference.FindAllStringSubmatch(arguments, -1) {
			if assigned, exists := assignments[reference[1]]; exists {
				maps = append(maps, assigned)
			}
		}
	}
	return maps
}

func perlBuildPLPrerequisiteMaps(argumentMap, group string) []string {
	maps := make([]string, 0)
	for _, match := range perlBuildPLGroup.FindAllStringSubmatchIndex(argumentMap, -1) {
		name := firstNonEmpty(perlBuildPLSubmatch(argumentMap, match, 1), perlBuildPLSubmatch(argumentMap, match, 2))
		if name != group {
			continue
		}
		if body, ok := perlBuildPLDelimitedBody(argumentMap, match[1]-1, '{', '}'); ok {
			maps = append(maps, body)
		}
	}
	return maps
}

func perlBuildPLSubmatch(content string, match []int, index int) string {
	start, end := match[index*2], match[index*2+1]
	if start < 0 || end < 0 {
		return ""
	}
	return content[start:end]
}

func perlBuildPLDelimitedBody(content string, open int, opening, closing byte) (string, bool) {
	if open < 0 || open >= len(content) || content[open] != opening {
		return "", false
	}
	depth := 0
	for index := open; index < len(content); index++ {
		switch content[index] {
		case '\'', '"':
			next, ok := perlBuildPLQuotedStringEnd(content, index)
			if !ok {
				return "", false
			}
			index = next
		case opening:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return content[open+1 : index], true
			}
		}
	}
	return "", false
}

func perlBuildPLQuotedStringEnd(content string, start int) (int, bool) {
	quote := content[start]
	for index := start + 1; index < len(content); index++ {
		if content[index] == '\\' {
			index++
			continue
		}
		if content[index] == quote {
			return index, true
		}
	}
	return 0, false
}

func perlBuildPLWithoutComments(content string) string {
	var cleaned strings.Builder
	for index := 0; index < len(content); index++ {
		if content[index] == '\'' || content[index] == '"' {
			end, ok := perlBuildPLQuotedStringEnd(content, index)
			if !ok {
				return content
			}
			cleaned.WriteString(content[index : end+1])
			index = end
			continue
		}
		if content[index] == '#' {
			for index < len(content) && content[index] != '\n' {
				cleaned.WriteByte(' ')
				index++
			}
			if index == len(content) {
				break
			}
		}
		cleaned.WriteByte(content[index])
	}
	return cleaned.String()
}

func perlBuildPLDependency(name, constraint string, group perlBuildPLPrerequisiteGroup) DependencyReference {
	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  group.name,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        group.scope,
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency
}
