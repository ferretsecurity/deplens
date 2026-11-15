package analyze

import (
	"fmt"
	"strings"

	ini "gopkg.in/ini.v1"
)

type iniMatcherConfig struct {
	Queries []iniQueryConfig `yaml:"queries"`
}

type iniQueryConfig struct {
	Section string `yaml:"section"`
	Key     string `yaml:"key"`
}

type iniQueryParser struct {
	queries          []iniQuery
	recognizeMissing bool
}

type iniQuery struct {
	sourceGroup string
	key         string
	anyKey      bool
}

func newINIQueryParser(raw iniMatcherConfig) (sourceAnalyzer, error) {
	if len(raw.Queries) == 0 {
		return nil, fmt.Errorf("ini.queries: must contain at least one entry")
	}

	queries := make([]iniQuery, 0, len(raw.Queries))
	for queryIdx, query := range raw.Queries {
		if query.Section == "" {
			return nil, fmt.Errorf("ini.queries[%d].section: required", queryIdx)
		}
		if query.Key == "" {
			return nil, fmt.Errorf("ini.queries[%d].key: required", queryIdx)
		}
		if query.Key != "*" && strings.Contains(query.Key, "*") {
			return nil, fmt.Errorf("ini.queries[%d].key: wildcard must be exactly \"*\"", queryIdx)
		}

		queries = append(queries, iniQuery{
			sourceGroup: query.Section,
			key:         query.Key,
			anyKey:      query.Key == "*",
		})
	}

	return iniQueryParser{queries: queries}, nil
}

func (p iniQueryParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	// The INI library ends Python continuations at blank or unindented comment
	// lines. Python's config parser ignores these lines inside a value.
	lines := strings.Split(string(content), "\n")
	valueLines := lines[:0]
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, ";") {
			valueLines = append(valueLines, line)
		}
	}
	file, err := ini.LoadSources(ini.LoadOptions{
		AllowPythonMultilineValues: true,
		Insensitive:                true,
		IgnoreInlineComment:        true,
		// Python multiline parsing peeks ahead only within the reader buffer.
		ReaderBufferSize: len(content) + 1,
	}, []byte(strings.Join(valueLines, "\n")+"\n"))
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse ini file %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	found := false
	var groups []DependencyGroup
	duplicates := duplicateINIKeys(valueLines)
	for _, query := range p.queries {
		sourceGroup, err := file.GetSection(query.sourceGroup)
		if err != nil {
			groups = append(groups, DependencyGroup{Name: query.sourceGroup + "." + query.key, Location: query.sourceGroup + "." + query.key, State: GroupMissing})
			continue
		}

		keys := matchingINIKeys(sourceGroup, query)
		if len(keys) == 0 {
			state := GroupMissing
			if query.anyKey {
				state = GroupEmpty
			}
			groups = append(groups, DependencyGroup{Name: query.sourceGroup + "." + query.key, Location: query.sourceGroup + "." + query.key, State: state})
			continue
		}

		found = true
		for _, key := range keys {
			dependencies = append(dependencies, extractINIDependencies(key)...)
			group := iniDependencyGroup(query, key)
			if duplicates[group.Location] {
				group.Diagnostics = append(group.Diagnostics, Diagnostic{Severity: DiagnosticError, Code: "ini-generation-incomplete", Message: "duplicate declaration at " + group.Location})
			}
			groups = append(groups, group)
		}
	}

	if !found && !p.recognizeMissing {
		return sourceAnalyzerResult{}, nil
	}
	if len(dependencies) == 0 {
		return sourceAnalyzerResult{Analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}, Recognized: true, Groups: groups}, nil
	}
	return sourceAnalyzerResult{
		Dependencies: dependencies,
		Groups:       groups,
		Analysis:     SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		Recognized:   true,
	}, nil
}

func matchingINIKeys(sourceGroup *ini.Section, query iniQuery) []*ini.Key {
	if query.anyKey {
		return sourceGroup.Keys()
	}
	if !sourceGroup.HasKey(query.key) {
		return nil
	}
	return []*ini.Key{sourceGroup.Key(query.key)}
}

func extractINIDependencies(key *ini.Key) []DependencyReference {
	value := key.Value()
	multiline := strings.Contains(value, "\n")
	separator := ";"
	if multiline {
		separator = "\n"
	} else {
		value = normalizeINIDependency(value)
	}
	entries := strings.Split(value, separator)
	dependencies := make([]DependencyReference, 0, len(entries))
	for _, entry := range entries {
		raw := normalizeINIDependency(entry)
		if raw == "" {
			continue
		}
		parsed, err := parsePythonRequirement(raw)
		if err != nil && !multiline {
			continue
		}
		dependency := DependencyReference{Raw: raw}
		if err == nil {
			dependency.Name = parsed.Name
			dependency.VersionConstraint = parsed.VersionConstraint
			dependency.Attributes = parsed.Attributes
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies
}

func normalizeINIDependency(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
		return ""
	}

	commentIdx := findInlineCommentStart(trimmed)
	if commentIdx >= 0 {
		trimmed = strings.TrimSpace(trimmed[:commentIdx])
	}
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "file:") || strings.HasPrefix(trimmed, "%(") {
		return ""
	}
	return trimmed
}

func findInlineCommentStart(value string) int {
	for idx := 0; idx < len(value); idx++ {
		switch value[idx] {
		case '#':
			if idx > 0 && isWhitespace(value[idx-1]) {
				return idx
			}
		}
	}
	return -1
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t'
}

func iniDependencyGroup(query iniQuery, key *ini.Key) DependencyGroup {
	location := query.sourceGroup + "." + key.Name()
	group := DependencyGroup{Name: location, Location: location, State: GroupEmpty}
	invalid := func(raw string) {
		group.Diagnostics = append(group.Diagnostics, Diagnostic{Severity: DiagnosticError, Code: "ini-generation-incomplete", Message: fmt.Sprintf("unsupported or malformed declaration at %s: %q", location, raw)})
	}
	if query.anyKey && strings.ContainsAny(key.Name(), ":%$") {
		invalid(key.Name())
	}
	value := key.Value()
	separator := ";"
	if strings.Contains(value, "\n") {
		separator = "\n"
	} else if index := findInlineCommentStart(value); index >= 0 {
		value = value[:index]
	}
	for _, entry := range strings.Split(value, separator) {
		raw := strings.TrimSpace(entry)
		if index := findInlineCommentStart(raw); index >= 0 {
			raw = strings.TrimSpace(raw[:index])
		}
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		if strings.Contains(raw, "%(") || strings.Contains(raw, "${") {
			invalid(raw)
			continue
		}
		dependency, err := parsePythonRequirement(raw)
		if err != nil || strings.ContainsAny(dependency.VersionConstraint, "\"'") {
			invalid(raw)
			continue
		}
		group.Dependencies = append(group.Dependencies, dependency)
	}
	if len(group.Dependencies) > 0 {
		group.State = GroupReady
	}
	return group
}

func duplicateINIKeys(lines []string) map[string]bool {
	seen := map[string]bool{}
	duplicates := map[string]bool{}
	section := "DEFAULT"
	inValue := false
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if len(line) == 0 || inValue && isWhitespace(line[0]) {
			continue
		}
		inValue = false
		if strings.HasPrefix(value, "[") {
			if end := strings.Index(value, "]"); end >= 0 {
				section = strings.ToLower(value[1:end])
			}
			continue
		}
		if end := strings.IndexAny(value, "=:"); end >= 0 {
			key := strings.ToLower(strings.Trim(strings.TrimSpace(value[:end]), "\"'`"))
			location := section + "." + key
			duplicates[location] = seen[location]
			seen[location] = true
			inValue = true
		}
	}
	return duplicates
}
