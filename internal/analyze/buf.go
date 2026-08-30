package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type bufManifestParserConfig struct{}

type bufManifestParser struct{}

type bufManifest struct {
	Version string      `yaml:"version"`
	Deps    []string    `yaml:"deps"`
	Modules []bufModule `yaml:"modules"`
}

type bufModule struct {
	Path string `yaml:"path"`
}

func newBufManifestParser(bufManifestParserConfig) (sourceAnalyzer, error) {
	return bufManifestParser{}, nil
}

func (bufManifestParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var manifest bufManifest
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Buf manifest %q: %w", path, err)
	}
	if manifest.Version != "v1" && manifest.Version != "v2" {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(manifest.Deps)+len(manifest.Modules))
	for _, raw := range manifest.Deps {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		dependencies = append(dependencies, DependencyReference{
			Raw:          name,
			Name:         name,
			SourceGroup:  "deps",
			OriginKind:   OriginRegistry,
			Relationship: RelationshipDirect,
			Scope:        ScopeRuntime,
		})
	}
	for _, module := range manifest.Modules {
		modulePath := strings.TrimSpace(module.Path)
		if modulePath == "" {
			continue
		}
		dependencies = append(dependencies, DependencyReference{
			Raw:          modulePath,
			Name:         modulePath,
			SourceGroup:  "modules",
			OriginKind:   OriginWorkspace,
			Relationship: RelationshipInconclusive,
		})
	}
	sortDependencyReferences(dependencies)

	return sourceAnalyzerResult{
		Dependencies: dependencies,
		Analysis:     completeAnalysis(dependencies),
		Recognized:   true,
	}, nil
}
