package analyze

import (
	"encoding/json"
	"fmt"
)

type ocamlEsyParser struct{}

type ocamlEsyDependencyGroup struct {
	path  []string
	scope DependencyScope
}

var ocamlEsyDependencyGroups = []ocamlEsyDependencyGroup{
	{path: []string{"dependencies"}, scope: ScopeRuntime},
	{path: []string{"devDependencies"}, scope: ScopeDevelopment},
	{path: []string{"override", "dependencies"}, scope: ScopeRuntime},
}

func newOCamlEsyParser(ocamlEsyParserConfig) (sourceAnalyzer, error) {
	return ocamlEsyParser{}, nil
}

func (ocamlEsyParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Esy manifest %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, group := range ocamlEsyDependencyGroups {
		dependencies, incomplete = appendOCamlEsyDependencyGroup(dependencies, incomplete, root, group)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func appendOCamlEsyDependencyGroup(dependencies []DependencyReference, incomplete []string, root map[string]json.RawMessage, group ocamlEsyDependencyGroup) ([]DependencyReference, []string) {
	groupName := group.path[0]
	raw, exists := root[groupName]
	if !exists || isJSONNull(raw) {
		return dependencies, incomplete
	}

	for _, key := range group.path[1:] {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return dependencies, append(incomplete, fmt.Sprintf("%s: expected an object", groupName))
		}
		raw, exists = object[key]
		groupName += "." + key
		if !exists || isJSONNull(raw) {
			return dependencies, incomplete
		}
	}

	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return dependencies, append(incomplete, fmt.Sprintf("%s: expected an object of dependency constraints", groupName))
	}
	for _, name := range sortedStringKeys(entries) {
		if name == "" {
			incomplete = append(incomplete, fmt.Sprintf("%s: dependency name must not be empty", groupName))
			continue
		}
		var constraint string
		if err := json.Unmarshal(entries[name], &constraint); err != nil {
			incomplete = append(incomplete, fmt.Sprintf("%s.%s: expected a string dependency constraint", groupName, name))
			continue
		}
		dependencies = append(dependencies, DependencyReference{
			Raw:               name + "@" + constraint,
			Name:              name,
			VersionConstraint: constraint,
			SourceGroup:       groupName,
			Relationship:      RelationshipDirect,
			Scope:             group.scope,
		})
	}
	return dependencies, incomplete
}
