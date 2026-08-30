package analyze

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type chefBerksfileLockParser struct{}

type chefBerksfileLockDeclaration struct {
	constraint string
	attributes map[string]string
}

type chefBerksfileLockJSONSource struct {
	LockedVersion string `json:"locked_version"`
	Git           string `json:"git"`
	Ref           string `json:"ref"`
	Path          string `json:"path"`
}

var (
	chefBerksfileLockDirect = regexp.MustCompile(`^ {2}(\S+)(?: \(([^)]+)\))?\s*$`)
	chefBerksfileLockGraph  = regexp.MustCompile(`^ {2}(\S+) \(([^)]+)\)\s*$`)
	chefBerksfileLockOption = regexp.MustCompile(`^ {4}([a-z_]+):\s*(.*?)\s*$`)
)

func newChefBerksfileLockParser(chefBerksfileLockMatcherConfig) (sourceAnalyzer, error) {
	return chefBerksfileLockParser{}, nil
}

func (chefBerksfileLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	trimmed := strings.TrimSpace(string(content))
	if strings.HasPrefix(trimmed, "{") {
		return parseChefBerksfileJSONLock(path, content)
	}
	return parseChefBerksfileTextLock(path, content)
}

func parseChefBerksfileJSONLock(path string, content []byte) (sourceAnalyzerResult, error) {
	var root struct {
		Sources json.RawMessage `json:"sources"`
	}
	if err := json.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Berksfile.lock JSON %q: %w", path, err)
	}
	if root.Sources == nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Berksfile.lock JSON %q: missing sources", path)
	}

	var sources map[string]chefBerksfileLockJSONSource
	if err := json.Unmarshal(root.Sources, &sources); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Berksfile.lock JSON %q sources: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0, len(sources))
	incomplete := make([]string, 0)
	for name, source := range sources {
		if name == "" || source.LockedVersion == "" {
			incomplete = append(incomplete, fmt.Sprintf("Berksfile.lock JSON source %q has no locked_version", name))
			continue
		}
		dependency := chefBerksfileLockDependency(name, source.LockedVersion, chefBerksfileLockDeclaration{
			attributes: map[string]string{"git": source.Git, "ref": source.Ref, "path": source.Path},
		}, true)
		dependencies = append(dependencies, dependency)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func parseChefBerksfileTextLock(path string, content []byte) (sourceAnalyzerResult, error) {
	declarations := make(map[string]chefBerksfileLockDeclaration)
	graph := make(map[string]string)
	section := ""
	lastDeclaration := ""
	recognized := false

	for _, line := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		switch line {
		case "DEPENDENCIES":
			section = "dependencies"
			lastDeclaration = ""
			recognized = true
			continue
		case "GRAPH":
			section = "graph"
			lastDeclaration = ""
			recognized = true
			continue
		}

		switch section {
		case "dependencies":
			if match := chefBerksfileLockDirect.FindStringSubmatch(line); match != nil {
				declarations[match[1]] = chefBerksfileLockDeclaration{constraint: strings.TrimSpace(match[2]), attributes: make(map[string]string)}
				lastDeclaration = match[1]
				continue
			}
			if match := chefBerksfileLockOption.FindStringSubmatch(line); match != nil && lastDeclaration != "" {
				declaration := declarations[lastDeclaration]
				declaration.attributes[match[1]] = match[2]
				declarations[lastDeclaration] = declaration
			}
		case "graph":
			if match := chefBerksfileLockGraph.FindStringSubmatch(line); match != nil {
				graph[match[1]] = match[2]
			}
		}
	}

	if !recognized {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Berksfile.lock %q: no recognized lockfile sections", path)
	}

	dependencies := make([]DependencyReference, 0, len(graph))
	incomplete := make([]string, 0)
	for name := range declarations {
		if _, found := graph[name]; !found {
			incomplete = append(incomplete, fmt.Sprintf("direct Berksfile.lock dependency %s has no resolved graph entry", name))
		}
	}
	for name, version := range graph {
		declaration, direct := declarations[name]
		dependencies = append(dependencies, chefBerksfileLockDependency(name, version, declaration, direct))
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func chefBerksfileLockDependency(name, version string, declaration chefBerksfileLockDeclaration, direct bool) DependencyReference {
	dependency := DependencyReference{
		Raw:          name + "@" + version,
		Name:         name,
		Version:      version,
		SourceGroup:  "default",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipTransitive,
		Scope:        ScopeRuntime,
	}
	if direct {
		dependency.Relationship = RelationshipDirect
		dependency.VersionConstraint = declaration.constraint
	}

	attributes := make(map[string]string)
	for key, value := range declaration.attributes {
		if value == "" {
			continue
		}
		switch key {
		case "git":
			dependency.OriginKind = OriginGit
			attributes["source_url"] = value
		case "path":
			dependency.OriginKind = OriginPath
			attributes["source_path"] = value
		case "revision", "ref":
			attributes["source_ref"] = value
			attributes["source_ref_kind"] = key
		case "tag":
			attributes["source_tag"] = value
		default:
			attributes[key] = value
		}
	}
	if len(attributes) > 0 {
		dependency.Attributes = attributes
	}
	return dependency
}
