package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type pyRequirementsMatcherConfig struct{}

type pyRequirementsMatcher struct{}

var ignoredPyRequirementsLinePrefixes = []string{
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

func (m pyRequirementsMatcher) Match(path string, content []byte) (manifestParserResult, error) {
	rawDeps, warnings := m.collectDependencies(path, content, map[string]bool{})
	if len(rawDeps) > 0 {
		deps := make([]Dependency, 0, len(rawDeps))
		for _, spec := range rawDeps {
			name, rest := parsePEP508Dep(spec)
			dep := Dependency{Raw: spec}
			if name != "" {
				dep.Name = name
				dep.Constraint = rest
			}
			deps = append(deps, dep)
		}
		return manifestParserResult{
			Dependencies:    deps,
			HasDependencies: boolPtr(true),
			Warnings:        warnings,
			Matched:         true,
		}, nil
	}
	if len(warnings) > 0 {
		return manifestParserResult{
			Warnings: warnings,
			Matched:  true,
		}, nil
	}
	return manifestParserResult{
		HasDependencies: boolPtr(false),
		Matched:         true,
	}, nil
}

func (m pyRequirementsMatcher) collectDependencies(path string, content []byte, active map[string]bool) ([]string, []string) {
	logicalLines := joinPyRequirementsContinuations(string(content))
	dependencies := make([]string, 0, len(logicalLines))
	warnings := make([]string, 0)

	cleanPath := filepath.Clean(path)
	active[cleanPath] = true
	defer delete(active, cleanPath)

	for _, line := range logicalLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if includeTarget, ok := parsePyRequirementsInclude(trimmed); ok {
			includedPath := filepath.Clean(filepath.Join(filepath.Dir(cleanPath), includeTarget))
			if active[includedPath] {
				warnings = append(warnings, fmt.Sprintf("detected requirements include cycle for %q via %q", includedPath, includeTarget))
				continue
			}

			includedContent, err := os.ReadFile(includedPath)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("could not read included requirements file %q: %v", includeTarget, err))
				continue
			}

			includedDependencies, includedWarnings := m.collectDependencies(includedPath, includedContent, active)
			dependencies = append(dependencies, includedDependencies...)
			warnings = append(warnings, includedWarnings...)
			continue
		}
		if isIgnoredPyRequirementsDirective(trimmed) {
			continue
		}
		dependencies = append(dependencies, trimmed)
	}

	return dependencies, warnings
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

func parsePyRequirementsInclude(line string) (string, bool) {
	for _, prefix := range []string{"-r", "--requirement", "--requirements"} {
		if line == prefix {
			return "", false
		}
		if strings.HasPrefix(line, prefix+" ") || strings.HasPrefix(line, prefix+"\t") {
			target := strings.TrimSpace(line[len(prefix):])
			return target, target != ""
		}
		if strings.HasPrefix(line, prefix+"=") {
			target := strings.TrimSpace(line[len(prefix)+1:])
			return target, target != ""
		}
	}
	return "", false
}
