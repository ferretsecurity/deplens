package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
)

type vcpkgConfigurationParser struct{}

func newVcpkgConfigurationParser(vcpkgConfigurationMatcherConfig) (sourceAnalyzer, error) {
	return vcpkgConfigurationParser{}, nil
}

func (vcpkgConfigurationParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse vcpkg configuration %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	rawRegistries, exists := root["registries"]
	if exists && !isJSONNull(rawRegistries) {
		var registries []json.RawMessage
		if err := json.Unmarshal(rawRegistries, &registries); err != nil {
			incomplete = append(incomplete, "registries: expected an array")
		} else {
			for index, rawRegistry := range registries {
				group := fmt.Sprintf("registries.%d.packages", index)
				var registry map[string]json.RawMessage
				if err := json.Unmarshal(rawRegistry, &registry); err != nil {
					incomplete = append(incomplete, fmt.Sprintf("registries.%d: expected an object", index))
					continue
				}
				dependencies, incomplete = appendVcpkgConfigurationPackages(dependencies, incomplete, registry["packages"], group)
			}
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func appendVcpkgConfigurationPackages(dependencies []DependencyReference, incomplete []string, raw json.RawMessage, group string) ([]DependencyReference, []string) {
	if len(raw) == 0 || isJSONNull(raw) {
		return dependencies, incomplete
	}

	var packages []json.RawMessage
	if err := json.Unmarshal(raw, &packages); err != nil {
		return dependencies, append(incomplete, fmt.Sprintf("%s: expected an array", group))
	}
	for _, rawPackage := range packages {
		var name string
		if err := json.Unmarshal(rawPackage, &name); err != nil {
			incomplete = append(incomplete, fmt.Sprintf("%s: package name must be a string", group))
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			incomplete = append(incomplete, fmt.Sprintf("%s: package name is required", group))
			continue
		}
		dependencies = append(dependencies, DependencyReference{
			Raw:          name,
			Name:         name,
			SourceGroup:  group,
			OriginKind:   OriginRegistry,
			Relationship: RelationshipDirect,
			Scope:        ScopeRuntime,
		})
	}
	return dependencies, incomplete
}
