package analyze

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
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

	steps, _ := runs["steps"].([]any)
	dependencies, incomplete := githubActionsStepDependencies(steps, "runs.steps")
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func githubActionsStepDependencies(steps []any, group string) ([]DependencyReference, []string) {
	dependencies := make([]DependencyReference, 0)
	var incomplete []string
	for index, rawStep := range steps {
		step, ok := asStringMap(rawStep)
		if !ok {
			continue
		}
		uses, ok := step["uses"].(string)
		if !ok {
			continue
		}
		dependency, ok := githubActionsActionDependency(uses, group)
		if ok {
			dependencies = append(dependencies, dependency)
		} else if strings.HasPrefix(strings.TrimSpace(uses), "docker://") {
			incomplete = append(incomplete, fmt.Sprintf("%s[%d].uses could not be parsed as a Docker image: %s", group, index, strings.TrimSpace(uses)))
		}
	}

	return dependencies, incomplete
}

func githubActionsActionDependency(raw, group string) (DependencyReference, bool) {
	raw = strings.TrimSpace(raw)
	if image, ok := strings.CutPrefix(raw, "docker://"); ok {
		if strings.Contains(image, "$") {
			return DependencyReference{}, false
		}
		dependency, ok := dockerImageDependency(image)
		if !ok {
			return DependencyReference{}, false
		}
		dependency.Raw = raw
		dependency.SourceGroup = group
		dependency.Relationship = RelationshipDirect
		dependency.Scope = ScopeRuntime
		return dependency, true
	}

	return githubRepositoryDependency(raw, group)
}

func githubRepositoryDependency(raw, group string) (DependencyReference, bool) {
	name, ref, hasRef := strings.Cut(raw, "@")
	if !hasRef || name == "" || ref == "" || strings.HasPrefix(name, "./") {
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

	attributes := map[string]string{
		"source_url": "https://github.com/" + strings.Join(parts[:2], "/"),
		"source_ref": ref,
	}
	if len(parts) > 2 {
		attributes["source_path"] = strings.Join(parts[2:], "/")
	}

	return DependencyReference{
		Raw:               raw,
		Name:              name,
		VersionConstraint: ref,
		SourceGroup:       group,
		OriginKind:        OriginGit,
		Relationship:      RelationshipDirect,
		Scope:             ScopeRuntime,
		Attributes:        attributes,
	}, true
}
