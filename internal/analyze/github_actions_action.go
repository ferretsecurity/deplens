package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type githubActionsActionParser struct{}

func newGithubActionsActionParser(githubActionsActionMatcherConfig) (sourceAnalyzer, error) {
	return githubActionsActionParser{}, nil
}

func (githubActionsActionParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var document any
	if err := yaml.Unmarshal(content, &document); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse GitHub Action metadata %q: %w", path, err)
	}

	root, ok := asStringMap(document)
	if !ok {
		return sourceAnalyzerResult{}, nil
	}
	runs, ok := asStringMap(root["runs"])
	if !ok {
		return sourceAnalyzerResult{}, nil
	}
	using, ok := runs["using"].(string)
	if !ok || strings.TrimSpace(using) == "" {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0)
	steps, _ := runs["steps"].([]any)
	for _, rawStep := range steps {
		step, ok := asStringMap(rawStep)
		if !ok {
			continue
		}
		uses, ok := step["uses"].(string)
		if !ok {
			continue
		}
		dependency, ok := githubActionsActionDependency(uses)
		if ok {
			dependencies = append(dependencies, dependency)
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func githubActionsActionDependency(raw string) (DependencyReference, bool) {
	raw = strings.TrimSpace(raw)
	name, ref, hasRef := strings.Cut(raw, "@")
	if !hasRef || name == "" || ref == "" || strings.HasPrefix(name, "./") || strings.HasPrefix(name, "docker://") {
		return DependencyReference{}, false
	}

	parts := strings.Split(name, "/")
	if len(parts) < 2 {
		return DependencyReference{}, false
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return DependencyReference{}, false
		}
	}

	return DependencyReference{
		Raw:               raw,
		Name:              name,
		VersionConstraint: ref,
		SourceGroup:       "runs.steps",
		OriginKind:        OriginGit,
		Relationship:      RelationshipDirect,
		Scope:             ScopeRuntime,
		Attributes: map[string]string{
			"source_url": "https://github.com/" + name,
			"source_ref": ref,
		},
	}, true
}
