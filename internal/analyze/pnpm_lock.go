package analyze

import (
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type pnpmLockParser struct{}

type pnpmLockFile struct {
	LockfileVersion      string                        `yaml:"lockfileVersion"`
	Importers            map[string]pnpmLockImporter   `yaml:"importers"`
	Dependencies         map[string]pnpmLockDependency `yaml:"dependencies"`
	DevDependencies      map[string]pnpmLockDependency `yaml:"devDependencies"`
	OptionalDependencies map[string]pnpmLockDependency `yaml:"optionalDependencies"`
	// `packages` lists every resolved version in the store (transitive, etc.).
	Packages map[string]yaml.Node `yaml:"packages"`
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
	seen := make(map[string]struct{})

	if importer, ok := file.Importers["."]; ok {
		dependencies = appendPNPMLockDependencies(dependencies, seen, "dependencies", importer.Dependencies)
		dependencies = appendPNPMLockDependencies(dependencies, seen, "devDependencies", importer.DevDependencies)
		dependencies = appendPNPMLockDependencies(dependencies, seen, "optionalDependencies", importer.OptionalDependencies)
	} else {
		dependencies = appendPNPMLockDependencies(dependencies, seen, "dependencies", file.Dependencies)
		dependencies = appendPNPMLockDependencies(dependencies, seen, "devDependencies", file.DevDependencies)
		dependencies = appendPNPMLockDependencies(dependencies, seen, "optionalDependencies", file.OptionalDependencies)
	}

	dependencies = appendPNPMLockTransitiveFromPackages(dependencies, seen, file.Packages)

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

func appendPNPMLockTransitiveFromPackages(dependencies []Dependency, seen map[string]struct{}, packages map[string]yaml.Node) []Dependency {
	if len(packages) == 0 {
		return dependencies
	}
	keys := make([]string, 0, len(packages))
	for k := range packages {
		if k != "" {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, key := range keys {
		name, version := pnpmLockPackageKeyNameVersion(pnpmLockStripPeeringSuffix(key))
		if name == "" {
			continue
		}
		raw := name
		if version != "" {
			raw = name + "@" + version
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		dep := Dependency{Raw: raw, Name: name}
		if version != "" {
			dep.Version = version
		}
		dependencies = append(dependencies, dep)
	}
	return dependencies
}

func pnpmLockStripPeeringSuffix(key string) string {
	if i := strings.Index(key, "("); i != -1 {
		return strings.TrimSpace(key[:i])
	}
	return key
}

// pnpmLockPackageKeyNameVersion parses a `packages` / lockfile key into name and resolved version.
// v9+: "react@1.0.0", "@types/node@20.0.0". v5/v6: "/react/1.0.0", "/@types/node/20.0.0".
func pnpmLockPackageKeyNameVersion(key string) (name, version string) {
	if key == "" {
		return "", ""
	}
	if strings.HasPrefix(key, "/") {
		return pnpmLockPackageKeyNameVersionV5Path(key)
	}
	return pnpmLockPackageKeyNameVersionV9(key)
}

func pnpmLockPackageKeyNameVersionV5Path(key string) (name, version string) {
	k := strings.TrimPrefix(key, "/")
	if k == "" {
		return "", ""
	}
	last := strings.LastIndex(k, "/")
	if last == -1 {
		return k, ""
	}
	return k[:last], k[last+1:]
}

func pnpmLockPackageKeyNameVersionV9(key string) (name, version string) {
	if strings.HasPrefix(key, "@") {
		rest := key[1:]
		i := strings.LastIndex(rest, "@")
		if i == -1 {
			return key, ""
		}
		if i == 0 { // "@@" weird
			return key, ""
		}
		return "@" + rest[:i], rest[i+1:]
	}
	i := strings.LastIndex(key, "@")
	if i <= 0 {
		return key, ""
	}
	return key[:i], key[i+1:]
}

func appendPNPMLockDependencies(dependencies []Dependency, seen map[string]struct{}, section string, values map[string]pnpmLockDependency) []Dependency {
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
		seen[dep.Raw] = struct{}{}
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
