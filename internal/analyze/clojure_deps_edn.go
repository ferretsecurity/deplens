package analyze

import (
	"fmt"
	"strings"
)

type clojureDepsEDNMatcherConfig struct{}

type clojureDepsEDNParser struct{}

func newClojureDepsEDNParser(clojureDepsEDNMatcherConfig) (sourceAnalyzer, error) {
	return clojureDepsEDNParser{}, nil
}

func (clojureDepsEDNParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	forms, err := parseClojureForms(content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Clojure deps.edn file %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seenDependencies := make(map[string]struct{})
	seenMessages := make(map[string]struct{})
	for _, form := range forms {
		if form.kind != clojureMap {
			continue
		}
		entries, err := clojureEDNMapEntries(form)
		if err != nil {
			return sourceAnalyzerResult{}, err
		}
		for _, entry := range entries {
			switch entry.key {
			case ":deps":
				collectClojureDepsEDNGroup(entry.value, "deps", ScopeRuntime, &dependencies, &incomplete, seenDependencies, seenMessages)
			case ":aliases":
				collectClojureDepsEDNAliases(entry.value, &dependencies, &incomplete, seenDependencies, seenMessages)
			}
		}
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

type clojureEDNMapEntry struct {
	key   string
	value clojureNode
}

func clojureEDNMapEntries(node clojureNode) ([]clojureEDNMapEntry, error) {
	node = unwrapClojureReaderMacros(node)
	if node.kind != clojureMap {
		return nil, fmt.Errorf("expected an EDN map")
	}
	if len(node.children)%2 != 0 {
		return nil, fmt.Errorf("EDN map has an unmatched key")
	}
	entries := make([]clojureEDNMapEntry, 0, len(node.children)/2)
	for index := 0; index < len(node.children); index += 2 {
		entries = append(entries, clojureEDNMapEntry{
			key:   clojureNodeValue(unwrapClojureReaderMacros(node.children[index])),
			value: node.children[index+1],
		})
	}
	return entries, nil
}

func collectClojureDepsEDNAliases(node clojureNode, dependencies *[]DependencyReference, incomplete *[]string, seenDependencies, seenMessages map[string]struct{}) {
	aliases, err := clojureEDNMapEntries(node)
	if err != nil {
		*incomplete = appendUniqueMessage(*incomplete, seenMessages, ":aliases is not an EDN map")
		return
	}
	for _, alias := range aliases {
		if !strings.HasPrefix(alias.key, ":") {
			*incomplete = appendUniqueMessage(*incomplete, seenMessages, ":aliases contains an alias without a static name")
			continue
		}
		entries, err := clojureEDNMapEntries(alias.value)
		if err != nil {
			*incomplete = appendUniqueMessage(*incomplete, seenMessages, ":aliases."+strings.TrimPrefix(alias.key, ":")+" is not an EDN map")
			continue
		}
		aliasName := strings.TrimPrefix(alias.key, ":")
		for _, entry := range entries {
			switch entry.key {
			case ":deps", ":extra-deps", ":replace-deps":
				group := "aliases." + aliasName + "." + strings.TrimPrefix(entry.key, ":")
				collectClojureDepsEDNGroup(entry.value, group, clojureDepsEDNAliasScope(aliasName), dependencies, incomplete, seenDependencies, seenMessages)
			}
		}
	}
}

func clojureDepsEDNAliasScope(alias string) DependencyScope {
	switch alias {
	case "test":
		return ScopeTest
	case "dev":
		return ScopeDevelopment
	case "build", "deploy", "install":
		return ScopeBuild
	default:
		return ScopeRuntime
	}
}

func collectClojureDepsEDNGroup(node clojureNode, group string, scope DependencyScope, dependencies *[]DependencyReference, incomplete *[]string, seenDependencies, seenMessages map[string]struct{}) {
	entries, err := clojureEDNMapEntries(node)
	if err != nil {
		*incomplete = appendUniqueMessage(*incomplete, seenMessages, group+": expected an EDN dependency map")
		return
	}
	for _, entry := range entries {
		dependency, ok, message := clojureDepsEDNDependency(entry.key, entry.value, group, scope)
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

func clojureDepsEDNDependency(name string, node clojureNode, group string, scope DependencyScope) (DependencyReference, bool, string) {
	if name == "" || strings.HasPrefix(name, ":") {
		return DependencyReference{}, false, group + ": dependency declaration has no static library name"
	}
	entries, err := clojureEDNMapEntries(node)
	if err != nil {
		return DependencyReference{}, false, fmt.Sprintf("%s.%s: expected an EDN coordinate map", group, name)
	}

	coordinates := make(map[string]string, len(entries))
	for _, entry := range entries {
		value := clojureNodeValue(unwrapClojureReaderMacros(entry.value))
		if value != "" {
			coordinates[entry.key] = value
		}
	}
	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		Relationship: RelationshipDirect,
		Scope:        scope,
	}
	if version := strings.TrimSpace(coordinates[":mvn/version"]); version != "" {
		dependency.Raw += "@" + version
		dependency.VersionConstraint = normalizeMavenManifestConstraint(version)
		dependency.OriginKind = OriginRegistry
		return dependency, true, ""
	}

	gitURL := strings.TrimSpace(coordinates[":git/url"])
	gitTag := strings.TrimSpace(coordinates[":git/tag"])
	gitSHA := strings.TrimSpace(coordinates[":git/sha"])
	if gitURL != "" || gitTag != "" || gitSHA != "" {
		dependency.OriginKind = OriginGit
		dependency.Attributes = make(map[string]string)
		if gitURL != "" {
			dependency.Raw += "@" + gitURL
			dependency.Attributes["source_url"] = gitURL
		} else if gitTag != "" {
			dependency.Raw += "@" + gitTag
		}
		if gitTag != "" {
			dependency.Attributes["source_tag"] = gitTag
		}
		if gitSHA != "" {
			dependency.Attributes["source_ref"] = gitSHA
			dependency.Attributes["source_ref_kind"] = "revision"
		}
		return dependency, true, ""
	}

	if localRoot := strings.TrimSpace(coordinates[":local/root"]); localRoot != "" {
		dependency.Raw += "@" + localRoot
		dependency.OriginKind = OriginPath
		dependency.Attributes = map[string]string{"source_path": localRoot}
		return dependency, true, ""
	}
	return DependencyReference{}, false, fmt.Sprintf("%s.%s: unsupported dependency coordinate", group, name)
}
