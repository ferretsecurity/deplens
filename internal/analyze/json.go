package analyze

import (
	"encoding/json"
	"fmt"
)

type jsonMatcherConfig struct {
	ExistsAny []string `yaml:"exists-any"`
}

type jsonMatcher struct {
	keys []string
}

func newJSONMatcher(raw jsonMatcherConfig) (manifestParser, error) {
	if len(raw.ExistsAny) == 0 {
		return nil, fmt.Errorf("json.exists-any: must contain at least one entry")
	}

	keys := make([]string, 0, len(raw.ExistsAny))
	seen := make(map[string]struct{}, len(raw.ExistsAny))
	for idx, key := range raw.ExistsAny {
		if key == "" {
			return nil, fmt.Errorf("json.exists-any[%d]: required", idx)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	return jsonMatcher{keys: keys}, nil
}

func (m jsonMatcher) Match(path string, content []byte) ([]Dependency, *bool, bool, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return nil, nil, false, fmt.Errorf("parse json file %q: %w", path, err)
	}

	for _, key := range m.keys {
		rawValue, ok := root[key]
		if !ok {
			continue
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(rawValue, &values); err != nil {
			continue
		}
		if len(values) > 0 {
			return nil, boolPtr(true), true, nil
		}
	}

	return nil, boolPtr(false), true, nil
}
