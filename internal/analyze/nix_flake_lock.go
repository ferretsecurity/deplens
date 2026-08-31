package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
)

type nixFlakeLockParserConfig struct{}

type nixFlakeLockParser struct{}

type nixFlakeLockFile struct {
	Nodes   map[string]json.RawMessage `json:"nodes"`
	Root    string                     `json:"root"`
	Version int                        `json:"version"`
}

type nixFlakeLockNode struct {
	Inputs map[string]json.RawMessage `json:"inputs"`
	Locked *nixFlakeLockLocked        `json:"locked"`
}

type nixFlakeLockLocked struct {
	Type  string `json:"type"`
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Rev   string `json:"rev"`
}

func newNixFlakeLockParser(nixFlakeLockParserConfig) (sourceAnalyzer, error) {
	return nixFlakeLockParser{}, nil
}

func (nixFlakeLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var lock nixFlakeLockFile
	if err := json.Unmarshal(content, &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Nix flake lockfile %q: %w", path, err)
	}
	if lock.Version == 0 || strings.TrimSpace(lock.Root) == "" || len(lock.Nodes) == 0 {
		return sourceAnalyzerResult{}, nil
	}

	rootNode, ok := nixFlakeLockNodeAt(lock.Nodes, lock.Root)
	if !ok {
		return sourceAnalyzerResult{}, nil
	}
	directNodes := nixFlakeLockInputNodeNames(rootNode.Inputs)

	dependencies := make([]DependencyReference, 0, len(lock.Nodes)-1)
	incomplete := make([]string, 0)
	for _, nodeName := range sortedRawMessageKeys(lock.Nodes) {
		if nodeName == lock.Root {
			continue
		}
		node, ok := nixFlakeLockNodeAt(lock.Nodes, nodeName)
		if !ok || node.Locked == nil {
			continue
		}

		dependency, message := nixFlakeLockDependency(nodeName, *node.Locked, directNodes[nodeName])
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func nixFlakeLockNodeAt(nodes map[string]json.RawMessage, name string) (nixFlakeLockNode, bool) {
	rawNode, ok := nodes[name]
	if !ok {
		return nixFlakeLockNode{}, false
	}
	var node nixFlakeLockNode
	if err := json.Unmarshal(rawNode, &node); err != nil {
		return nixFlakeLockNode{}, false
	}
	return node, true
}

func nixFlakeLockInputNodeNames(inputs map[string]json.RawMessage) map[string]bool {
	names := make(map[string]bool)
	for _, rawInput := range inputs {
		var name string
		if json.Unmarshal(rawInput, &name) == nil && name != "" {
			names[name] = true
		}
	}
	return names
}

func nixFlakeLockDependency(nodeName string, locked nixFlakeLockLocked, direct bool) (DependencyReference, string) {
	if strings.TrimSpace(locked.Type) != "github" {
		return DependencyReference{}, "nodes." + nodeName + ".locked.type: unsupported value " + fmt.Sprintf("%q", locked.Type)
	}

	owner := strings.TrimSpace(locked.Owner)
	repository := strings.TrimSpace(locked.Repo)
	revision := strings.TrimSpace(locked.Rev)
	if owner == "" || repository == "" || revision == "" {
		return DependencyReference{}, "nodes." + nodeName + ".locked: GitHub owner, repo, and rev are required"
	}

	name := owner + "/" + repository
	relationship := RelationshipTransitive
	if direct {
		relationship = RelationshipDirect
	}
	return DependencyReference{
		PackageType:  "github",
		Raw:          name + "@" + revision,
		Name:         name,
		Version:      revision,
		SourceGroup:  "nodes",
		OriginKind:   OriginGit,
		Relationship: relationship,
		Scope:        ScopeRuntime,
		Attributes: map[string]string{
			"source_url":      "https://github.com/" + name,
			"source_ref":      revision,
			"source_ref_kind": "commit",
		},
	}, ""
}
