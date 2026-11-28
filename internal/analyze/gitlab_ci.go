package analyze

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/distribution/reference"
	"go.yaml.in/yaml/v3"
)

type gitlabCIConfig struct{}
type gitlabCIParser struct{}

func newGitlabCIParser(gitlabCIConfig) (sourceAnalyzer, error) {
	return gitlabCIParser{}, nil
}

func (gitlabCIParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var dependencies []DependencyReference
	var incomplete []string
	recognized := false
	addImage := func(value any, location string) {
		if fields, ok := asStringMap(value); ok {
			value = fields["name"]
			location += ".name"
		}
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
		dep, ok := dockerImageDependency(image)
		if _, err := reference.Parse(image); err != nil || !ok {
			incomplete = append(incomplete, fmt.Sprintf("%s could not be parsed as a Docker image: %s", location, image))
			return
		}
		dep.SourceGroup = location
		dep.Relationship = RelationshipDirect
		dep.Scope = ScopeRuntime
		dependencies = append(dependencies, dep)
	}
	addFields := func(fields map[string]any, prefix string) {
		if image, exists := fields["image"]; exists {
			recognized = true
			if image != nil {
				addImage(image, prefix+"image")
			}
		}
		if rawServices, exists := fields["services"]; exists && rawServices != nil {
			recognized = true
			services, ok := rawServices.([]any)
			if !ok {
				incomplete = append(incomplete, prefix+"services is not a list")
				return
			}
			for i, service := range services {
				addImage(service, fmt.Sprintf("%sservices[%d]", prefix, i))
			}
		}
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var root map[string]any
		if err := decoder.Decode(&root); err != nil {
			if err == io.EOF {
				break
			}
			return sourceAnalyzerResult{}, fmt.Errorf("parse GitLab CI file %q: %w", path, err)
		}
		addFields(root, "")
		for _, key := range sortedStringKeys(root) {
			switch key {
			case "image", "services", "variables", "include", "stages", "workflow", "spec", "before_script", "after_script", "cache", "types":
				continue
			}
			if fields, ok := asStringMap(root[key]); ok {
				addFields(fields, key+".")
			}
		}
	}
	if !recognized {
		return sourceAnalyzerResult{}, nil
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}
