package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	scriptBlockRegexp       = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
	srcAttrRegexp           = regexp.MustCompile(`(?is)\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	typeAttrRegexp          = regexp.MustCompile(`(?is)\btype\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	moduleImportRegexp      = regexp.MustCompile(`(?is)\bimport\s+(?:[^"'()]+?\s+from\s+)?(?:"(https?://[^"]+)"|'(https?://[^']+)')`)
	importMapImportsRegexp  = regexp.MustCompile(`(?is)"imports"\s*:\s*\{(.*?)\}`)
	importMapHTTPURLRegexp  = regexp.MustCompile(`(?is)"[^"]+"\s*:\s*"(https?://[^"]+)"`)
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

		src := firstNonEmptyMatch(srcAttrRegexp.FindStringSubmatch(tagAttrs)[1:]...)
		if src != "" {
			value := strings.TrimSpace(src)
			if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
				dependencies = append(dependencies, value)
			}
			continue
		}

		if strings.EqualFold(firstNonEmptyMatch(typeAttrRegexp.FindStringSubmatch(tagAttrs)[1:]...), "importmap") {
			importMapMatches := importMapImportsRegexp.FindAllStringSubmatch(body, -1)
			for _, importMapMatch := range importMapMatches {
				urlMatches := importMapHTTPURLRegexp.FindAllStringSubmatch(importMapMatch[1], -1)
				for _, urlMatch := range urlMatches {
					if value := strings.TrimSpace(firstNonEmptyMatch(urlMatch[1:]...)); value != "" {
						dependencies = append(dependencies, value)
					}
				}
			}
			continue
		}

		imports := moduleImportRegexp.FindAllStringSubmatch(body, -1)
		for _, importMatch := range imports {
			value := strings.TrimSpace(firstNonEmptyMatch(importMatch[1:]...))
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

func firstNonEmptyMatch(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
