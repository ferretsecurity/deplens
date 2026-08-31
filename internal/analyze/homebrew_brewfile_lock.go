package analyze

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type homebrewBrewfileLockMatcherConfig struct{}

type homebrewBrewfileLockParser struct{}

type homebrewBrewfileLockEntry struct {
	Version  string          `json:"version"`
	ID       json.RawMessage `json:"id"`
	Revision string          `json:"revision"`
	Bottle   json.RawMessage `json:"bottle"`
}

type homebrewBrewfileLegacyLockEntry struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Version string `json:"version"`
}

type homebrewBrewfileLockBottle struct {
	RootURL string                                    `json:"root_url"`
	Files   map[string]homebrewBrewfileLockBottleFile `json:"files"`
}

type homebrewBrewfileLockBottleFile struct {
	SHA256 string `json:"sha256"`
}

func newHomebrewBrewfileLockParser(homebrewBrewfileLockMatcherConfig) (sourceAnalyzer, error) {
	return homebrewBrewfileLockParser{}, nil
}

func (homebrewBrewfileLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Homebrew Brewfile lockfile %q: %w", path, err)
	}
	rawEntries, ok := root["entries"]
	if !ok || string(rawEntries) == "null" {
		return sourceAnalyzerResult{}, nil
	}

	var entries map[string]json.RawMessage
	if err := json.Unmarshal(rawEntries, &entries); err == nil && entries != nil {
		dependencies, incomplete := homebrewBrewfileLockGroupedDependencies(entries)
		return semanticAnalyzerResult(dependencies, incomplete), nil
	}

	var legacyEntries []homebrewBrewfileLegacyLockEntry
	if err := json.Unmarshal(rawEntries, &legacyEntries); err == nil && legacyEntries != nil {
		dependencies, incomplete := homebrewBrewfileLegacyLockDependencies(legacyEntries)
		return semanticAnalyzerResult(dependencies, incomplete), nil
	}
	return sourceAnalyzerResult{}, fmt.Errorf("parse Homebrew Brewfile lockfile %q: entries must be an object or array", path)
}

func homebrewBrewfileLockGroupedDependencies(entries map[string]json.RawMessage) ([]DependencyReference, []string) {
	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	knownGroups := map[string]struct{}{"brew": {}, "cask": {}, "mas": {}, "tap": {}}
	for _, group := range []string{"brew", "cask", "mas", "tap"} {
		rawEntries, ok := entries[group]
		if !ok || string(rawEntries) == "null" {
			continue
		}

		var entries map[string]json.RawMessage
		if err := json.Unmarshal(rawEntries, &entries); err != nil {
			incomplete = append(incomplete, fmt.Sprintf("entries.%s: expected an object of locked dependencies", group))
			continue
		}
		for _, name := range sortedRawMessageKeys(entries) {
			dependency, message := homebrewBrewfileLockDependency(group, name, entries[name])
			if message != "" {
				incomplete = append(incomplete, message)
				continue
			}
			dependencies = append(dependencies, dependency)
		}
	}

	for _, group := range sortedRawMessageKeys(entries) {
		if _, known := knownGroups[group]; !known && string(entries[group]) != "null" {
			incomplete = append(incomplete, fmt.Sprintf("entries.%s: unsupported Homebrew dependency group", group))
		}
	}

	sortDependencyReferences(dependencies)
	return dependencies, incomplete
}

func homebrewBrewfileLegacyLockDependencies(entries []homebrewBrewfileLegacyLockEntry) ([]DependencyReference, []string) {
	dependencies := make([]DependencyReference, 0, len(entries))
	incomplete := make([]string, 0)
	for index, entry := range entries {
		group := strings.TrimSpace(entry.Type)
		if group != "brew" && group != "cask" {
			incomplete = append(incomplete, fmt.Sprintf("entries[%d].type: unsupported Homebrew dependency group %q", index, group))
			continue
		}
		dependency, message := homebrewBrewfileLockDependencyEntry(group, entry.Name, homebrewBrewfileLockEntry{Version: entry.Version})
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}
	sortDependencyReferences(dependencies)
	return dependencies, incomplete
}

func homebrewBrewfileLockDependency(group, rawName string, rawEntry json.RawMessage) (DependencyReference, string) {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("entries.%s: dependency name is required", group)
	}

	var entry homebrewBrewfileLockEntry
	if err := json.Unmarshal(rawEntry, &entry); err != nil {
		return DependencyReference{}, fmt.Sprintf("entries.%s.%s: expected an object of locked dependency fields", group, name)
	}
	return homebrewBrewfileLockDependencyEntry(group, name, entry)
}

func homebrewBrewfileLockDependencyEntry(group, rawName string, entry homebrewBrewfileLockEntry) (DependencyReference, string) {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return DependencyReference{}, fmt.Sprintf("entries.%s: dependency name is required", group)
	}
	dependency := DependencyReference{
		PackageType:  "generic",
		Name:         name,
		SourceGroup:  group,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	switch group {
	case "brew", "cask":
		version := strings.TrimSpace(entry.Version)
		if version == "" {
			return DependencyReference{}, fmt.Sprintf("entries.%s.%s.version: resolved version is required", group, name)
		}
		dependency.Raw = name + "@" + version
		dependency.Version = version
		dependency.OriginKind = OriginRegistry
		attributes, message := homebrewBrewfileLockBottleAttributes(entry.Bottle, group, name)
		if message != "" {
			return DependencyReference{}, message
		}
		dependency.Attributes = attributes
	case "mas":
		id := homebrewBrewfileLockAppStoreID(entry.ID)
		if id == "" {
			return DependencyReference{}, fmt.Sprintf("entries.mas.%s.id: App Store ID is required", name)
		}
		dependency.Raw = name
		dependency.OriginKind = OriginRegistry
		dependency.Attributes = map[string]string{"app_store_id": id}
		if version := strings.TrimSpace(entry.Version); version != "" {
			dependency.Raw += "@" + version
			dependency.Version = version
		}
	case "tap":
		revision := strings.TrimSpace(entry.Revision)
		if revision == "" {
			return DependencyReference{}, fmt.Sprintf("entries.tap.%s.revision: resolved revision is required", name)
		}
		dependency.Raw = name + "@" + revision
		dependency.Version = revision
		dependency.OriginKind = OriginGit
		dependency.Attributes = map[string]string{"source_ref": revision, "source_ref_kind": "revision"}
	}
	return dependency, ""
}

func homebrewBrewfileLockBottleAttributes(raw json.RawMessage, group, name string) (map[string]string, string) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "false" {
		return nil, ""
	}

	var bottle homebrewBrewfileLockBottle
	if err := json.Unmarshal(raw, &bottle); err != nil {
		return nil, fmt.Sprintf("entries.%s.%s.bottle: expected false or an object of bottle metadata", group, name)
	}
	attributes := make(map[string]string)
	if rootURL := strings.TrimSpace(bottle.RootURL); rootURL != "" {
		attributes["source_url"] = rootURL
	}
	for _, platform := range sortedBottlePlatforms(bottle.Files) {
		if checksum := strings.TrimSpace(bottle.Files[platform].SHA256); checksum != "" {
			attributes["bottle_sha256_"+platform] = checksum
		}
	}
	if len(attributes) == 0 {
		return nil, ""
	}
	return attributes, ""
}

func homebrewBrewfileLockAppStoreID(raw json.RawMessage) string {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return ""
	}
	if strings.HasPrefix(value, "\"") {
		var id string
		if err := json.Unmarshal(raw, &id); err != nil {
			return ""
		}
		value = strings.TrimSpace(id)
	}
	if value == "" {
		return ""
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return ""
		}
	}
	return value
}

func sortedRawMessageKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func sortedBottlePlatforms(values map[string]homebrewBrewfileLockBottleFile) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
