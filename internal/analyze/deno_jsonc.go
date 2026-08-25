package analyze

import "fmt"

type denoJSONCParserConfig struct{}

type denoJSONCParser struct{}

func newDenoJSONCParser(denoJSONCParserConfig) (sourceAnalyzer, error) {
	return denoJSONCParser{}, nil
}

func (denoJSONCParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	jsonContent, err := normalizeJSONC(content)
	if err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Deno JSONC configuration %q: %w", path, err)
	}
	return denoJSONParser{}.Analyze(path, jsonContent)
}

// normalizeJSONC removes comments and trailing commas without interpreting
// comment-like text inside JSON strings.
func normalizeJSONC(content []byte) ([]byte, error) {
	withoutComments := make([]byte, 0, len(content))
	inString := false
	escaped := false

	for index := 0; index < len(content); index++ {
		current := content[index]
		if inString {
			withoutComments = append(withoutComments, current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}

		switch current {
		case '"':
			inString = true
			withoutComments = append(withoutComments, current)
		case '/':
			if index+1 == len(content) {
				withoutComments = append(withoutComments, current)
				continue
			}

			switch content[index+1] {
			case '/':
				index += 2
				for index < len(content) && content[index] != '\n' && content[index] != '\r' {
					index++
				}
				if index < len(content) {
					withoutComments = append(withoutComments, content[index])
				}
			case '*':
				index += 2
				terminated := false
				for index < len(content) {
					if content[index] == '*' && index+1 < len(content) && content[index+1] == '/' {
						index++
						terminated = true
						break
					}
					if content[index] == '\n' || content[index] == '\r' {
						withoutComments = append(withoutComments, content[index])
					}
					index++
				}
				if !terminated {
					return nil, fmt.Errorf("unterminated block comment")
				}
			default:
				withoutComments = append(withoutComments, current)
			}
		default:
			withoutComments = append(withoutComments, current)
		}
	}

	if inString {
		return nil, fmt.Errorf("unterminated string")
	}

	result := make([]byte, 0, len(withoutComments))
	inString = false
	escaped = false
	for index := 0; index < len(withoutComments); index++ {
		current := withoutComments[index]
		if inString {
			result = append(result, current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}

		if current == '"' {
			inString = true
			result = append(result, current)
			continue
		}
		if current == ',' {
			next := index + 1
			for next < len(withoutComments) && isJSONWhitespace(withoutComments[next]) {
				next++
			}
			if next < len(withoutComments) && (withoutComments[next] == '}' || withoutComments[next] == ']') {
				continue
			}
		}
		result = append(result, current)
	}
	return result, nil
}

func isJSONWhitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r'
}
