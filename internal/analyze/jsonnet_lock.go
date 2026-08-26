package analyze

import (
	"encoding/json"
	"fmt"
)

type jsonnetLockParserConfig struct{}

type jsonnetLockParser struct{}

func newJSONNetLockParser(jsonnetLockParserConfig) (sourceAnalyzer, error) {
	return jsonnetLockParser{}, nil
}

func (jsonnetLockParser) Analyze(pathname string, content []byte) (sourceAnalyzerResult, error) {
	var lock map[string]json.RawMessage
	if err := json.Unmarshal(content, &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse jsonnet-bundler lockfile %q: %w", pathname, err)
	}

	rawDependencies, exists := lock["dependencies"]
	if !exists || isJSONNull(rawDependencies) {
		return semanticAnalyzerResult(nil, nil), nil
	}

	var entries []json.RawMessage
	if err := json.Unmarshal(rawDependencies, &entries); err != nil {
		return semanticAnalyzerResult(nil, []string{"dependencies: expected an array of resolved dependencies"}), nil
	}

	dependencies := make([]DependencyReference, 0, len(entries))
	incomplete := make([]string, 0)
	for index, rawEntry := range entries {
		dependency, message := jsonnetLockDependency(rawEntry, index)
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func jsonnetLockDependency(rawEntry json.RawMessage, index int) (DependencyReference, string) {
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
	if version == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d].version: required", index)
	}
	checksum, message := jsonnetBundlerOptionalString(entry, "sum", index)
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
	rawGit, exists := source["git"]
	if !exists || isJSONNull(rawGit) {
		return DependencyReference{}, fmt.Sprintf("dependencies[%d].source: expected git source", index)
	}

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

	attributes := map[string]string{
		"source_url":      remote,
		"source_ref":      version,
		"source_ref_kind": "commit",
	}
	if subdir != "" {
		attributes["source_path"] = subdir
	}
	if checksum != "" {
		attributes["checksum"] = checksum
	}

	return DependencyReference{
		Raw:          name + "@" + version,
		Name:         name,
		Version:      version,
		SourceGroup:  "dependencies",
		OriginKind:   OriginGit,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
		Attributes:   attributes,
	}, ""
}
