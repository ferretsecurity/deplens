package analyze

import "fmt"

type iniMatcherConfig struct {
	Queries []iniQueryConfig `yaml:"queries"`
}

type iniQueryConfig struct {
	Section string `yaml:"section"`
	Key     string `yaml:"key"`
}

type iniQueryParser struct {
	queries []iniQueryConfig
}

func newINIQueryParser(raw iniMatcherConfig) (manifestParser, error) {
	if len(raw.Queries) == 0 {
		return nil, fmt.Errorf("ini.queries: must contain at least one entry")
	}

	queries := make([]iniQueryConfig, 0, len(raw.Queries))
	for queryIdx, query := range raw.Queries {
		if query.Section == "" {
			return nil, fmt.Errorf("ini.queries[%d].section: required", queryIdx)
		}
		if query.Key == "" {
			return nil, fmt.Errorf("ini.queries[%d].key: required", queryIdx)
		}
		queries = append(queries, query)
	}

	return iniQueryParser{queries: queries}, nil
}

func (p iniQueryParser) Match(path string, content []byte) ([]string, bool, error) {
	return nil, false, nil
}
