package analyze

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

type jsonnetBundlerParserConfig struct{}

type jsonnetBundlerParser struct{}

func newJSONNetBundlerParser(jsonnetBundlerParserConfig) (sourceAnalyzer, error) {
	return jsonnetBundlerParser{}, nil
}

func (jsonnetBundlerParser) Analyze(pathname string, content []byte) (sourceAnalyzerResult, error) {
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(content, &manifest); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse jsonnet-bundler manifest %q: %w", pathname, err)
	}

	rawDependencies, exists := manifest["dependencies"]
	if !exists || isJSONNull(rawDependencies) {
		return semanticAnalyzerResult(nil, nil), nil
	}

	var entries []json.RawMessage
	if err := json.Unmarshal(rawDependencies, &entries); err != nil {
		return semanticAnalyzerResult(nil, []string{"dependencies: expected an array of dependency declarations"}), nil
	}

	dependencies := make([]DependencyReference, 0, len(entries))
	incomplete := make([]string, 0)
	for index, rawEntry := range entries {
		dependency, message := jsonnetBundlerDependency(rawEntry, index)
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func jsonnetBundlerDependency(rawEntry json.RawMessage, index int) (DependencyReference, string) {
	var entry map[string]json.RawMessage
	if err := json.Unmarshal(rawEntry, &entry); err != nil {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d]: expected an object declaration", index)
	}

	declaredName, message := jsonnetBundlerOptionalString(entry, "name", index)
	if message != "" {
		return DependencyReference{}, message
	}
	version, message := jsonnetBundlerOptionalString(entry, "version", index)
	if message != "" {
		return DependencyReference{}, message
	}

	rawSource, exists := entry["source"]
	if !exists || isJSONNull(rawSource) {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d].source: required", index)
	}
	var source map[string]json.RawMessage
	if err := json.Unmarshal(rawSource, &source); err != nil {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d].source: expected an object", index)
	}

	if rawGit, exists := source["git"]; exists && !isJSONNull(rawGit) {
		return jsonnetBundlerGitDependency(rawGit, index, declaredName, version)
	}
	if rawLocal, exists := source["local"]; exists && !isJSONNull(rawLocal) {
		return jsonnetBundlerLocalDependency(rawLocal, index, declaredName, version)
	}
	return DependencyReference{}, fmt.Sprintf("dependencies[%d].source: expected git or local source", index)
}

func jsonnetBundlerGitDependency(rawGit json.RawMessage, index int, declaredName, version string) (DependencyReference, string) {
	var git map[string]json.RawMessage
	if err := json.Unmarshal(rawGit, &git); err != nil {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d].source.git: expected an object", index)
	}
	remote, message := jsonnetBundlerRequiredString(git, "remote", index, "source.git")
	if message != "" {
		return DependencyReference{}, message
	}
	subdir, message := jsonnetBundlerOptionalStringAt(git, "subdir", index, "source.git")
	if message != "" {
		return DependencyReference{}, message
	}

	name := declaredName
	if name == "" {
		name = jsonnetBundlerDerivedName(remote)
	}
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d]: name is required when it cannot be derived from the Git remote", index)
	}

	attributes := map[string]string{"source_url": remote}
	if subdir != "" {
		attributes["source_path"] = subdir
	}
	if version != "" {
		attributes["source_ref"] = version
		attributes["source_ref_kind"] = "version"
	}
	return jsonnetBundlerDependencyReference(name, remote, version, OriginGit, attributes), ""
}

func jsonnetBundlerLocalDependency(rawLocal json.RawMessage, index int, declaredName, version string) (DependencyReference, string) {
	var local map[string]json.RawMessage
	if err := json.Unmarshal(rawLocal, &local); err != nil {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d].source.local: expected an object", index)
	}
	directory, message := jsonnetBundlerRequiredString(local, "directory", index, "source.local")
	if message != "" {
		return DependencyReference{}, message
	}

	name := declaredName
	if name == "" {
		name = jsonnetBundlerDerivedName(directory)
	}
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d]: name is required when it cannot be derived from the local directory", index)
	}

	attributes := map[string]string{"path": directory}
	if version != "" {
		attributes["source_ref"] = version
		attributes["source_ref_kind"] = "version"
	}
	return jsonnetBundlerDependencyReference(name, directory, version, OriginPath, attributes), ""
}

func jsonnetBundlerDependencyReference(name, source, version string, origin OriginKind, attributes map[string]string) DependencyReference {
	dependency := DependencyReference{
		Raw:               name + "@" + source,
		Name:              name,
		VersionConstraint: version,
		SourceGroup:       "dependencies",
		OriginKind:        origin,
		Relationship:      RelationshipDirect,
		Scope:             ScopeRuntime,
		Attributes:        attributes,
	}
	return dependency
}

func jsonnetBundlerRequiredString(values map[string]json.RawMessage, key string, index int, group string) (string, string) {
	value, message := jsonnetBundlerOptionalStringAt(values, key, index, group)
	if message != "" {
		return "", message
	}
	if value == "" {
		return "", fmt.Sprintf("dependencies[%d].%s.%s: required", index, group, key)
	}
	return value, ""
}

func jsonnetBundlerOptionalString(values map[string]json.RawMessage, key string, index int) (string, string) {
	return jsonnetBundlerOptionalStringAt(values, key, index, "")
}

func jsonnetBundlerOptionalStringAt(values map[string]json.RawMessage, key string, index int, group string) (string, string) {
	rawValue, exists := values[key]
	if !exists || isJSONNull(rawValue) {
		return "", ""
	}
	var value string
	if err := json.Unmarshal(rawValue, &value); err != nil {
		field := key
		if group != "" {
			field = group + "." + key
		}
		return "", fmt.Sprintf("dependencies[%d].%s: expected a string", index, field)
	}
	return strings.TrimSpace(value), ""
}

func jsonnetBundlerDerivedName(source string) string {
	clean := strings.TrimSuffix(strings.TrimSpace(source), "/")
	clean = strings.TrimSuffix(clean, ".git")
	return strings.TrimSpace(path.Base(clean))
}
