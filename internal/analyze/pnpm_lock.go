package analyze

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
	"golang.org/x/mod/semver"
)

type pnpmLockParser struct{}

type pnpmLockFile struct {
	LockfileVersion      string                        `yaml:"lockfileVersion"`
	Importers            map[string]pnpmLockImporter   `yaml:"importers"`
	Dependencies         map[string]pnpmLockDependency `yaml:"dependencies"`
	DevDependencies      map[string]pnpmLockDependency `yaml:"devDependencies"`
	OptionalDependencies map[string]pnpmLockDependency `yaml:"optionalDependencies"`
	// `packages` lists every resolved version in the store (transitive, etc.).
	Packages pnpmLockPackages `yaml:"packages"`
}

// pnpmLockPackages avoids yaml.v3's all-pairs duplicate-key check on large
// package stores. Keep the upstream decoder for small or unusual mappings so
// merge semantics, complex keys, and duplicate diagnostics stay unchanged.
type pnpmLockPackages map[string]yaml.Node

func (p *pnpmLockPackages) UnmarshalYAML(node *yaml.Node) error {
	fallback := func() error {
		return node.Decode((*map[string]yaml.Node)(p))
	}
	if node.Kind != yaml.MappingNode || len(node.Content) < 128 {
		return fallback()
	}
	seen := make(map[string]struct{}, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		// A string scalar has the same decoded map key as its node value. Other
		// tags, aliases, and merge keys need the upstream conversion rules.
		if key.Kind != yaml.ScalarNode || key.ShortTag() != "!!str" {
			return fallback()
		}
		if _, duplicate := seen[key.Value]; duplicate {
			return fallback()
		}
		seen[key.Value] = struct{}{}
	}
	if *p == nil {
		*p = make(pnpmLockPackages, len(seen))
	}
	for i := 0; i < len(node.Content); i += 2 {
		(*p)[node.Content[i].Value] = *node.Content[i+1]
	}
	return nil
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

type pnpmLockPackageID struct {
	name    string
	version string
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

func newPNPMLockParser(raw pnpmLockMatcherConfig) (sourceAnalyzer, error) {
	return pnpmLockParser{}, nil
}

func (p pnpmLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var file pnpmLockFile
	if err := yaml.Unmarshal(content, &file); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse yaml file %q: %w", path, err)
	}
	if file.LockfileVersion == "" {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0)
	seen := make(map[pnpmLockPackageID]struct{})

	if importer, ok := file.Importers["."]; ok {
		dependencies = appendPNPMLockDependencies(dependencies, seen, "dependencies", importer.Dependencies)
		dependencies = appendPNPMLockDependencies(dependencies, seen, "devDependencies", importer.DevDependencies)
		dependencies = appendPNPMLockDependencies(dependencies, seen, "optionalDependencies", importer.OptionalDependencies)
	} else {
		dependencies = appendPNPMLockDependencies(dependencies, seen, "dependencies", file.Dependencies)
		dependencies = appendPNPMLockDependencies(dependencies, seen, "devDependencies", file.DevDependencies)
		dependencies = appendPNPMLockDependencies(dependencies, seen, "optionalDependencies", file.OptionalDependencies)
	}

	dependencies = appendPNPMLockTransitiveFromPackages(dependencies, seen, file.Packages, file.LockfileVersion)

	if len(dependencies) == 0 {
		return sourceAnalyzerResult{
			Recognized: true,
			Analysis:   SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
		}, nil
	}

	return sourceAnalyzerResult{
		Dependencies: dependencies,
		Recognized:   true,
		Analysis:     SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
	}, nil
}

func appendPNPMLockTransitiveFromPackages(dependencies []DependencyReference, seen map[pnpmLockPackageID]struct{}, packages map[string]yaml.Node, lockfileVersion string) []DependencyReference {
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
		packageKey, _, _ := strings.Cut(key, "(")
		name, version := pnpmLockPackageKeyNameVersion(packageKey, lockfileVersion)
		version, _ = pnpmLockVersionAndPeerContext(version)
		if name == "" {
			continue
		}
		raw := name
		if version != "" {
			raw = name + "@" + version
		}
		id := pnpmLockPackageID{name: name, version: version}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		dep := DependencyReference{Raw: raw, Name: name}
		if version != "" {
			dep.Version = version
		}
		dependencies = append(dependencies, dep)
	}
	return dependencies
}

func pnpmLockVersionAndPeerContext(version string) (string, string) {
	if i := strings.IndexAny(version, "(_"); i != -1 {
		return version[:i], version[i:]
	}
	return version, ""
}

// pnpmLockPackageKeyNameVersion parses a `packages` / lockfile key into name and resolved version.
// v5: "/react/1.0.0". v6: "/react@1.0.0". v9+: "react@1.0.0".
func pnpmLockPackageKeyNameVersion(key, lockfileVersion string) (name, version string) {
	if key == "" {
		return "", ""
	}
	major, _, _ := strings.Cut(lockfileVersion, ".")
	switch major {
	case "5":
		return pnpmLockPackageKeyNameVersionV5Path(key)
	case "6":
		key = strings.TrimPrefix(key, "/")
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

func pnpmLockLocatorNameVersion(key string) (name, version string) {
	key = strings.TrimPrefix(key, "/")
	start := 0
	if strings.HasPrefix(key, "@") {
		start = strings.Index(key, "/") + 1
		if start == 0 {
			return key, ""
		}
	}
	i := strings.IndexAny(key[start:], "@/")
	if i == -1 {
		return key, ""
	}
	i += start
	return key[:i], key[i+1:]
}

func appendPNPMLockDependencies(dependencies []DependencyReference, seen map[pnpmLockPackageID]struct{}, sourceGroup string, values map[string]pnpmLockDependency) []DependencyReference {
	for _, name := range sortedPNPMLockDependencyNames(values) {
		dep := pnpmLockImporterDependency(name, values[name], sourceGroup)
		seen[pnpmLockPackageID{name: dep.Name, version: dep.Version}] = struct{}{}
		dependencies = append(dependencies, dep)
	}
	return dependencies
}

func pnpmLockImporterDependency(name string, value pnpmLockDependency, sourceGroup string) DependencyReference {
	dep := DependencyReference{
		Raw:         formatPNPMLockDependencyName(name, value),
		Name:        name,
		Version:     value.Version,
		SourceGroup: sourceGroup,
	}
	if value.Version == "" {
		dep.VersionConstraint = value.Specifier
		return dep
	}
	attributes := make(map[string]string)
	if value.Specifier != "" {
		attributes["specifier"] = value.Specifier
	}
	switch {
	case strings.HasPrefix(value.Version, "link:"), strings.HasPrefix(value.Version, "file:"):
		protocol, path, _ := strings.Cut(value.Version, ":")
		dep.Version = ""
		dep.OriginKind = OriginPath
		if protocol == "link" && strings.HasPrefix(value.Specifier, "workspace:") {
			dep.OriginKind = OriginWorkspace
		}
		attributes["protocol"] = protocol
		attributes["path"] = path
	case !strings.Contains(value.Version, ":"):
		version, peerContext := pnpmLockVersionAndPeerContext(value.Version)
		// A native version can contain peer names with @. Parse aliases only
		// after ruling out that version prefix, so underscores in names survive.
		if !semver.IsValid("v" + strings.TrimPrefix(version, "v")) {
			if targetName, targetVersion := pnpmLockLocatorNameVersion(value.Version); targetVersion != "" {
				dep.Name = targetName
				version, peerContext = pnpmLockVersionAndPeerContext(targetVersion)
			}
		}
		dep.Version = version
		if dep.Name == name && strings.HasPrefix(value.Specifier, "npm:") {
			if targetName, _ := parseNPMAlias(strings.TrimPrefix(value.Specifier, "npm:")); targetName != "" {
				dep.Name = targetName
			}
		}
		if dep.Name != name {
			attributes["declared_name"] = name
		}
		if peerContext != "" {
			attributes["peer_context"] = peerContext
		}
	}
	if len(attributes) > 0 {
		dep.Attributes = attributes
	}
	return dep
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
