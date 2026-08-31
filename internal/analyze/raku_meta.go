package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
)

type rakuMetaParser struct{}

type rakuMetaDependencyGroup struct {
	name  string
	scope DependencyScope
}

var rakuMetaDependencyGroups = []rakuMetaDependencyGroup{
	{name: "depends", scope: ScopeRuntime},
	{name: "build-depends", scope: ScopeBuild},
	{name: "test-depends", scope: ScopeTest},
}

func newRakuMetaParser(rakuMetaMatcherConfig) (sourceAnalyzer, error) {
	return rakuMetaParser{}, nil
}

func (rakuMetaParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(content, &manifest); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Raku META6 manifest %q: %w", path, err)
	}
	if !isRakuMetaManifest(manifest) {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, group := range rakuMetaDependencyGroups {
		rawValues, exists := manifest[group.name]
		if !exists || isJSONNull(rawValues) {
			continue
		}

		var values []string
		if err := json.Unmarshal(rawValues, &values); err != nil {
			incomplete = append(incomplete, fmt.Sprintf("%s: expected an array of dependency strings", group.name))
			continue
		}
		for _, value := range values {
			dependency, message := rakuMetaDependency(value, group)
			if message != "" {
				incomplete = append(incomplete, message)
				continue
			}
			dependencies = append(dependencies, dependency)
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func isRakuMetaManifest(manifest map[string]json.RawMessage) bool {
	rawName, exists := manifest["name"]
	if !exists {
		return false
	}
	var name string
	return json.Unmarshal(rawName, &name) == nil && strings.TrimSpace(name) != ""
}

func rakuMetaDependency(raw string, group rakuMetaDependencyGroup) (DependencyReference, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DependencyReference{}, fmt.Sprintf("%s: dependency name must not be empty", group.name)
	}

	name, qualifiers, message := splitRakuMetaDependency(raw)
	if message != "" {
		return DependencyReference{}, fmt.Sprintf("%s.%s: %s", group.name, raw, message)
	}
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("%s: dependency name must not be empty", group.name)
	}

	dependency := DependencyReference{
		Raw:          raw,
		Name:         name,
		SourceGroup:  group.name,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        group.scope,
	}
	if version := qualifiers["ver"]; version != "" {
		dependency.VersionConstraint = version
	}
	for _, key := range []string{"auth", "api"} {
		if value := qualifiers[key]; value != "" {
			if dependency.Attributes == nil {
				dependency.Attributes = make(map[string]string)
			}
			dependency.Attributes[key] = value
		}
	}
	return dependency, ""
}

func splitRakuMetaDependency(raw string) (string, map[string]string, string) {
	firstQualifier := -1
	for index := 0; index < len(raw); index++ {
		if raw[index] != ':' {
			continue
		}
		if _, _, _, ok := rakuMetaQualifierAt(raw, index); ok {
			firstQualifier = index
			break
		}
	}
	if firstQualifier == -1 {
		return raw, nil, ""
	}

	name := strings.TrimSpace(raw[:firstQualifier])
	qualifiers := make(map[string]string)
	for index := firstQualifier; index < len(raw); {
		key, value, next, ok := rakuMetaQualifierAt(raw, index)
		if !ok {
			return "", nil, "invalid dependency qualifier"
		}
		if value == "" {
			return "", nil, fmt.Sprintf("%s qualifier must not be empty", key)
		}
		qualifiers[key] = value
		index = next
	}
	return name, qualifiers, ""
}

func rakuMetaQualifierAt(value string, start int) (string, string, int, bool) {
	if start >= len(value) || value[start] != ':' {
		return "", "", start, false
	}
	keyStart := start + 1
	keyEnd := keyStart
	for keyEnd < len(value) && ((value[keyEnd] >= 'a' && value[keyEnd] <= 'z') || (value[keyEnd] >= 'A' && value[keyEnd] <= 'Z') || value[keyEnd] == '-') {
		keyEnd++
	}
	if keyEnd == keyStart {
		return "", "", start, false
	}
	valueStart := keyEnd
	if valueStart < len(value) && value[valueStart] == ':' {
		valueStart++
	}
	if valueStart >= len(value) || value[valueStart] != '<' {
		return "", "", start, false
	}
	valueEnd := strings.IndexByte(value[valueStart+1:], '>')
	if valueEnd == -1 {
		return "", "", start, false
	}
	valueEnd += valueStart + 1
	return value[keyStart:keyEnd], strings.TrimSpace(value[valueStart+1 : valueEnd]), valueEnd + 1, true
}
