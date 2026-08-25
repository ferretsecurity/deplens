package analyze

import (
	"fmt"
	"strconv"
)

type erlangRebarLockMatcherConfig struct{}

type erlangRebarLockParser struct{}

func newErlangRebarLockParser(erlangRebarLockMatcherConfig) (sourceAnalyzer, error) {
	return erlangRebarLockParser{}, nil
}

func (erlangRebarLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	terms, err := parseErlangTerms(content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse rebar.lock file %q: %w", path, err)
	}

	entries, ok := rebarLockEntries(terms)
	if !ok {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(entries.values))
	incomplete := make([]string, 0)
	for _, entry := range entries.values {
		dependency, message := rebarLockDependency(entry)
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func rebarLockEntries(terms []erlangTerm) (erlangTerm, bool) {
	if len(terms) == 0 {
		return erlangTerm{}, false
	}
	if terms[0].kind == erlangList {
		return terms[0], true
	}
	if terms[0].kind != erlangTuple || len(terms[0].values) != 2 || !isErlangName(terms[0].values[0]) || terms[0].values[1].kind != erlangList {
		return erlangTerm{}, false
	}
	return terms[0].values[1], true
}

func rebarLockDependency(entry erlangTerm) (DependencyReference, string) {
	if entry.kind != erlangTuple || len(entry.values) != 3 || !isErlangName(entry.values[0]) {
		return DependencyReference{}, "dependencies: expected {name, source, level} entry"
	}

	name := entry.values[0].value
	relationship, ok := rebarLockRelationship(entry.values[2])
	if !ok {
		return DependencyReference{}, fmt.Sprintf("dependencies.%s: level must be a non-negative integer", name)
	}

	if source, ok := rebarGitSource(entry.values[1]); ok {
		dependency := rebarGitDependency(name, source, "dependencies")
		dependency.Relationship = relationship
		if source.ref != "" {
			dependency.Raw = name + "@" + source.ref
			dependency.Version = source.ref
		}
		return dependency, ""
	}

	if entry.values[1].kind != erlangTuple || len(entry.values[1].values) != 3 || erlangAtomValue(entry.values[1].values[0]) != "pkg" {
		return DependencyReference{}, fmt.Sprintf("dependencies.%s: unsupported dependency source", name)
	}
	packageName, packageNameOK := erlangTextValue(entry.values[1].values[1])
	version, versionOK := erlangTextValue(entry.values[1].values[2])
	if !packageNameOK || packageName == "" || !versionOK || version == "" {
		return DependencyReference{}, fmt.Sprintf("dependencies.%s: package name and version are required", name)
	}

	dependency := DependencyReference{
		Raw:          packageName + "@" + version,
		Name:         packageName,
		Version:      version,
		SourceGroup:  "dependencies",
		OriginKind:   OriginRegistry,
		Relationship: relationship,
		Scope:        ScopeRuntime,
	}
	if packageName != name {
		dependency.Attributes = map[string]string{"declared_name": name}
	}
	return dependency, ""
}

func rebarLockRelationship(term erlangTerm) (Relationship, bool) {
	level, err := strconv.Atoi(term.value)
	if !isErlangName(term) || err != nil || level < 0 {
		return "", false
	}
	if level == 0 {
		return RelationshipDirect, true
	}
	return RelationshipTransitive, true
}
