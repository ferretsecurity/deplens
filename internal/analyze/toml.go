package analyze

import (
	"fmt"
	"cmp"
	"strings"

	"github.com/BurntSushi/toml"
	"slices"
)

type tomlMatcherConfig struct {
	Queries []string `yaml:"queries"`
}

type pipfileMatcherConfig struct{}

type tomlSegment struct {
	key    string
	expand bool
	wild   bool
}

type tomlQuery struct {
	segments   []tomlSegment
	skipPython bool
}

type tomlQueryParser struct {
	queries []tomlQuery
}

type pipfileParser struct{}

var pipfileIgnoredSections = map[string]struct{}{
	"source":   {},
	"requires": {},
	"scripts":  {},
	"pipenv":   {},
}

var pipfileSectionPriority = map[string]int{
	"packages":     0,
	"dev-packages": 1,
}

func newTOMLQueryParser(raw tomlMatcherConfig) (manifestParser, error) {
	if len(raw.Queries) == 0 {
		return nil, fmt.Errorf("toml.queries: must contain at least one entry")
	}

	queries := make([]tomlQuery, 0, len(raw.Queries))
	for queryIdx, rawQuery := range raw.Queries {
		query, err := compileTOMLQuery(rawQuery)
		if err != nil {
			return nil, fmt.Errorf("toml.queries[%d]: %w", queryIdx, err)
		}
		queries = append(queries, query)
	}

	return tomlQueryParser{queries: queries}, nil
}

func newPipfileParser(raw pipfileMatcherConfig) (manifestParser, error) {
	return pipfileParser{}, nil
}

func (p tomlQueryParser) Match(path string, content []byte) ([]string, bool, error) {
	var root map[string]any
	if err := toml.Unmarshal(content, &root); err != nil {
		return nil, false, fmt.Errorf("parse toml file %q: %w", path, err)
	}

	dependencies := make([]string, 0)
	for _, query := range p.queries {
		nodes := evalTOMLQuery([]any{root}, query)
		dependencies = append(dependencies, extractTOMLDependencies(nodes, query)...)
	}
	if len(dependencies) == 0 {
		return nil, false, nil
	}
	return dependencies, true, nil
}

func (p pipfileParser) Match(path string, content []byte) ([]string, bool, error) {
	var root map[string]any
	if err := toml.Unmarshal(content, &root); err != nil {
		return nil, false, fmt.Errorf("parse toml file %q: %w", path, err)
	}

	keys := make([]string, 0, len(root))
	for key := range root {
		if _, ignored := pipfileIgnoredSections[key]; ignored {
			continue
		}
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b string) int {
		return cmp.Or(
			cmp.Compare(pipfileSectionPriorityValue(a), pipfileSectionPriorityValue(b)),
			cmp.Compare(a, b),
		)
	})

	dependencies := make([]string, 0)
	for _, key := range keys {
		table, ok := root[key].(map[string]any)
		if !ok {
			continue
		}
		dependencies = append(dependencies, serializeTOMLDependencyTable(table, false)...)
	}
	if len(dependencies) == 0 {
		return nil, false, nil
	}
	return dependencies, true, nil
}

func pipfileSectionPriorityValue(key string) int {
	priority, ok := pipfileSectionPriority[key]
	if ok {
		return priority
	}
	return len(pipfileSectionPriority) + 1
}

func compileTOMLQuery(raw string) (tomlQuery, error) {
	if raw == "" {
		return tomlQuery{}, fmt.Errorf("required")
	}

	parts := strings.Split(raw, ".")
	segments := make([]tomlSegment, 0, len(parts))
	for idx, part := range parts {
		if part == "" {
			return tomlQuery{}, fmt.Errorf("invalid empty segment at position %d", idx)
		}

		segment := tomlSegment{key: part}
		if strings.HasSuffix(part, "[]") {
			segment.expand = true
			segment.key = strings.TrimSuffix(part, "[]")
		}

		if segment.key == "*" {
			segment.wild = true
			segment.key = ""
		}

		if segment.key == "" && !segment.wild {
			return tomlQuery{}, fmt.Errorf("invalid segment %q", part)
		}
		if !segment.wild && (strings.Contains(segment.key, "[") || strings.Contains(segment.key, "]") || strings.Contains(segment.key, "*")) {
			return tomlQuery{}, fmt.Errorf("invalid segment %q", part)
		}

		segments = append(segments, segment)
	}

	return tomlQuery{
		segments:   segments,
		skipPython: isPoetryDependencyQuery(segments),
	}, nil
}

func evalTOMLQuery(current []any, query tomlQuery) []any {
	for _, segment := range query.segments {
		next := make([]any, 0)
		for _, node := range current {
			mapped, ok := node.(map[string]any)
			if !ok {
				continue
			}

			switch {
			case segment.wild:
				keys := make([]string, 0, len(mapped))
				for key := range mapped {
					keys = append(keys, key)
				}
				slices.Sort(keys)
				for _, key := range keys {
					value := mapped[key]
					if segment.expand {
						next = appendTOMLArrayValues(next, value)
						continue
					}
					next = append(next, value)
				}
			case segment.expand:
				value, ok := mapped[segment.key]
				if !ok {
					continue
				}
				next = appendTOMLArrayValues(next, value)
			default:
				value, ok := mapped[segment.key]
				if !ok {
					continue
				}
				next = append(next, value)
			}
		}
		current = next
		if len(current) == 0 {
			return nil
		}
	}

	return current
}

func appendTOMLArrayValues(dst []any, value any) []any {
	switch typed := value.(type) {
	case []any:
		return append(dst, typed...)
	case []map[string]any:
		for _, item := range typed {
			dst = append(dst, item)
		}
		return dst
	default:
		return dst
	}
}

func isPoetryDependencyQuery(segments []tomlSegment) bool {
	if len(segments) == 3 &&
		segments[0].key == "tool" &&
		segments[1].key == "poetry" &&
		segments[2].key == "dependencies" {
		return true
	}

	if len(segments) == 5 &&
		segments[0].key == "tool" &&
		segments[1].key == "poetry" &&
		segments[2].key == "group" &&
		segments[4].key == "dependencies" {
		return true
	}

	return false
}

func extractTOMLDependencies(nodes []any, query tomlQuery) []string {
	dependencies := make([]string, 0, len(nodes))
	allowDependencyTables := allowsTOMLDependencyTables(query.segments)
	for _, node := range nodes {
		switch value := node.(type) {
		case string:
			if value != "" {
				dependencies = append(dependencies, value)
			}
		case map[string]any:
			if !allowDependencyTables {
				continue
			}
			dependencies = append(dependencies, serializeTOMLDependencyTable(value, query.skipPython)...)
		}
	}
	return dependencies
}

func serializeTOMLDependencyTable(value map[string]any, skipPython bool) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		if skipPython && key == "python" {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)

	dependencies := make([]string, 0, len(keys))
	for _, key := range keys {
		serialized, ok := serializeTOMLValue(value[key])
		if !ok {
			continue
		}
		dependencies = append(dependencies, fmt.Sprintf("%s = %s", key, serialized))
	}
	return dependencies
}

func allowsTOMLDependencyTables(segments []tomlSegment) bool {
	if len(segments) == 0 {
		return false
	}

	return !segments[len(segments)-1].expand
}

func serializeTOMLValue(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return fmt.Sprintf("%q", typed), true
	case bool:
		if typed {
			return "true", true
		}
		return "false", true
	case int:
		return fmt.Sprintf("%d", typed), true
	case int64:
		return fmt.Sprintf("%d", typed), true
	case float64:
		return fmt.Sprintf("%v", typed), true
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			serialized, ok := serializeTOMLValue(item)
			if !ok {
				return "", false
			}
			parts = append(parts, serialized)
		}
		return "[" + strings.Join(parts, ", ") + "]", true
	case []map[string]any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			serialized, ok := serializeTOMLValue(item)
			if !ok {
				return "", false
			}
			parts = append(parts, serialized)
		}
		return "[" + strings.Join(parts, ", ") + "]", true
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		slices.Sort(keys)

		var b strings.Builder
		b.WriteString("{ ")
		for idx, key := range keys {
			if idx > 0 {
				b.WriteString(", ")
			}
			serialized, ok := serializeTOMLValue(typed[key])
			if !ok {
				return "", false
			}
			b.WriteString(key)
			b.WriteString(" = ")
			b.WriteString(serialized)
		}
		b.WriteString(" }")
		return b.String(), true
	default:
		return "", false
	}
}
