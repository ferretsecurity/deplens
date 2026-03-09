package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	scriptTagRegexp = regexp.MustCompile(`(?is)<script\b[^>]*>`)
	srcAttrRegexp   = regexp.MustCompile(`(?is)\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

type htmlMatcherConfig struct {
	ExternalScripts bool `yaml:"external_scripts"`
}

type htmlExternalScriptsParser struct{}

func newHTMLMatcher(raw htmlMatcherConfig) (manifestParser, error) {
	if !raw.ExternalScripts {
		return nil, fmt.Errorf("html.external_scripts: must be true")
	}
	return htmlExternalScriptsParser{}, nil
}

func (p htmlExternalScriptsParser) Match(path string, content []byte) ([]string, bool, error) {
	matches := scriptTagRegexp.FindAllSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, false, nil
	}

	dependencies := make([]string, 0, len(matches))
	for _, match := range matches {
		tag := string(match[0])
		src := srcAttrRegexp.FindStringSubmatch(tag)
		if len(src) != 3 {
			continue
		}

		value := strings.TrimSpace(src[1])
		if value == "" {
			value = strings.TrimSpace(src[2])
		}
		if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
			dependencies = append(dependencies, value)
		}
	}
	if len(dependencies) == 0 {
		return nil, false, nil
	}
	return dependencies, true, nil
}
