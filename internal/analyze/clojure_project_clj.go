package analyze

import (
	"fmt"
	"strings"
)

type clojureProjectCLJMatcherConfig struct{}

type clojureProjectCLJParser struct{}

func newClojureProjectCLJParser(clojureProjectCLJMatcherConfig) (sourceAnalyzer, error) {
	return clojureProjectCLJParser{}, nil
}

func (clojureProjectCLJParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	forms, err := parseClojureForms(content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Clojure project.clj file %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seenDependencies := make(map[string]struct{})
	seenMessages := make(map[string]struct{})
	for _, form := range forms {
		if form.kind != clojureList || len(form.children) == 0 || clojureNodeValue(form.children[0]) != "defproject" {
			continue
		}
		collectClojureProjectSections(form, "", ScopeRuntime, &dependencies, &incomplete, seenDependencies, seenMessages)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func collectClojureProjectSections(node clojureNode, prefix string, scope DependencyScope, dependencies *[]DependencyReference, incomplete *[]string, seenDependencies, seenMessages map[string]struct{}) {
	for index := 0; index < len(node.children); index++ {
		key := clojureNodeValue(node.children[index])
		if index+1 < len(node.children) {
			switch key {
			case ":dependencies":
				collectClojureProjectDependencyVector(node.children[index+1], clojureProjectGroup(prefix, "dependencies"), scope, dependencies, incomplete, seenDependencies, seenMessages)
				index++
				continue
			case ":plugins":
				collectClojureProjectDependencyVector(node.children[index+1], clojureProjectGroup(prefix, "plugins"), ScopeBuild, dependencies, incomplete, seenDependencies, seenMessages)
				index++
				continue
			case ":profiles":
				collectClojureProjectProfiles(node.children[index+1], prefix, dependencies, incomplete, seenDependencies, seenMessages)
				index++
				continue
			}
		}
		collectClojureProjectSections(node.children[index], prefix, scope, dependencies, incomplete, seenDependencies, seenMessages)
	}
}

func collectClojureProjectProfiles(node clojureNode, prefix string, dependencies *[]DependencyReference, incomplete *[]string, seenDependencies, seenMessages map[string]struct{}) {
	entries, err := clojureEDNMapEntries(node)
	if err != nil {
		*incomplete = appendUniqueMessage(*incomplete, seenMessages, ":profiles is not a static map")
		return
	}
	for _, profile := range entries {
		name := strings.TrimPrefix(profile.key, ":")
		if name == "" || !strings.HasPrefix(profile.key, ":") {
			*incomplete = appendUniqueMessage(*incomplete, seenMessages, ":profiles contains a profile without a static name")
			continue
		}
		collectClojureProjectSections(profile.value, clojureProjectGroup(prefix, "profiles."+name), clojureProjectProfileScope(name), dependencies, incomplete, seenDependencies, seenMessages)
	}
}

func clojureProjectGroup(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func clojureProjectProfileScope(name string) DependencyScope {
	switch name {
	case "dev":
		return ScopeDevelopment
	case "test":
		return ScopeTest
	case "provided":
		return ScopeBuild
	default:
		return ScopeRuntime
	}
}

func collectClojureProjectDependencyVector(node clojureNode, group string, scope DependencyScope, dependencies *[]DependencyReference, incomplete *[]string, seenDependencies, seenMessages map[string]struct{}) {
	node = unwrapClojureReaderMacros(node)
	if node.kind != clojureVector {
		*incomplete = appendUniqueMessage(*incomplete, seenMessages, group+": expected a static dependency vector")
		return
	}
	for _, entry := range node.children {
		dependency, ok, message := clojureProjectDependency(entry, group, scope)
		if message != "" {
			*incomplete = appendUniqueMessage(*incomplete, seenMessages, message)
		}
		if !ok {
			continue
		}
		key := dependency.SourceGroup + "\x00" + dependency.Name + "\x00" + dependency.Raw
		*dependencies = appendUniqueDependency(*dependencies, seenDependencies, key, dependency)
	}
}

func clojureProjectDependency(entry clojureNode, group string, scope DependencyScope) (DependencyReference, bool, string) {
	entry = unwrapClojureReaderMacros(entry)
	if entry.kind != clojureVector || len(entry.children) == 0 {
		return DependencyReference{}, false, group + ": contains a non-vector declaration"
	}
	name := clojureNodeValue(unwrapClojureReaderMacros(entry.children[0]))
	if name == "" || strings.HasPrefix(name, ":") {
		return DependencyReference{}, false, group + ": contains a declaration without a static coordinate"
	}

	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        scope,
	}
	if len(entry.children) > 1 {
		version := clojureNodeValue(unwrapClojureReaderMacros(entry.children[1]))
		if version != "" && !strings.HasPrefix(version, ":") {
			dependency.Raw += "@" + version
			dependency.VersionConstraint = normalizeMavenManifestConstraint(version)
		}
	}
	return dependency, true, ""
}
