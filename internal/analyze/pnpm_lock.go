package analyze

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"
)

type pnpmLockParser struct{}

type pnpmLockFile struct {
	LockfileVersion      string                        `yaml:"lockfileVersion"`
	Importers            map[string]pnpmLockImporter   `yaml:"importers"`
	Dependencies         map[string]pnpmLockDependency `yaml:"dependencies"`
	DevDependencies      map[string]pnpmLockDependency `yaml:"devDependencies"`
	OptionalDependencies map[string]pnpmLockDependency `yaml:"optionalDependencies"`
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
	if importer, ok := file.Importers["."]; ok {
		dependencies = appendPNPMLockDependencies(dependencies, "dependencies", importer.Dependencies)
		dependencies = appendPNPMLockDependencies(dependencies, "devDependencies", importer.DevDependencies)
		dependencies = appendPNPMLockDependencies(dependencies, "optionalDependencies", importer.OptionalDependencies)
	} else {
		dependencies = appendPNPMLockDependencies(dependencies, "dependencies", file.Dependencies)
		dependencies = appendPNPMLockDependencies(dependencies, "devDependencies", file.DevDependencies)
		dependencies = appendPNPMLockDependencies(dependencies, "optionalDependencies", file.OptionalDependencies)
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
		d := values[name]
		dep := Dependency{
			Raw:     formatPNPMLockDependencyName(name, d),
			Name:    name,
			Section: section,
		}
		if d.Version != "" {
			dep.Version = d.Version
			if d.Specifier != "" {
				dep.Extras = map[string]string{"specifier": d.Specifier}
			}
		} else if d.Specifier != "" {
			dep.Constraint = d.Specifier
		}
		dependencies = append(dependencies, dep)
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
