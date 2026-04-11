package analyze

import "strings"

type pyRequirementsMatcherConfig struct{}

type pyRequirementsMatcher struct{}

var ignoredPyRequirementsLinePrefixes = []string{
	"-r",
	"--requirement",
	"-c",
	"--constraint",
	"--index-url",
	"--extra-index-url",
	"--find-links",
	"--trusted-host",
	"--hash",
}

func newPyRequirementsMatcher(raw pyRequirementsMatcherConfig) (manifestParser, error) {
	return pyRequirementsMatcher{}, nil
}

func (m pyRequirementsMatcher) Match(path string, content []byte) ([]Dependency, *bool, bool, error) {
	logicalLines := joinPyRequirementsContinuations(string(content))
	dependencies := make([]string, 0, len(logicalLines))

	for _, line := range logicalLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if isIgnoredPyRequirementsDirective(trimmed) {
			continue
		}
		dependencies = append(dependencies, trimmed)
	}

	if len(dependencies) == 0 {
		return nil, boolPtr(false), true, nil
	}
	return dependenciesFromStrings(dependencies), boolPtr(true), true, nil
}

func joinPyRequirementsContinuations(content string) []string {
	lines := strings.Split(content, "\n")
	joined := make([]string, 0, len(lines))
	var current strings.Builder

	flushCurrent := func() {
		if current.Len() == 0 {
			joined = append(joined, "")
			return
		}
		joined = append(joined, current.String())
		current.Reset()
	}

	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		trimmedRight := strings.TrimRight(line, " \t")
		continued := strings.HasSuffix(trimmedRight, `\`)
		if continued {
			trimmedRight = strings.TrimSuffix(trimmedRight, `\`)
		}

		part := strings.TrimSpace(trimmedRight)
		if current.Len() > 0 && part != "" {
			current.WriteByte(' ')
		}
		current.WriteString(part)

		if continued {
			continue
		}
		flushCurrent()
	}

	if current.Len() > 0 {
		flushCurrent()
	}

	return joined
}

func isIgnoredPyRequirementsDirective(line string) bool {
	for _, prefix := range ignoredPyRequirementsLinePrefixes {
		if line == prefix {
			return true
		}
		if strings.HasPrefix(line, prefix+" ") || strings.HasPrefix(line, prefix+"\t") {
			return true
		}
		if strings.HasPrefix(line, prefix+"=") {
			return true
		}
	}
	return false
}
