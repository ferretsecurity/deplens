package analyze

import (
	"fmt"
	"regexp"
)

const bannerRegexScanLimit = 4096

type bannerRegexParser struct {
	regex *regexp.Regexp
}

func newBannerRegexParser(pattern string) (sourceAnalyzer, error) {
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

func (p bannerRegexParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	if len(content) > bannerRegexScanLimit {
		content = content[:bannerRegexScanLimit]
	}

	match := p.regex.FindSubmatch(content)
	if len(match) == 0 {
		return sourceAnalyzerResult{}, nil
	}

	name := string(match[1])
	version := string(match[2])
	if name == "" || version == "" {
		return sourceAnalyzerResult{}, nil
	}

	return sourceAnalyzerResult{
		Dependencies: dependenciesFromStrings([]string{fmt.Sprintf("%s@%s", name, version)}),
		Analysis:     SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		Recognized:   true,
	}, nil
}
