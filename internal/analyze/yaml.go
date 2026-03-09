package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type yamlMatcherConfig struct {
	Query string `yaml:"query"`
}

type yamlPathSegment struct {
	key    string
	expand bool
}

type yamlQueryParser struct {
	segments []yamlPathSegment
}

func newYAMLQueryParser(raw yamlMatcherConfig) (manifestParser, error) {
	if raw.Query == "" {
		return nil, fmt.Errorf("yaml.query: required")
	}

	parts := strings.Split(raw.Query, ".")
	segments := make([]yamlPathSegment, 0, len(parts))
	for idx, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("yaml.query: invalid empty segment at position %d", idx)
		}

		segment := yamlPathSegment{key: part}
		if strings.HasSuffix(part, "[]") {
			segment.expand = true
			segment.key = strings.TrimSuffix(part, "[]")
		}
		if segment.key == "" {
			return nil, fmt.Errorf("yaml.query: invalid segment %q", part)
		}
		if strings.Contains(segment.key, "[") || strings.Contains(segment.key, "]") {
			return nil, fmt.Errorf("yaml.query: invalid segment %q", part)
		}
		segments = append(segments, segment)
	}
	return yamlQueryParser{segments: segments}, nil
}

func (p yamlQueryParser) Match(path string, content []byte) ([]string, bool, error) {
	var root any
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, false, fmt.Errorf("parse yaml file %q: %w", path, err)
	}

	current := []any{root}
	for _, segment := range p.segments {
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
			return nil, false, nil
		}
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
		return nil, false, nil
	}
	return dependencies, true, nil
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
