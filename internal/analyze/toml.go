package analyze

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"slices"
)

type tomlMatcherConfig struct {
	Queries        []string `yaml:"queries"`
	TableQueries   []string `yaml:"table-queries"`
	ExistsAny      []string `yaml:"exists-any"`
	TableExistsAny []string `yaml:"table-exists-any"`
	ExcludeKeys    []string `yaml:"exclude-keys"`
}

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
	queries        []tomlQuery
	tableQueries   []tomlQuery
	existsAny      []tomlQuery
	tableExistsAny []tomlQuery
	excludeKeys    map[string]struct{}
}

type tomlMatchedTable struct {
	key     string
	section string
	value   map[string]any
}

type tomlMatchedValue struct {
	section string
	value   any
}

func newTOMLQueryParser(raw tomlMatcherConfig) (manifestParser, error) {
	if len(raw.Queries) == 0 && len(raw.TableQueries) == 0 && len(raw.ExistsAny) == 0 && len(raw.TableExistsAny) == 0 {
		return nil, fmt.Errorf("toml: at least one of queries, table-queries, exists-any, or table-exists-any must contain an entry")
	}

	queries := make([]tomlQuery, 0, len(raw.Queries))
	for queryIdx, rawQuery := range raw.Queries {
		query, err := compileTOMLQuery(rawQuery)
		if err != nil {
			return nil, fmt.Errorf("toml.queries[%d]: %w", queryIdx, err)
		}
		queries = append(queries, query)
	}

	tableQueries := make([]tomlQuery, 0, len(raw.TableQueries))
	for queryIdx, rawQuery := range raw.TableQueries {
		query, err := compileTOMLQuery(rawQuery)
		if err != nil {
			return nil, fmt.Errorf("toml.table-queries[%d]: %w", queryIdx, err)
		}
		tableQueries = append(tableQueries, query)
	}

	existsAny := make([]tomlQuery, 0, len(raw.ExistsAny))
	for queryIdx, rawQuery := range raw.ExistsAny {
		query, err := compileTOMLQuery(rawQuery)
		if err != nil {
			return nil, fmt.Errorf("toml.exists-any[%d]: %w", queryIdx, err)
		}
		existsAny = append(existsAny, query)
	}

	tableExistsAny := make([]tomlQuery, 0, len(raw.TableExistsAny))
	for queryIdx, rawQuery := range raw.TableExistsAny {
		query, err := compileTOMLQuery(rawQuery)
		if err != nil {
			return nil, fmt.Errorf("toml.table-exists-any[%d]: %w", queryIdx, err)
		}
		tableExistsAny = append(tableExistsAny, query)
	}

	excludeKeys := make(map[string]struct{}, len(raw.ExcludeKeys))
	for idx, key := range raw.ExcludeKeys {
		if key == "" {
			return nil, fmt.Errorf("toml.exclude-keys[%d]: required", idx)
		}
		excludeKeys[key] = struct{}{}
	}

	return tomlQueryParser{
		queries:        queries,
		tableQueries:   tableQueries,
		existsAny:      existsAny,
		tableExistsAny: tableExistsAny,
		excludeKeys:    excludeKeys,
	}, nil
}

func (p tomlQueryParser) Match(path string, content []byte) (manifestParserResult, error) {
	var root map[string]any
	if err := toml.Unmarshal(content, &root); err != nil {
		return manifestParserResult{}, fmt.Errorf("parse toml file %q: %w", path, err)
	}

	dependencies := make([]Dependency, 0)
	for _, query := range p.queries {
		nodes := evalTOMLQuery([]tomlMatchedValue{{value: root}}, query)
		dependencies = append(dependencies, extractTOMLDependencies(nodes, query)...)
	}
	for _, query := range p.tableQueries {
		tables := evalTOMLTableQuery([]tomlMatchedTable{{value: root}}, query)
		dependencies = append(dependencies, extractTOMLTableDependencies(tables, p.excludeKeys)...)
	}
	if len(dependencies) == 0 {
		if len(p.existsAny) > 0 {
			for _, query := range p.existsAny {
				nodes := evalTOMLQuery([]tomlMatchedValue{{value: root}}, query)
				if hasNonEmptyTOMLValue(nodes) {
					return manifestParserResult{HasDependencies: boolPtr(true), Matched: true}, nil
				}
			}
			return manifestParserResult{HasDependencies: boolPtr(false), Matched: true}, nil
		}
		if len(p.tableExistsAny) > 0 {
			for _, query := range p.tableExistsAny {
				tables := evalTOMLTableQuery([]tomlMatchedTable{{value: root}}, query)
				if hasNonEmptyTOMLTable(tables) {
					return manifestParserResult{HasDependencies: boolPtr(true), Matched: true}, nil
				}
			}
			return manifestParserResult{HasDependencies: boolPtr(false), Matched: true}, nil
		}
		return manifestParserResult{}, nil
	}
	return manifestParserResult{
		Dependencies:    dependencies,
		HasDependencies: boolPtr(true),
		Matched:         true,
	}, nil
}

func hasNonEmptyTOMLTable(nodes []tomlMatchedTable) bool {
	for _, node := range nodes {
		if len(node.value) > 0 {
			return true
		}
	}
	return false
}

func hasNonEmptyTOMLValue(nodes []tomlMatchedValue) bool {
	for _, node := range nodes {
		switch value := node.value.(type) {
		case nil:
			continue
		case string:
			if value != "" {
				return true
			}
		case []any:
			if len(value) > 0 {
				return true
			}
		case []map[string]any:
			if len(value) > 0 {
				return true
			}
		case map[string]any:
			if len(value) > 0 {
				return true
			}
		default:
			return true
		}
	}
	return false
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

func evalTOMLQuery(current []tomlMatchedValue, query tomlQuery) []tomlMatchedValue {
	for _, segment := range query.segments {
		next := make([]tomlMatchedValue, 0)
		for _, node := range current {
			mapped, ok := node.value.(map[string]any)
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
					section := appendTOMLSection(node.section, key)
					if segment.expand {
						next = appendTOMLArrayValues(next, value, section)
						continue
					}
					next = append(next, tomlMatchedValue{section: section, value: value})
				}
			case segment.expand:
				value, ok := mapped[segment.key]
				if !ok {
					continue
				}
				next = appendTOMLArrayValues(next, value, appendTOMLSection(node.section, segment.key))
			default:
				value, ok := mapped[segment.key]
				if !ok {
					continue
				}
				next = append(next, tomlMatchedValue{
					section: appendTOMLSection(node.section, segment.key),
					value:   value,
				})
			}
		}
		current = next
		if len(current) == 0 {
			return nil
		}
	}

	return current
}

func evalTOMLTableQuery(current []tomlMatchedTable, query tomlQuery) []tomlMatchedTable {
	for _, segment := range query.segments {
		next := make([]tomlMatchedTable, 0)
		for _, node := range current {
			mapped := node.value
			if mapped == nil {
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
					value, ok := mapped[key].(map[string]any)
					if !ok {
						continue
					}
					next = append(next, tomlMatchedTable{
						key:     key,
						section: appendTOMLSection(node.section, key),
						value:   value,
					})
				}
			case segment.expand:
				// Table queries only support matching TOML tables, not arrays.
				continue
			default:
				value, ok := mapped[segment.key].(map[string]any)
				if !ok {
					continue
				}
				next = append(next, tomlMatchedTable{
					key:     segment.key,
					section: appendTOMLSection(node.section, segment.key),
					value:   value,
				})
			}
		}
		current = next
		if len(current) == 0 {
			return nil
		}
	}

	return current
}

func appendTOMLArrayValues(dst []tomlMatchedValue, value any, section string) []tomlMatchedValue {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			dst = append(dst, tomlMatchedValue{section: section, value: item})
		}
		return dst
	case []map[string]any:
		for _, item := range typed {
			dst = append(dst, tomlMatchedValue{section: section, value: item})
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

func extractTOMLDependencies(nodes []tomlMatchedValue, query tomlQuery) []Dependency {
	dependencies := make([]Dependency, 0, len(nodes))
	allowDependencyTables := allowsTOMLDependencyTables(query.segments)
	for _, node := range nodes {
		switch value := node.value.(type) {
		case string:
			if value != "" {
				dependencies = append(dependencies, Dependency{Name: value, Section: node.section})
			}
		case map[string]any:
			if !allowDependencyTables {
				continue
			}
			dependencies = append(dependencies, serializeTOMLDependencyTable(value, query.skipPython, node.section)...)
		}
	}
	return dependencies
}

func extractTOMLTableDependencies(nodes []tomlMatchedTable, excludeKeys map[string]struct{}) []Dependency {
	filtered := make([]tomlMatchedTable, 0, len(nodes))
	for _, node := range nodes {
		if _, excluded := excludeKeys[node.key]; excluded {
			continue
		}
		filtered = append(filtered, node)
	}
	slices.SortFunc(filtered, func(a, b tomlMatchedTable) int {
		return cmp.Or(
			cmp.Compare(tomlDependencyKeyPriority(a.key), tomlDependencyKeyPriority(b.key)),
			cmp.Compare(a.key, b.key),
		)
	})

	dependencies := make([]Dependency, 0, len(filtered))
	for _, node := range filtered {
		dependencies = append(dependencies, serializeTOMLDependencyTable(node.value, false, node.section)...)
	}
	return dependencies
}

func serializeTOMLDependencyTable(value map[string]any, skipPython bool, section string) []Dependency {
	keys := make([]string, 0, len(value))
	for key := range value {
		if skipPython && key == "python" {
			continue
		}
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b string) int {
		return cmp.Or(
			cmp.Compare(tomlDependencyKeyPriority(a), tomlDependencyKeyPriority(b)),
			cmp.Compare(a, b),
		)
	})

	dependencies := make([]Dependency, 0, len(keys))
	for _, key := range keys {
		serialized, ok := serializeTOMLValue(value[key])
		if !ok {
			continue
		}
		dependencies = append(dependencies, Dependency{
			Name:    fmt.Sprintf("%s = %s", key, serialized),
			Section: section,
		})
	}
	return dependencies
}

func appendTOMLSection(base string, segment string) string {
	if base == "" {
		return segment
	}
	return base + "." + segment
}

func tomlDependencyKeyPriority(key string) int {
	switch key {
	case "packages":
		return 0
	case "dev-packages":
		return 1
	default:
		return 2
	}
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
