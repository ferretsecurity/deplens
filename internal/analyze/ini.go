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
	queries []iniQuery
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
	}, []byte(strings.Join(valueLines, "\n")+"\n"))
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse ini file %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0)
	found := false
	for _, query := range p.queries {
		sourceGroup, err := file.GetSection(query.sourceGroup)
		if err != nil {
			continue
		}

		keys := matchingINIKeys(sourceGroup, query)
		if len(keys) == 0 {
			continue
		}

		found = true
		for _, key := range keys {
			dependencies = append(dependencies, extractINIDependencies(key)...)
		}
	}

	if !found {
		return sourceAnalyzerResult{}, nil
	}
	if len(dependencies) == 0 {
		return sourceAnalyzerResult{Analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}, Recognized: true}, nil
	}
	return sourceAnalyzerResult{
		Dependencies: dependencies,
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
