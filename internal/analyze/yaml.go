package analyze

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/itchyny/gojq"
	"gopkg.in/yaml.v3"
)

type yamlMatcherConfig struct {
	Query     string            `yaml:"query"`
	Exists    string            `yaml:"exists"`
	ExistsAny []string          `yaml:"exists-any"`
	Groups    *yamlGroupsConfig `yaml:"groups"`
}

type yamlGroupsConfig struct {
	Query             string `yaml:"query"`
	NameQuery         string `yaml:"name-query"`
	DependenciesQuery string `yaml:"dependencies-query"`
}

type yamlGroupsParser struct{ pathCode, nameCode, dependenciesCode *gojq.Code }

var jqIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

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

func newYAMLQueryParser(raw yamlMatcherConfig) (sourceAnalyzer, error) {
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
	if raw.Groups != nil {
		modeCount++
	}
	if modeCount > 1 {
		return nil, fmt.Errorf("yaml.query, yaml.exists, yaml.exists-any, and yaml.groups are mutually exclusive")
	}
	if modeCount == 0 {
		return nil, fmt.Errorf("yaml.query, yaml.exists, yaml.exists-any, or yaml.groups: required")
	}
	if raw.Groups != nil {
		return newYAMLGroupsParser(*raw.Groups)
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

func newYAMLGroupsParser(raw yamlGroupsConfig) (sourceAnalyzer, error) {
	if raw.Query == "" || raw.NameQuery == "" || raw.DependenciesQuery == "" {
		return nil, fmt.Errorf("yaml.groups.query, name-query, and dependencies-query are required")
	}
	compile := func(label, query string, variables ...string) (*gojq.Code, error) {
		parsed, err := gojq.Parse(query)
		if err != nil {
			return nil, fmt.Errorf("yaml.groups.%s: %w", label, err)
		}
		code, err := gojq.Compile(parsed, gojq.WithVariables(variables))
		if err != nil {
			return nil, fmt.Errorf("yaml.groups.%s: %w", label, err)
		}
		return code, nil
	}
	pathCode, err := compile("query", "path("+raw.Query+")")
	if err != nil {
		return nil, err
	}
	nameCode, err := compile("name-query", raw.NameQuery, "$key")
	if err != nil {
		return nil, err
	}
	depsCode, err := compile("dependencies-query", raw.DependenciesQuery, "$key")
	if err != nil {
		return nil, err
	}
	return yamlGroupsParser{pathCode, nameCode, depsCode}, nil
}

func (p yamlGroupsParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root any
	if err := yaml.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse yaml file %q: %w", path, err)
	}
	paths, err := jqValues(p.pathCode, root)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("select yaml groups in %q: %w", path, err)
	}
	if len(paths) == 0 {
		return sourceAnalyzerResult{}, nil
	}
	groups := make([]DependencyGroup, 0, len(paths))
	deps := make([]DependencyReference, 0)
	seenPaths := map[string]struct{}{}
	for _, rawPath := range paths {
		segments, ok := rawPath.([]any)
		if !ok {
			return sourceAnalyzerResult{}, fmt.Errorf("select yaml groups in %q: query did not select source nodes", path)
		}
		node, ok := yamlNodeAt(root, segments)
		if !ok {
			return sourceAnalyzerResult{}, fmt.Errorf("select yaml groups in %q: query transformed a group", path)
		}
		location := jqPath(segments)
		if _, exists := seenPaths[location]; exists {
			return sourceAnalyzerResult{}, fmt.Errorf("select yaml groups in %q: query selected %s more than once", path, location)
		}
		seenPaths[location] = struct{}{}
		var key any
		if len(segments) > 0 {
			if mappingKey, ok := segments[len(segments)-1].(string); ok {
				key = mappingKey
			}
		}
		names, err := jqValuesWithVariables(p.nameCode, node, key)
		if err != nil {
			return sourceAnalyzerResult{}, fmt.Errorf("group %s in %q name: %w", location, path, err)
		}
		if len(names) != 1 {
			return sourceAnalyzerResult{}, fmt.Errorf("group %s in %q: name-query must produce one value", location, path)
		}
		var name string
		if names[0] != nil {
			var ok bool
			name, ok = names[0].(string)
			if !ok {
				return sourceAnalyzerResult{}, fmt.Errorf("group %s in %q: name-query must produce a string or null, got %T", location, path, names[0])
			}
		}
		values, err := jqValuesWithVariables(p.dependenciesCode, node, key)
		if err != nil {
			return sourceAnalyzerResult{}, fmt.Errorf("group %q at %s in %q dependencies: %w", name, location, path, err)
		}
		group := DependencyGroup{Name: name, Location: location}
		if len(values) == 0 || (len(values) == 1 && values[0] == nil) {
			group.State = GroupMissing
			groups = append(groups, group)
			continue
		}
		if len(values) != 1 {
			return sourceAnalyzerResult{}, fmt.Errorf("group %q at %s in %q: dependencies-query must produce one list", name, location, path)
		}
		items, ok := values[0].([]any)
		if !ok {
			return sourceAnalyzerResult{}, fmt.Errorf("group %q at %s in %q: dependencies must be a list", name, location, path)
		}
		if len(items) == 0 {
			group.State = GroupEmpty
			groups = append(groups, group)
			continue
		}
		group.State = GroupReady
		for i, item := range items {
			spec, ok := item.(string)
			if !ok {
				return sourceAnalyzerResult{}, fmt.Errorf("group %q at %s in %q: dependency %d must be a string", name, location, path, i)
			}
			dep, err := parsePythonRequirement(spec)
			if err != nil {
				return sourceAnalyzerResult{}, fmt.Errorf("group %q at %s in %q: dependency %q: %w", name, location, path, spec, err)
			}
			dep.SourceGroup = name
			group.Dependencies = append(group.Dependencies, dep)
			deps = append(deps, dep)
		}
		groups = append(groups, group)
	}
	analysis := SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}
	if len(deps) > 0 {
		analysis.Presence = PresencePresent
	}
	return sourceAnalyzerResult{Recognized: true, Analysis: analysis, Dependencies: deps, Groups: groups}, nil
}

func jqValues(code *gojq.Code, input any) ([]any, error) {
	return jqValuesWithVariables(code, input)
}

func jqValuesWithVariables(code *gojq.Code, input any, variables ...any) ([]any, error) {
	var out []any
	iter := code.Run(input, variables...)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func yamlNodeAt(root any, path []any) (any, bool) {
	current := root
	for _, segment := range path {
		switch s := segment.(type) {
		case string:
			m, ok := asStringMap(current)
			if !ok {
				return nil, false
			}
			current, ok = m[s]
			if !ok {
				return nil, false
			}
		case int:
			a, ok := current.([]any)
			if !ok || s < 0 || s >= len(a) {
				return nil, false
			}
			current = a[s]
		default:
			return nil, false
		}
	}
	return current, true
}

func jqPath(path []any) string {
	var b strings.Builder
	b.WriteByte('.')
	for _, s := range path {
		switch v := s.(type) {
		case string:
			if jqIdentifier.MatchString(v) {
				if b.Len() > 1 {
					b.WriteByte('.')
				}
				b.WriteString(v)
			} else {
				b.WriteByte('[')
				b.WriteString(strconv.Quote(v))
				b.WriteByte(']')
			}
		case int:
			fmt.Fprintf(&b, "[%d]", v)
		}
	}
	return b.String()
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

func (p yamlQueryParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	current, err := resolveYAMLPath(path, content, p.segments)
	if err != nil {
		return sourceAnalyzerResult{}, err
	}
	if len(current) == 0 {
		return sourceAnalyzerResult{}, nil
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
		return sourceAnalyzerResult{}, nil
	}
	return sourceAnalyzerResult{
		Dependencies: dependenciesFromStrings(dependencies),
		Analysis:     SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		Recognized:   true,
	}, nil
}

func (p yamlExistsParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	current, err := resolveYAMLPath(path, content, p.segments)
	if err != nil {
		return sourceAnalyzerResult{}, err
	}
	if len(current) == 0 {
		return sourceAnalyzerResult{}, nil
	}
	return sourceAnalyzerResult{Recognized: true, Analysis: identifiedAnalysis()}, nil
}

func (p yamlExistsAnyParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	for _, query := range p.queries {
		current, err := resolveYAMLPath(path, content, query)
		if err != nil {
			return sourceAnalyzerResult{}, err
		}
		if hasNonEmptyYAMLValue(current) {
			return sourceAnalyzerResult{Analysis: presenceAnalysis(true), Recognized: true}, nil
		}
	}
	return sourceAnalyzerResult{Analysis: presenceAnalysis(false), Recognized: true}, nil
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
