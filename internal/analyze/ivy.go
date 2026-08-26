package analyze

import (
	"encoding/xml"
	"fmt"
	"strings"
)

type ivyParser struct{}

type ivyModule struct {
	XMLName      xml.Name        `xml:"ivy-module"`
	Dependencies []ivyDependency `xml:"dependencies>dependency"`
}

type ivyDependency struct {
	Organization  string `xml:"org,attr"`
	Name          string `xml:"name,attr"`
	Revision      string `xml:"rev,attr"`
	Configuration string `xml:"conf,attr"`
}

func newIvyParser(ivyMatcherConfig) (sourceAnalyzer, error) {
	return ivyParser{}, nil
}

func (ivyParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var module ivyModule
	if err := xml.Unmarshal(content, &module); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Ivy manifest %q: %w", path, err)
	}
	if module.XMLName.Local != "ivy-module" {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(module.Dependencies))
	incomplete := make([]string, 0)
	for _, value := range module.Dependencies {
		organization := strings.TrimSpace(value.Organization)
		name := strings.TrimSpace(value.Name)
		if organization == "" || name == "" {
			incomplete = append(incomplete, "dependencies contains a dependency without both org and name")
			continue
		}

		coordinate := organization + ":" + name
		dependency := DependencyReference{
			Raw:          coordinate,
			Name:         coordinate,
			SourceGroup:  "dependencies",
			OriginKind:   OriginRegistry,
			Relationship: RelationshipDirect,
			Scope:        ScopeRuntime,
		}
		if revision := strings.TrimSpace(value.Revision); revision != "" {
			dependency.Raw += ":" + revision
			dependency.VersionConstraint = normalizeMavenManifestConstraint(revision)
		}
		if configuration := strings.TrimSpace(value.Configuration); configuration != "" {
			dependency.Attributes = map[string]string{"conf": configuration}
		}
		dependencies = append(dependencies, dependency)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}
