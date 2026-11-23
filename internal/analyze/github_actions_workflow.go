package analyze

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

type githubActionsWorkflowMatcherConfig struct{}
type githubActionsWorkflowParser struct{}

func newGithubActionsWorkflowParser(githubActionsWorkflowMatcherConfig) (sourceAnalyzer, error) {
	return githubActionsWorkflowParser{}, nil
}

func (githubActionsWorkflowParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var workflow struct {
		Jobs map[string]struct {
			Uses  string
			Steps []any
		}
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse GitHub Actions workflow %q: %w", path, err)
	}
	var dependencies []DependencyReference
	var incomplete []string
	for _, id := range sortedStringKeys(workflow.Jobs) {
		job := workflow.Jobs[id]
		group := "jobs." + id
		uses := strings.TrimSpace(job.Uses)
		if dep, ok := githubRepositoryDependency(uses, group+".uses"); ok && isReusableWorkflowPath(dep.Attributes["source_path"]) {
			dependencies = append(dependencies, dep)
		}
		steps, messages := githubActionsStepDependencies(job.Steps, group+".steps")
		dependencies = append(dependencies, steps...)
		incomplete = append(incomplete, messages...)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func isReusableWorkflowPath(path string) bool {
	filename, ok := strings.CutPrefix(path, ".github/workflows/")
	return ok && !strings.Contains(filename, "/") && (strings.HasSuffix(filename, ".yml") || strings.HasSuffix(filename, ".yaml"))
}
