package analyze

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type conanLockParser struct{}

type conanLockFile struct {
	Version        string          `json:"version"`
	Requires       []string        `json:"requires"`
	BuildRequires  []string        `json:"build_requires"`
	PythonRequires []string        `json:"python_requires"`
	ConfigRequires []string        `json:"config_requires"`
	GraphLock      *conanLockGraph `json:"graph_lock"`
}

type conanLockGraph struct {
	Nodes map[string]conanLockGraphNode `json:"nodes"`
}

type conanLockGraphNode struct {
	Ref           string   `json:"ref"`
	Requires      []string `json:"requires"`
	BuildRequires []string `json:"build_requires"`
	PackageID     string   `json:"package_id"`
	PackageRev    string   `json:"prev"`
	Context       string   `json:"context"`
}

type conanLockDependencyGroup struct {
	Name  string
	Refs  []string
	Scope DependencyScope
}

func newConanLockParser(conanLockMatcherConfig) (sourceAnalyzer, error) {
	return conanLockParser{}, nil
}

func (conanLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var file conanLockFile
	if err := json.Unmarshal(content, &file); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Conan lockfile %q: %w", path, err)
	}

	var dependencies []DependencyReference
	var incomplete []string
	switch file.Version {
	case "0.5":
		dependencies, incomplete = conanLockV05Dependencies(file)
	case "0.4":
		if file.GraphLock == nil || file.GraphLock.Nodes == nil {
			return sourceAnalyzerResult{}, nil
		}
		dependencies, incomplete = conanLockV04Dependencies(file.GraphLock)
	default:
		return sourceAnalyzerResult{}, nil
	}

	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func conanLockV05Dependencies(file conanLockFile) ([]DependencyReference, []string) {
	groups := []conanLockDependencyGroup{
		{Name: "requires", Refs: file.Requires, Scope: ScopeRuntime},
		{Name: "build_requires", Refs: file.BuildRequires, Scope: ScopeBuild},
		{Name: "python_requires", Refs: file.PythonRequires, Scope: ScopeDevelopment},
		{Name: "config_requires", Refs: file.ConfigRequires, Scope: ScopeDevelopment},
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, group := range groups {
		for _, reference := range group.Refs {
			dependency, ok := conanLockReference(reference)
			if !ok {
				incomplete = append(incomplete, fmt.Sprintf("Conan lockfile %s reference %q is invalid", group.Name, reference))
				continue
			}
			dependency.SourceGroup = group.Name
			dependency.Scope = group.Scope
			dependencies = append(dependencies, dependency)
		}
	}
	sortDependencyReferences(dependencies)
	return dependencies, incomplete
}

func conanLockV04Dependencies(graph *conanLockGraph) ([]DependencyReference, []string) {
	root := graph.Nodes["0"]
	directRuntime := conanNodeReferences(root.Requires)
	directBuild := conanNodeReferences(root.BuildRequires)

	nodeIDs := make([]string, 0, len(graph.Nodes))
	for nodeID := range graph.Nodes {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Strings(nodeIDs)

	dependencies := make([]DependencyReference, 0, len(nodeIDs))
	incomplete := make([]string, 0)
	for _, nodeID := range nodeIDs {
		node := graph.Nodes[nodeID]
		if node.Ref == "" {
			continue
		}
		dependency, ok := conanLockReference(node.Ref)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("Conan graph lock node %s reference %q is invalid", nodeID, node.Ref))
			continue
		}
		dependency.SourceGroup = "graph_lock.nodes"
		dependency.Relationship = RelationshipTransitive
		if node.Context == "build" {
			dependency.Scope = ScopeBuild
		} else {
			dependency.Scope = ScopeRuntime
		}
		if _, ok := directRuntime[nodeID]; ok {
			dependency.Relationship = RelationshipDirect
		}
		if _, ok := directBuild[nodeID]; ok {
			dependency.Relationship = RelationshipDirect
			dependency.Scope = ScopeBuild
		}
		if node.PackageID != "" || node.PackageRev != "" {
			dependency.Attributes = make(map[string]string)
			if node.PackageID != "" {
				dependency.Attributes["package_id"] = node.PackageID
			}
			if node.PackageRev != "" {
				dependency.Attributes["package_revision"] = node.PackageRev
			}
		}
		dependencies = append(dependencies, dependency)
	}
	sortDependencyReferences(dependencies)
	return dependencies, incomplete
}

func conanNodeReferences(references []string) map[string]struct{} {
	values := make(map[string]struct{}, len(references))
	for _, reference := range references {
		values[reference] = struct{}{}
	}
	return values
}

func conanLockReference(value string) (DependencyReference, bool) {
	raw := strings.TrimSpace(value)
	name, remainder, ok := strings.Cut(raw, "/")
	if !ok || name == "" || remainder == "" {
		return DependencyReference{}, false
	}

	versionEnd := strings.IndexAny(remainder, "@#:%")
	if versionEnd < 0 {
		versionEnd = len(remainder)
	}
	version := remainder[:versionEnd]
	if version == "" {
		return DependencyReference{}, false
	}

	dependency := DependencyReference{Raw: raw, Name: name, Version: version, OriginKind: OriginRegistry}
	if revision, ok := conanReferencePart(remainder, '#', '%'); ok {
		dependency.Attributes = map[string]string{"recipe_revision": revision}
	}
	return dependency, true
}

func conanReferencePart(value string, start, end byte) (string, bool) {
	index := strings.IndexByte(value, start)
	if index < 0 {
		return "", false
	}
	part := value[index+1:]
	if endIndex := strings.IndexByte(part, end); endIndex >= 0 {
		part = part[:endIndex]
	}
	if part == "" {
		return "", false
	}
	return part, true
}
