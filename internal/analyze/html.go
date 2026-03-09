package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	scriptBlockRegexp  = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
	srcAttrRegexp      = regexp.MustCompile(`(?is)\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	moduleImportRegexp = regexp.MustCompile(`(?is)\bimport\s+(?:[^"'()]+?\s+from\s+)?(?:"(https?://[^"]+)"|'(https?://[^']+)')`)
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
	matches := scriptBlockRegexp.FindAllSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, false, nil
	}

	dependencies := make([]string, 0, len(matches))
	for _, match := range matches {
		tagAttrs := string(match[1])
		body := string(match[2])

		src := srcAttrRegexp.FindStringSubmatch(tagAttrs)
		if len(src) == 3 {
			value := strings.TrimSpace(src[1])
			if value == "" {
				value = strings.TrimSpace(src[2])
			}
			if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
				dependencies = append(dependencies, value)
			}
			continue
		}

		imports := moduleImportRegexp.FindAllStringSubmatch(body, -1)
		for _, importMatch := range imports {
			value := strings.TrimSpace(importMatch[1])
			if value == "" {
				value = strings.TrimSpace(importMatch[2])
			}
			if value != "" {
				dependencies = append(dependencies, value)
			}
		}
	}
	if len(dependencies) == 0 {
		return nil, false, nil
	}
	return dependencies, true, nil
}
