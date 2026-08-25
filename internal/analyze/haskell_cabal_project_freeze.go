package analyze

import (
	"fmt"
	"strings"
)

type haskellCabalProjectFreezeMatcherConfig struct{}

type haskellCabalProjectFreezeParser struct{}

func newHaskellCabalProjectFreezeParser(haskellCabalProjectFreezeMatcherConfig) (sourceAnalyzer, error) {
	return haskellCabalProjectFreezeParser{}, nil
}

func (haskellCabalProjectFreezeParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	lines := strings.Split(string(content), "\n")
	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seen := make(map[string]struct{})
	seenMessages := make(map[string]struct{})
	var constraints strings.Builder
	recognized := false
	collectingConstraints := false

	flushConstraints := func() {
		if !collectingConstraints {
			return
		}
		collectHaskellCabalProjectFreezeConstraints(constraints.String(), &dependencies, &incomplete, seen, seenMessages)
		constraints.Reset()
		collectingConstraints = false
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(haskellCabalWithoutComment(rawLine))
		if line == "" {
			continue
		}

		if haskellCabalIndentation(rawLine) == 0 {
			field, value, ok := haskellCabalField(line)
			if !ok {
				flushConstraints()
				continue
			}
			flushConstraints()
			switch strings.ToLower(field) {
			case "index-state":
				if strings.TrimSpace(value) != "" {
					recognized = true
				}
			case "constraints":
				recognized = true
				constraints.WriteString(value)
				collectingConstraints = true
			}
			continue
		}

		if collectingConstraints {
			if constraints.Len() > 0 {
				constraints.WriteByte(' ')
			}
			constraints.WriteString(line)
		}
	}
	flushConstraints()

	if !recognized {
		return sourceAnalyzerResult{}, nil
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func collectHaskellCabalProjectFreezeConstraints(value string, dependencies *[]DependencyReference, incomplete *[]string, seen, seenMessages map[string]struct{}) {
	for _, rawConstraint := range strings.Split(value, ",") {
		constraint := strings.TrimSpace(rawConstraint)
		if constraint == "" {
			continue
		}

		dependency, message := haskellCabalProjectFreezeDependency(constraint)
		if message != "" {
			*incomplete = appendUniqueMessage(*incomplete, seenMessages, message)
			continue
		}
		key := dependency.Name + "\x00" + dependency.Version
		*dependencies = appendUniqueDependency(*dependencies, seen, key, dependency)
	}
}

func haskellCabalProjectFreezeDependency(constraint string) (DependencyReference, string) {
	packageSelector, version, found := strings.Cut(constraint, "==")
	if !found {
		return DependencyReference{}, fmt.Sprintf("constraints contains unsupported declaration %q", constraint)
	}

	name := strings.TrimSpace(packageSelector)
	if strings.HasPrefix(strings.ToLower(name), "any.") {
		name = name[len("any."):]
	}
	if !isHaskellCabalPackageName(name) {
		return DependencyReference{}, fmt.Sprintf("constraints contains invalid package declaration %q", constraint)
	}
	version = strings.TrimSpace(version)
	if version == "" || strings.ContainsAny(version, " \t") {
		return DependencyReference{}, fmt.Sprintf("constraints contains invalid version declaration %q", constraint)
	}

	return DependencyReference{
		Raw:          name + "@" + version,
		Name:         name,
		Version:      version,
		SourceGroup:  "constraints",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipInconclusive,
		Scope:        ScopeRuntime,
	}, ""
}
