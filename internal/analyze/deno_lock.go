package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
)

type denoLockParser struct{}

type denoLockPackages struct {
	JSR map[string]json.RawMessage `json:"jsr"`
	NPM map[string]json.RawMessage `json:"npm"`
}

type denoLockFile struct {
	Version  string                     `json:"version"`
	Remote   map[string]json.RawMessage `json:"remote"`
	Packages denoLockPackages           `json:"packages"`
	JSR      map[string]json.RawMessage `json:"jsr"`
	NPM      map[string]json.RawMessage `json:"npm"`
}

func newDenoLockParser(denoLockParserConfig) (sourceAnalyzer, error) {
	return denoLockParser{}, nil
}

func (denoLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var lock denoLockFile
	if err := json.Unmarshal(content, &lock); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Deno lockfile %q: %w", path, err)
	}
	if !isDenoLockVersion(lock.Version) {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(lock.Remote)+len(lock.Packages.JSR)+len(lock.Packages.NPM)+len(lock.JSR)+len(lock.NPM))
	seen := make(map[string]struct{})
	dependencies = appendDenoLockRemoteDependencies(dependencies, seen, lock.Remote)
	dependencies = appendDenoLockRegistryDependencies(dependencies, seen, "jsr", lock.Packages.JSR)
	dependencies = appendDenoLockRegistryDependencies(dependencies, seen, "npm", lock.Packages.NPM)
	dependencies = appendDenoLockRegistryDependencies(dependencies, seen, "jsr", lock.JSR)
	dependencies = appendDenoLockRegistryDependencies(dependencies, seen, "npm", lock.NPM)
	sortDependencyReferences(dependencies)

	return semanticAnalyzerResult(dependencies, nil), nil
}

func isDenoLockVersion(version string) bool {
	switch version {
	case "1", "2", "3", "4", "5":
		return true
	default:
		return false
	}
}

func appendDenoLockRemoteDependencies(dependencies []DependencyReference, seen map[string]struct{}, remote map[string]json.RawMessage) []DependencyReference {
	for _, sourceURL := range sortedStringKeys(remote) {
		sourceURL = strings.TrimSpace(sourceURL)
		if sourceURL == "" {
			continue
		}
		dependencies = appendUniqueDependency(dependencies, seen, "remote\x00"+sourceURL, DependencyReference{
			Raw:          sourceURL,
			SourceGroup:  "remote",
			OriginKind:   OriginURL,
			Relationship: RelationshipInconclusive,
			Scope:        ScopeRuntime,
			Attributes:   map[string]string{"source_url": sourceURL},
		})
	}
	return dependencies
}

func appendDenoLockRegistryDependencies(dependencies []DependencyReference, seen map[string]struct{}, registry string, packages map[string]json.RawMessage) []DependencyReference {
	for _, packageKey := range sortedStringKeys(packages) {
		name, version := splitDenoLockPackageKey(packageKey)
		if name == "" || version == "" {
			continue
		}
		dependency := DependencyReference{
			Raw:          name + "@" + version,
			Name:         name,
			Version:      version,
			SourceGroup:  registry,
			OriginKind:   OriginRegistry,
			Relationship: RelationshipInconclusive,
			Scope:        ScopeRuntime,
			Attributes:   map[string]string{"registry": registry},
		}
		if registry == "npm" {
			dependency.PackageType = "npm"
		}
		dependencies = appendUniqueDependency(dependencies, seen, registry+"\x00"+dependency.Raw, dependency)
	}
	return dependencies
}

func splitDenoLockPackageKey(value string) (name, version string) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "@") {
		if index := strings.LastIndex(value[1:], "@"); index > 0 {
			return value[:index+1], value[index+2:]
		}
		return "", ""
	}
	if index := strings.LastIndex(value, "@"); index > 0 {
		return value[:index], value[index+1:]
	}
	return "", ""
}
