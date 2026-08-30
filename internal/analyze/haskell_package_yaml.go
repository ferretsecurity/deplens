package analyze

import (
	"fmt"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

type haskellPackageYAMLMatcherConfig struct{}

type haskellPackageYAMLParser struct{}

type haskellPackageYAMLFile struct {
	Name              string                                 `yaml:"name"`
	Dependencies      []string                               `yaml:"dependencies"`
	Library           *haskellPackageYAMLComponent           `yaml:"library"`
	Executables       map[string]haskellPackageYAMLComponent `yaml:"executables"`
	Tests             map[string]haskellPackageYAMLComponent `yaml:"tests"`
	Benchmarks        map[string]haskellPackageYAMLComponent `yaml:"benchmarks"`
	InternalLibraries map[string]haskellPackageYAMLComponent `yaml:"internal-libraries"`
}

type haskellPackageYAMLComponent struct {
	Dependencies []string `yaml:"dependencies"`
}

func newHaskellPackageYAMLParser(haskellPackageYAMLMatcherConfig) (sourceAnalyzer, error) {
	return haskellPackageYAMLParser{}, nil
}

func (haskellPackageYAMLParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var manifest haskellPackageYAMLFile
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Hpack package manifest %q: %w", path, err)
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	dependencies, incomplete = appendHaskellPackageYAMLDependencies(dependencies, incomplete, "dependencies", ScopeRuntime, manifest.Dependencies)
	if manifest.Library != nil {
		dependencies, incomplete = appendHaskellPackageYAMLDependencies(dependencies, incomplete, "library.dependencies", ScopeRuntime, manifest.Library.Dependencies)
	}
	dependencies, incomplete = appendHaskellPackageYAMLComponentDependencies(dependencies, incomplete, "executables", ScopeRuntime, manifest.Executables)
	dependencies, incomplete = appendHaskellPackageYAMLComponentDependencies(dependencies, incomplete, "tests", ScopeTest, manifest.Tests)
	dependencies, incomplete = appendHaskellPackageYAMLComponentDependencies(dependencies, incomplete, "benchmarks", ScopeBuild, manifest.Benchmarks)
	dependencies, incomplete = appendHaskellPackageYAMLComponentDependencies(dependencies, incomplete, "internal-libraries", ScopeRuntime, manifest.InternalLibraries)

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func appendHaskellPackageYAMLComponentDependencies(dependencies []DependencyReference, incomplete []string, kind string, scope DependencyScope, components map[string]haskellPackageYAMLComponent) ([]DependencyReference, []string) {
	for name, component := range components {
		group := kind + "." + name + ".dependencies"
		dependencies, incomplete = appendHaskellPackageYAMLDependencies(dependencies, incomplete, group, scope, component.Dependencies)
	}
	return dependencies, incomplete
}

func appendHaskellPackageYAMLDependencies(dependencies []DependencyReference, incomplete []string, group string, scope DependencyScope, values []string) ([]DependencyReference, []string) {
	for index, value := range values {
		dependency, message := haskellPackageYAMLDependency(value, group, scope)
		if message != "" {
			incomplete = append(incomplete, fmt.Sprintf("%s dependency %d: %s", group, index+1, message))
			continue
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies, incomplete
}

func haskellPackageYAMLDependency(value, group string, scope DependencyScope) (DependencyReference, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return DependencyReference{}, "package is required"
	}

	nameEnd := strings.IndexFunc(value, unicode.IsSpace)
	name := value
	constraint := ""
	if nameEnd >= 0 {
		name = value[:nameEnd]
		constraint = strings.TrimSpace(value[nameEnd:])
	}
	if !isHaskellCabalPackageName(name) {
		return DependencyReference{}, fmt.Sprintf("invalid package declaration %q", value)
	}

	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        scope,
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency, ""
}
