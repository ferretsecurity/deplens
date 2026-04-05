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
	section string
	key     string
	anyKey  bool
}

func newINIQueryParser(raw iniMatcherConfig) (manifestParser, error) {
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
			section: query.Section,
			key:     query.Key,
			anyKey:  query.Key == "*",
		})
	}

	return iniQueryParser{queries: queries}, nil
}

func (p iniQueryParser) Match(path string, content []byte) ([]string, *bool, bool, error) {
	file, err := ini.LoadSources(ini.LoadOptions{
		AllowNestedValues:        true,
		Insensitive:             true,
		SpaceBeforeInlineComment: true,
	}, content)
	if err != nil {
		return nil, nil, false, fmt.Errorf("parse ini file %q: %w", path, err)
	}

	dependencies := make([]string, 0)
	found := false
	for _, query := range p.queries {
		section, err := file.GetSection(query.section)
		if err != nil {
			continue
		}

		keys := matchingINIKeys(section, query)
		if len(keys) == 0 {
			continue
		}

		found = true
		for _, key := range keys {
			dependencies = append(dependencies, extractINIDependencies(key)...)
		}
	}

	if !found {
		return nil, nil, false, nil
	}
	if len(dependencies) == 0 {
		return nil, boolPtr(false), true, nil
	}
	return dependencies, boolPtr(true), true, nil
}

func matchingINIKeys(section *ini.Section, query iniQuery) []*ini.Key {
	if query.anyKey {
		return section.Keys()
	}
	if !section.HasKey(query.key) {
		return nil
	}
	return []*ini.Key{section.Key(query.key)}
}

func extractINIDependencies(key *ini.Key) []string {
	entries := key.NestedValues()
	if len(entries) == 0 {
		return nil
	}

	dependencies := make([]string, 0, len(entries))
	for _, entry := range entries {
		dependency := normalizeINIDependency(entry)
		if dependency == "" {
			continue
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
		case '#', ';':
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
