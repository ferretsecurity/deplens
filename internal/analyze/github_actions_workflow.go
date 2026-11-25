package analyze

import (
	_ "crypto/sha256"
	_ "crypto/sha512"
	"fmt"
	"strings"

	"github.com/distribution/reference"
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
			Uses      string
			Steps     []any
			Container any
			Services  any
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
		images, messages := githubActionsWorkflowContainerDependencies(job.Container, job.Services, group)
		dependencies = append(dependencies, images...)
		incomplete = append(incomplete, messages...)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func githubActionsWorkflowContainerDependencies(container, rawServices any, group string) ([]DependencyReference, []string) {
	var dependencies []DependencyReference
	var incomplete []string
	addImage := func(value any, location string) {
		image, ok := value.(string)
		if !ok || strings.TrimSpace(image) == "" {
			incomplete = append(incomplete, location+" is not a non-empty string")
			return
		}
		image = strings.TrimSpace(image)
		if strings.Contains(image, "$") {
			incomplete = append(incomplete, fmt.Sprintf("%s contains unresolved interpolation: %s", location, image))
			return
		}
		dependency, ok := dockerImageDependency(image)
		if _, err := reference.Parse(image); err != nil || !ok {
			incomplete = append(incomplete, fmt.Sprintf("%s could not be parsed as a Docker image: %s", location, image))
			return
		}
		dependency.SourceGroup = location
		dependency.Relationship = RelationshipDirect
		dependency.Scope = ScopeRuntime
		dependencies = append(dependencies, dependency)
	}
	if container != nil {
		if fields, ok := asStringMap(container); ok {
			addImage(fields["image"], group+".container.image")
		} else {
			addImage(container, group+".container")
		}
	}
	if rawServices != nil {
		services, ok := asStringMap(rawServices)
		if !ok {
			incomplete = append(incomplete, group+".services is not a mapping")
		} else {
			for _, name := range sortedStringKeys(services) {
				location := group + ".services." + name
				service, ok := asStringMap(services[name])
				if !ok {
					incomplete = append(incomplete, location+" is not a mapping")
					continue
				}
				addImage(service["image"], location+".image")
			}
		}
	}
	return dependencies, incomplete
}

func isReusableWorkflowPath(path string) bool {
	filename, ok := strings.CutPrefix(path, ".github/workflows/")
	return ok && !strings.Contains(filename, "/") && (strings.HasSuffix(filename, ".yml") || strings.HasSuffix(filename, ".yaml"))
}
