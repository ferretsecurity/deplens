package analyze

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"
)

type pnpmLockParser struct{}

type pnpmLockFile struct {
	LockfileVersion string                      `yaml:"lockfileVersion"`
	Importers       map[string]pnpmLockImporter `yaml:"importers"`
}

type pnpmLockImporter struct {
	Dependencies         map[string]pnpmLockDependency `yaml:"dependencies"`
	DevDependencies      map[string]pnpmLockDependency `yaml:"devDependencies"`
	OptionalDependencies map[string]pnpmLockDependency `yaml:"optionalDependencies"`
}

type pnpmLockDependency struct {
	Version   string `yaml:"version"`
	Specifier string `yaml:"specifier"`
}

func (d *pnpmLockDependency) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		d.Version = value.Value
		return nil
	}

	type dependency pnpmLockDependency
	var parsed dependency
	if err := value.Decode(&parsed); err != nil {
		return err
	}
	*d = pnpmLockDependency(parsed)
	return nil
}

func newPNPMLockParser(raw pnpmLockMatcherConfig) (manifestParser, error) {
	return pnpmLockParser{}, nil
}

func (p pnpmLockParser) Match(path string, content []byte) (manifestParserResult, error) {
	var file pnpmLockFile
	if err := yaml.Unmarshal(content, &file); err != nil {
		return manifestParserResult{}, fmt.Errorf("parse yaml file %q: %w", path, err)
	}
	if file.LockfileVersion == "" {
		return manifestParserResult{}, nil
	}

	dependencies := make([]Dependency, 0)
	for _, importerPath := range sortedPNPMLockImporterPaths(file.Importers) {
		importer := file.Importers[importerPath]
		dependencies = appendPNPMLockDependencies(dependencies, "dependencies", importer.Dependencies)
		dependencies = appendPNPMLockDependencies(dependencies, "devDependencies", importer.DevDependencies)
		dependencies = appendPNPMLockDependencies(dependencies, "optionalDependencies", importer.OptionalDependencies)
	}

	if len(dependencies) == 0 {
		return manifestParserResult{
			Matched:         true,
			HasDependencies: boolPtr(false),
		}, nil
	}

	return manifestParserResult{
		Dependencies:    dependencies,
		Matched:         true,
		HasDependencies: boolPtr(true),
	}, nil
}

func appendPNPMLockDependencies(dependencies []Dependency, section string, values map[string]pnpmLockDependency) []Dependency {
	for _, name := range sortedPNPMLockDependencyNames(values) {
		dependencies = append(dependencies, Dependency{
			Name:    formatPNPMLockDependencyName(name, values[name]),
			Section: section,
		})
	}
	return dependencies
}

func formatPNPMLockDependencyName(name string, dependency pnpmLockDependency) string {
	if dependency.Version != "" {
		return name + "@" + dependency.Version
	}
	if dependency.Specifier != "" {
		return name + "@" + dependency.Specifier
	}
	return name
}

func sortedPNPMLockImporterPaths(importers map[string]pnpmLockImporter) []string {
	if len(importers) == 0 {
		return nil
	}

	paths := make([]string, 0, len(importers))
	for path := range importers {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

func sortedPNPMLockDependencyNames(dependencies map[string]pnpmLockDependency) []string {
	if len(dependencies) == 0 {
		return nil
	}

	names := make([]string, 0, len(dependencies))
	for name := range dependencies {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
