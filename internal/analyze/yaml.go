package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type yamlMatcherConfig struct {
	Query     string   `yaml:"query"`
	Exists    string   `yaml:"exists"`
	ExistsAny []string `yaml:"exists-any"`
}

type yamlPathSegment struct {
	key    string
	expand bool
}

type yamlQueryParser struct {
	segments []yamlPathSegment
}

type yamlExistsParser struct {
	segments []yamlPathSegment
}

type yamlExistsAnyParser struct {
	queries [][]yamlPathSegment
}

func newYAMLQueryParser(raw yamlMatcherConfig) (manifestParser, error) {
	modeCount := 0
	if raw.Query != "" {
		modeCount++
	}
	if raw.Exists != "" {
		modeCount++
	}
	if len(raw.ExistsAny) > 0 {
		modeCount++
	}
	if modeCount > 1 {
		return nil, fmt.Errorf("yaml.query, yaml.exists, and yaml.exists-any are mutually exclusive")
	}
	if modeCount == 0 {
		return nil, fmt.Errorf("yaml.query, yaml.exists, or yaml.exists-any: required")
	}

	if raw.Query != "" {
		segments, err := parseYAMLPath(raw.Query, "yaml.query")
		if err != nil {
			return nil, err
		}
		return yamlQueryParser{segments: segments}, nil
	}

	if len(raw.ExistsAny) > 0 {
		queries := make([][]yamlPathSegment, 0, len(raw.ExistsAny))
		for idx, rawPath := range raw.ExistsAny {
			segments, err := parseYAMLPath(rawPath, fmt.Sprintf("yaml.exists-any[%d]", idx))
			if err != nil {
				return nil, err
			}
			queries = append(queries, segments)
		}
		return yamlExistsAnyParser{queries: queries}, nil
	}

	segments, err := parseYAMLPath(raw.Exists, "yaml.exists")
	if err != nil {
		return nil, err
	}
	return yamlExistsParser{segments: segments}, nil
}

func parseYAMLPath(raw string, fieldName string) ([]yamlPathSegment, error) {
	parts := strings.Split(raw, ".")
	segments := make([]yamlPathSegment, 0, len(parts))
	for idx, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("%s: invalid empty segment at position %d", fieldName, idx)
		}

		segment := yamlPathSegment{key: part}
		if strings.HasSuffix(part, "[]") {
			segment.expand = true
			segment.key = strings.TrimSuffix(part, "[]")
		}
		if segment.key == "" {
			return nil, fmt.Errorf("%s: invalid segment %q", fieldName, part)
		}
		if strings.Contains(segment.key, "[") || strings.Contains(segment.key, "]") {
			return nil, fmt.Errorf("%s: invalid segment %q", fieldName, part)
		}
		segments = append(segments, segment)
	}
	return segments, nil
}

func (p yamlQueryParser) Match(path string, content []byte) ([]Dependency, *bool, bool, error) {
	current, err := resolveYAMLPath(path, content, p.segments)
	if err != nil {
		return nil, nil, false, err
	}
	if len(current) == 0 {
		return nil, nil, false, nil
	}

	dependencies := make([]string, 0, len(current))
	for _, node := range current {
		value, ok := node.(string)
		if !ok || value == "" {
			continue
		}
		dependencies = append(dependencies, value)
	}
	if len(dependencies) == 0 {
		return nil, nil, false, nil
	}
	return dependenciesFromStrings(dependencies), boolPtr(true), true, nil
}

func (p yamlExistsParser) Match(path string, content []byte) ([]Dependency, *bool, bool, error) {
	current, err := resolveYAMLPath(path, content, p.segments)
	if err != nil {
		return nil, nil, false, err
	}
	if len(current) == 0 {
		return nil, nil, false, nil
	}
	return nil, nil, true, nil
}

func (p yamlExistsAnyParser) Match(path string, content []byte) ([]Dependency, *bool, bool, error) {
	for _, query := range p.queries {
		current, err := resolveYAMLPath(path, content, query)
		if err != nil {
			return nil, nil, false, err
		}
		if hasNonEmptyYAMLValue(current) {
			return nil, boolPtr(true), true, nil
		}
	}
	return nil, boolPtr(false), true, nil
}

func resolveYAMLPath(path string, content []byte, segments []yamlPathSegment) ([]any, error) {
	var root any
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("parse yaml file %q: %w", path, err)
	}

	current := []any{root}
	for _, segment := range segments {
		next := make([]any, 0)
		for _, node := range current {
			mapped, ok := asStringMap(node)
			if !ok {
				continue
			}
			value, ok := mapped[segment.key]
			if !ok {
				continue
			}
			if segment.expand {
				items, ok := value.([]any)
				if !ok {
					continue
				}
				next = append(next, items...)
				continue
			}
			next = append(next, value)
		}
		current = next
		if len(current) == 0 {
			return nil, nil
		}
	}

	return current, nil
}

func asStringMap(value any) (map[string]any, bool) {
	switch mapped := value.(type) {
	case map[string]any:
		return mapped, true
	case map[any]any:
		normalized := make(map[string]any, len(mapped))
		for key, item := range mapped {
			stringKey, ok := key.(string)
			if !ok {
				return nil, false
			}
			normalized[stringKey] = item
		}
		return normalized, true
	default:
		return nil, false
	}
}

func hasNonEmptyYAMLValue(values []any) bool {
	for _, value := range values {
		switch typed := value.(type) {
		case nil:
			continue
		case string:
			if typed != "" {
				return true
			}
		case []any:
			if len(typed) > 0 {
				return true
			}
		case map[string]any:
			if len(typed) > 0 {
				return true
			}
		case map[any]any:
			if len(typed) > 0 {
				return true
			}
		default:
			return true
		}
	}
	return false
}
