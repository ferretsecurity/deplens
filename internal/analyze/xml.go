package analyze

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type xmlMatcherConfig struct {
	ExistsAny []string `yaml:"exists-any"`
}

type xmlExistsAnyParser struct {
	queries [][]string
}

func newXMLMatcher(raw xmlMatcherConfig) (manifestParser, error) {
	if len(raw.ExistsAny) == 0 {
		return nil, fmt.Errorf("xml.exists-any: must contain at least one entry")
	}

	queries := make([][]string, 0, len(raw.ExistsAny))
	for idx, rawPath := range raw.ExistsAny {
		segments, err := parseXMLPath(rawPath, fmt.Sprintf("xml.exists-any[%d]", idx))
		if err != nil {
			return nil, err
		}
		queries = append(queries, segments)
	}

	return xmlExistsAnyParser{queries: queries}, nil
}

func parseXMLPath(raw string, fieldName string) ([]string, error) {
	parts := strings.Split(raw, ".")
	segments := make([]string, 0, len(parts))
	for idx, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("%s: invalid empty segment at position %d", fieldName, idx)
		}
		if strings.ContainsAny(part, "[]*") {
			return nil, fmt.Errorf("%s: invalid segment %q", fieldName, part)
		}
		segments = append(segments, part)
	}
	return segments, nil
}

func (p xmlExistsAnyParser) Match(path string, content []byte) ([]Dependency, *bool, bool, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	stack := make([]string, 0)

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return nil, boolPtr(false), true, nil
			}
			return nil, nil, false, fmt.Errorf("parse xml file %q: %w", path, err)
		}

		switch typed := token.(type) {
		case xml.StartElement:
			stack = append(stack, typed.Name.Local)
			for _, query := range p.queries {
				if len(stack) != len(query) {
					continue
				}
				if xmlPathMatches(stack, query) {
					return nil, boolPtr(true), true, nil
				}
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
}

func xmlPathMatches(stack []string, query []string) bool {
	for idx := range query {
		if stack[idx] != query[idx] {
			return false
		}
	}
	return true
}
