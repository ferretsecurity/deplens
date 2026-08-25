package analyze

import (
	"fmt"
	"strings"
	"unicode"
)

type haskellCabalMatcherConfig struct{}

type haskellCabalParser struct{}

type haskellCabalComponent struct {
	group string
	scope DependencyScope
}

func newHaskellCabalParser(haskellCabalMatcherConfig) (sourceAnalyzer, error) {
	return haskellCabalParser{}, nil
}

func (haskellCabalParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	lines := strings.Split(string(content), "\n")
	if !isHaskellCabalManifest(lines) {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seen := make(map[string]struct{})
	seenMessages := make(map[string]struct{})
	component := haskellCabalComponent{group: "build-depends", scope: ScopeRuntime}
	var dependencyValue strings.Builder
	collectingDependencies := false

	flushDependencies := func() {
		if !collectingDependencies {
			return
		}
		collectHaskellCabalDependencies(dependencyValue.String(), component, &dependencies, &incomplete, seen, seenMessages)
		dependencyValue.Reset()
		collectingDependencies = false
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(haskellCabalWithoutComment(rawLine))
		if line == "" {
			continue
		}

		if haskellCabalComponentHeader(line) {
			flushDependencies()
			component = haskellCabalComponentForHeader(line)
			continue
		}
		if haskellCabalConditional(line) {
			flushDependencies()
			continue
		}

		fieldName, value, isField := haskellCabalField(line)
		if isField {
			flushDependencies()
			if strings.EqualFold(fieldName, "build-depends") {
				dependencyValue.WriteString(value)
				collectingDependencies = true
			}
			continue
		}

		if collectingDependencies && haskellCabalIndentation(rawLine) > 0 {
			if dependencyValue.Len() > 0 {
				dependencyValue.WriteByte(' ')
			}
			dependencyValue.WriteString(line)
			continue
		}
		flushDependencies()
	}
	flushDependencies()

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func isHaskellCabalManifest(lines []string) bool {
	for _, rawLine := range lines {
		if haskellCabalIndentation(rawLine) != 0 {
			continue
		}
		field, value, ok := haskellCabalField(strings.TrimSpace(haskellCabalWithoutComment(rawLine)))
		if ok && strings.EqualFold(field, "name") && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func haskellCabalWithoutComment(line string) string {
	if index := strings.Index(line, "--"); index >= 0 {
		return line[:index]
	}
	return line
}

func haskellCabalField(line string) (string, string, bool) {
	index := strings.IndexByte(line, ':')
	if index <= 0 {
		return "", "", false
	}
	if index+1 < len(line) && line[index+1] != ' ' && line[index+1] != '\t' {
		return "", "", false
	}
	name := strings.TrimSpace(line[:index])
	if name == "" {
		return "", "", false
	}
	for _, character := range name {
		if !unicode.IsLetter(character) && character != '-' {
			return "", "", false
		}
	}
	return name, strings.TrimSpace(line[index+1:]), true
}

func haskellCabalComponentHeader(line string) bool {
	first, _, _ := strings.Cut(line, " ")
	switch strings.ToLower(first) {
	case "library", "executable", "test-suite", "benchmark", "foreign-library", "common", "custom-setup":
		return true
	default:
		return false
	}
}

func haskellCabalComponentForHeader(line string) haskellCabalComponent {
	fields := strings.Fields(line)
	kind := strings.ToLower(fields[0])
	name := ""
	if len(fields) > 1 {
		name = fields[1]
	}
	group := kind
	if name != "" {
		group += "." + name
	}
	group += ".build-depends"

	scope := ScopeRuntime
	switch kind {
	case "test-suite":
		scope = ScopeTest
	case "benchmark", "custom-setup":
		scope = ScopeBuild
	}
	return haskellCabalComponent{group: group, scope: scope}
}

func haskellCabalConditional(line string) bool {
	lower := strings.ToLower(line)
	return lower == "else" || strings.HasPrefix(lower, "if ") || strings.HasPrefix(lower, "if(")
}

func haskellCabalIndentation(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func collectHaskellCabalDependencies(value string, component haskellCabalComponent, dependencies *[]DependencyReference, incomplete *[]string, seen, seenMessages map[string]struct{}) {
	for _, rawDependency := range strings.Split(value, ",") {
		dependencyText := strings.TrimSpace(rawDependency)
		if dependencyText == "" {
			continue
		}
		dependency, message := haskellCabalDependency(dependencyText, component)
		if message != "" {
			*incomplete = appendUniqueMessage(*incomplete, seenMessages, message)
			continue
		}
		key := dependency.SourceGroup + "\x00" + dependency.Name + "\x00" + dependency.VersionConstraint
		*dependencies = appendUniqueDependency(*dependencies, seen, key, dependency)
	}
}

func haskellCabalDependency(value string, component haskellCabalComponent) (DependencyReference, string) {
	nameEnd := strings.IndexFunc(value, unicode.IsSpace)
	name := value
	constraint := ""
	if nameEnd >= 0 {
		name = value[:nameEnd]
		constraint = strings.TrimSpace(value[nameEnd:])
	}
	name = strings.TrimSpace(name)
	if !isHaskellCabalPackageName(name) {
		return DependencyReference{}, fmt.Sprintf("build-depends contains invalid package declaration %q", value)
	}

	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  component.group,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        component.scope,
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency, ""
}

func isHaskellCabalPackageName(name string) bool {
	if name == "" {
		return false
	}
	for _, character := range name {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '-' && character != ':' {
			return false
		}
	}
	return true
}
