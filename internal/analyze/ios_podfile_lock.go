package analyze

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type iosPodfileLockParser struct{}

var iosPodfileLockPod = regexp.MustCompile(`^(.+?) \(([^()]+)\)$`)

func newIOSPodfileLockParser(iosPodfileLockMatcherConfig) (sourceAnalyzer, error) {
	return iosPodfileLockParser{}, nil
}

func (iosPodfileLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse CocoaPods Podfile lockfile %q: %w", path, err)
	}

	root := iosPodfileLockRoot(&document)
	if root == nil || root.Kind != yaml.MappingNode {
		return sourceAnalyzerResult{}, nil
	}
	pods, hasPods := iosPodfileLockField(root, "PODS")
	_, hasChecksum := iosPodfileLockField(root, "PODFILE CHECKSUM")
	_, hasCocoaPods := iosPodfileLockField(root, "COCOAPODS")
	if !hasPods && !(hasChecksum && hasCocoaPods) {
		return sourceAnalyzerResult{}, nil
	}
	if !hasPods || pods.Kind == 0 || pods.Tag == "!!null" {
		return semanticAnalyzerResult([]DependencyReference{}, nil), nil
	}
	if pods.Kind != yaml.SequenceNode {
		return sourceAnalyzerResult{}, fmt.Errorf("parse CocoaPods Podfile lockfile %q: PODS must be a sequence", path)
	}

	dependencies := make([]DependencyReference, 0, len(pods.Content))
	incomplete := make([]string, 0)
	seen := make(map[string]struct{})
	for index, entry := range pods.Content {
		declaration, ok := iosPodfileLockDeclaration(entry)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("PODS[%d]: expected a resolved pod declaration", index))
			continue
		}
		dependency, ok := iosPodfileLockDependency(declaration)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("PODS[%d]: invalid resolved pod declaration %q", index, declaration))
			continue
		}
		dependencies = appendUniqueDependency(dependencies, seen, dependency.Raw, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func iosPodfileLockRoot(document *yaml.Node) *yaml.Node {
	if document.Kind == yaml.DocumentNode && len(document.Content) == 1 {
		return document.Content[0]
	}
	return nil
}

func iosPodfileLockField(root *yaml.Node, name string) (*yaml.Node, bool) {
	for index := 0; index+1 < len(root.Content); index += 2 {
		if root.Content[index].Value == name {
			return root.Content[index+1], true
		}
	}
	return nil, false
}

func iosPodfileLockDeclaration(entry *yaml.Node) (string, bool) {
	switch entry.Kind {
	case yaml.ScalarNode:
		return entry.Value, true
	case yaml.MappingNode:
		if len(entry.Content) == 2 && entry.Content[0].Kind == yaml.ScalarNode {
			return entry.Content[0].Value, true
		}
	}
	return "", false
}

func iosPodfileLockDependency(declaration string) (DependencyReference, bool) {
	match := iosPodfileLockPod.FindStringSubmatch(strings.TrimSpace(declaration))
	if match == nil {
		return DependencyReference{}, false
	}
	name := strings.TrimSpace(match[1])
	version := strings.TrimSpace(match[2])
	if name == "" || version == "" {
		return DependencyReference{}, false
	}
	return DependencyReference{
		Raw:          name + "@" + version,
		Name:         name,
		Version:      version,
		SourceGroup:  "PODS",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipInconclusive,
		Scope:        ScopeRuntime,
	}, true
}
