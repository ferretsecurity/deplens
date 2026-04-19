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

func (m jsonMatcher) Match(path string, content []byte) (manifestParserResult, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return manifestParserResult{}, fmt.Errorf("parse json file %q: %w", path, err)
	}

	for _, key := range m.keys {
		rawValue, ok := root[key]
		if !ok {
			continue
		}
		var objectValues map[string]json.RawMessage
		if err := json.Unmarshal(rawValue, &objectValues); err == nil {
			if len(objectValues) > 0 {
				return manifestParserResult{HasDependencies: boolPtr(true), Matched: true}, nil
			}
			continue
		}

		var arrayValues []json.RawMessage
		if err := json.Unmarshal(rawValue, &arrayValues); err == nil {
			if len(arrayValues) > 0 {
				return manifestParserResult{HasDependencies: boolPtr(true), Matched: true}, nil
			}
			continue
		}

		var stringValue string
		if err := json.Unmarshal(rawValue, &stringValue); err == nil {
			if stringValue != "" {
				return manifestParserResult{HasDependencies: boolPtr(true), Matched: true}, nil
			}
			continue
		}

		if string(rawValue) == "null" {
			continue
		}

		var boolValue bool
		if err := json.Unmarshal(rawValue, &boolValue); err == nil {
			if boolValue {
				return manifestParserResult{HasDependencies: boolPtr(true), Matched: true}, nil
			}
			continue
		}

		var numberValue float64
		if err := json.Unmarshal(rawValue, &numberValue); err == nil {
			return manifestParserResult{HasDependencies: boolPtr(true), Matched: true}, nil
		}
	}

	return manifestParserResult{HasDependencies: boolPtr(false), Matched: true}, nil
}
