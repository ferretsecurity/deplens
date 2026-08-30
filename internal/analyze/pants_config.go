package analyze

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

const pantsDistribution = "pantsbuild.pants"

type pantsConfigParser struct{}

func newPantsConfigParser(pantsConfigMatcherConfig) (sourceAnalyzer, error) {
	return pantsConfigParser{}, nil
}

func (pantsConfigParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root map[string]any
	if err := toml.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Pants configuration %q: %w", path, err)
	}

	global, _ := anyStringMap(root["GLOBAL"])
	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)

	if rawVersion, exists := global["pants_version"]; exists {
		version, ok := rawVersion.(string)
		version = strings.TrimSpace(version)
		if !ok || version == "" {
			incomplete = append(incomplete, "GLOBAL.pants_version: expected a non-empty string")
		} else {
			dependencies = append(dependencies, DependencyReference{
				Raw:               pantsDistribution + "@" + version,
				Name:              pantsDistribution,
				VersionConstraint: version,
				SourceGroup:       "GLOBAL.pants_version",
				OriginKind:        OriginRegistry,
				Relationship:      RelationshipDirect,
				Scope:             ScopeBuild,
			})
		}
	}

	if rawBackends, exists := global["backend_packages"]; exists {
		backends, ok := rawBackends.([]any)
		if !ok {
			incomplete = append(incomplete, "GLOBAL.backend_packages: expected an array of package names")
		} else {
			for index, rawBackend := range backends {
				backend, ok := rawBackend.(string)
				backend = strings.TrimSpace(backend)
				if !ok || backend == "" {
					incomplete = append(incomplete, fmt.Sprintf("GLOBAL.backend_packages[%d]: expected a non-empty package name", index))
					continue
				}
				dependencies = append(dependencies, DependencyReference{
					Raw:          backend,
					Name:         backend,
					SourceGroup:  "GLOBAL.backend_packages",
					OriginKind:   OriginRegistry,
					Relationship: RelationshipDirect,
					Scope:        ScopeBuild,
				})
			}
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}
