package analyze

import (
	"fmt"
	"regexp"
)

const bannerRegexScanLimit = 4096

type bannerRegexParser struct {
	regex *regexp.Regexp
}

func newBannerRegexParser(pattern string) (manifestParser, error) {
	if pattern == "" {
		return nil, fmt.Errorf("banner-regex: required")
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("banner-regex: compile %q: %w", pattern, err)
	}
	if compiled.NumSubexp() < 2 {
		return nil, fmt.Errorf("banner-regex: must define at least two capture groups")
	}

	return bannerRegexParser{regex: compiled}, nil
}

func (p bannerRegexParser) Match(path string, content []byte) (manifestParserResult, error) {
	if len(content) > bannerRegexScanLimit {
		content = content[:bannerRegexScanLimit]
	}

	match := p.regex.FindSubmatch(content)
	if len(match) == 0 {
		return manifestParserResult{}, nil
	}

	name := string(match[1])
	version := string(match[2])
	if name == "" || version == "" {
		return manifestParserResult{}, nil
	}

	return manifestParserResult{
		Dependencies:    dependenciesFromStrings([]string{fmt.Sprintf("%s@%s", name, version)}),
		HasDependencies: boolPtr(true),
		Matched:         true,
	}, nil
}
