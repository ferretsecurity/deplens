package analyze

import (
	"encoding/json"
	"fmt"
	"slices"
)

type composerManifestParser struct{}

func newComposerManifestParser(composerManifestMatcherConfig) (sourceAnalyzer, error) {
	return composerManifestParser{}, nil
}

func (composerManifestParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse composer manifest %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	dependencies, incomplete = appendComposerManifestGroup(dependencies, incomplete, root, "require", ScopeRuntime)
	dependencies, incomplete = appendComposerManifestGroup(dependencies, incomplete, root, "require-dev", ScopeDevelopment)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func appendComposerManifestGroup(dependencies []DependencyReference, incomplete []string, root map[string]json.RawMessage, group string, scope DependencyScope) ([]DependencyReference, []string) {
	raw, ok := root[group]
	if !ok || isJSONNull(raw) {
		return dependencies, incomplete
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return dependencies, append(incomplete, fmt.Sprintf("%s: expected an object of dependency constraints", group))
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if name == "" {
			incomplete = append(incomplete, fmt.Sprintf("%s: dependency name must not be empty", group))
			continue
		}
		if isComposerPlatformPackage(name) {
			continue
		}
		var constraint string
		if err := json.Unmarshal(values[name], &constraint); err != nil {
			incomplete = append(incomplete, fmt.Sprintf("%s.%s: expected a string dependency constraint", group, name))
			continue
		}
		dependencies = append(dependencies, DependencyReference{
			Raw:               name + "@" + constraint,
			Name:              name,
			VersionConstraint: constraint,
			SourceGroup:       group,
			Relationship:      RelationshipDirect,
			Scope:             scope,
		})
	}
	return dependencies, incomplete
}
