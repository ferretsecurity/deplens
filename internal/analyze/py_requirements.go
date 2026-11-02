package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/git-pkgs/vers"
)

type pyRequirementsMatcherConfig struct{}

type pyRequirementsMatcher struct{}

var pyRequirementConstraintPattern = regexp.MustCompile(`^(===|==|!=|~=|<=|>=|<|>)\s*([^\s,;()]+)$`)

var pyRequirementNamePattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$`)

var pyRequirementCompatibleReleasePattern = regexp.MustCompile(`^[0-9]+(?:![0-9]+)?\.[0-9]`)

func newPyRequirementsMatcher(raw pyRequirementsMatcherConfig) (sourceAnalyzer, error) {
	return pyRequirementsMatcher{}, nil
}

func (m pyRequirementsMatcher) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	deps, diagnosticMessages := m.collectDependencies(path, content, map[string]bool{})
	if len(deps) > 0 {
		return sourceAnalyzerResult{
			Dependencies: deps,
			Analysis: SourceAnalysis{
				Presence: PresencePresent,
				Extraction: func() ExtractionState {
					if len(diagnosticMessages) > 0 {
						return ExtractionPartial
					}
					return ExtractionComplete
				}(),
			},
			Diagnostics: diagnosticsFromMessages(DiagnosticWarning, "included-requirement-unreadable", diagnosticMessages),
			Recognized:  true,
		}, nil
	}
	if len(diagnosticMessages) > 0 {
		return sourceAnalyzerResult{
			Analysis:    failedAnalysis(),
			Diagnostics: diagnosticsFromMessages(DiagnosticError, "included-requirement-unreadable", diagnosticMessages),
			Recognized:  true,
		}, nil
	}
	return sourceAnalyzerResult{
		Analysis:   completeAnalysis(nil),
		Recognized: true,
	}, nil
}

func (m pyRequirementsMatcher) collectDependencies(path string, content []byte, active map[string]bool) ([]DependencyReference, []string) {
	logicalLines := joinPyRequirementsContinuations(string(content))
	dependencies := make([]DependencyReference, 0, len(logicalLines))
	diagnosticMessages := make([]string, 0)

	cleanPath := filepath.Clean(path)
	active[cleanPath] = true
	defer delete(active, cleanPath)

	for _, line := range logicalLines {
		trimmed := stripPyRequirementsComment(line)
		if trimmed == "" {
			continue
		}
		if includeTarget, ok := parsePyRequirementsInclude(trimmed); ok {
			includedPath := filepath.Clean(filepath.Join(filepath.Dir(cleanPath), includeTarget))
			if active[includedPath] {
				diagnosticMessages = append(diagnosticMessages, fmt.Sprintf("detected requirements include cycle for %q via %q", includedPath, includeTarget))
				continue
			}

			includedContent, err := os.ReadFile(includedPath)
			if err != nil {
				diagnosticMessages = append(diagnosticMessages, fmt.Sprintf("could not read included requirements file %q: %v", includeTarget, err))
				continue
			}

			includedDependencies, includedDiagnostics := m.collectDependencies(includedPath, includedContent, active)
			dependencies = append(dependencies, includedDependencies...)
			diagnosticMessages = append(diagnosticMessages, includedDiagnostics...)
			continue
		}
		_, editable := pyRequirementsEditableTarget(trimmed)
		if strings.HasPrefix(trimmed, "-") && !editable {
			continue
		}
		dependencies = append(dependencies, parsePyRequirementsDependency(strings.TrimSpace(line)))
	}

	return dependencies, diagnosticMessages
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

func pyRequirementsEditableTarget(line string) (string, bool) {
	for _, prefix := range []string{"-e", "--editable"} {
		if strings.HasPrefix(line, prefix+" ") || strings.HasPrefix(line, prefix+"\t") || strings.HasPrefix(line, prefix+"=") {
			return strings.TrimSpace(line[len(prefix)+1:]), true
		}
	}
	return line, false
}

func parsePyRequirementsDependency(raw string) DependencyReference {
	declaration, options := splitPyRequirementsOptions(stripPyRequirementsComment(raw))
	attributes, valid := parsePyRequirementsOptions(options)
	if !valid {
		return DependencyReference{Raw: raw}
	}
	declaration, editable := pyRequirementsEditableTarget(declaration)
	if editable {
		attributes["editable"] = "true"
	}
	parsed := parsePythonDependencySource(declaration)
	if editable && parsed.originKind == "" && declaration != "" && !strings.ContainsAny(declaration, " \t@;=") {
		parsed = pep508Dependency{originKind: OriginPath, attributes: map[string]string{"source_path": declaration}}
	}
	if parsed.originKind == "" {
		parsed = parsePEP508Dep(declaration)
		if !validPyRequirementsDeclaration(parsed, declaration) {
			return DependencyReference{Raw: raw}
		}
	}
	for key, value := range parsed.attributes {
		attributes[key] = value
	}
	if len(attributes) == 0 {
		attributes = nil
	}
	return DependencyReference{Raw: raw, Name: parsed.name, VersionConstraint: parsed.versionConstraint, OriginKind: parsed.originKind, Attributes: attributes}
}

func validPyRequirementsDeclaration(parsed pep508Dependency, declaration string) bool {
	if !pyRequirementNamePattern.MatchString(parsed.name) {
		return false
	}
	if marker := parsed.attributes["marker"]; marker != "" && validatePEP508Marker(marker) != nil {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(declaration, parsed.name))
	if strings.HasPrefix(rest, "[") {
		close := strings.IndexByte(rest, ']')
		if close < 0 {
			return false
		}
		if extras := strings.TrimSpace(rest[1:close]); extras != "" {
			for _, extra := range strings.Split(extras, ",") {
				if !pyRequirementNamePattern.MatchString(strings.TrimSpace(extra)) {
					return false
				}
			}
		}
		rest = strings.TrimSpace(rest[close+1:])
	}
	if parsed.originKind != "" {
		return strings.HasPrefix(rest, "@")
	}
	constraint, marker := splitPEP508Marker(rest)
	if strings.Contains(rest, ";") && marker == "" {
		return false
	}
	if constraint == "" {
		return rest == "" || strings.HasPrefix(rest, ";")
	}
	if strings.HasPrefix(constraint, "(") && strings.HasSuffix(constraint, ")") {
		constraint = strings.TrimSpace(constraint[1 : len(constraint)-1])
	}
	for _, clause := range strings.Split(strings.TrimSuffix(strings.TrimSpace(constraint), ","), ",") {
		parts := pyRequirementConstraintPattern.FindStringSubmatch(strings.TrimSpace(clause))
		if parts == nil {
			return false
		}
		if parts[1] != "===" {
			if strings.HasSuffix(parts[2], ".*") && parts[1] != "==" && parts[1] != "!=" {
				return false
			}
			version, err := vers.NormalizeWithScheme(strings.TrimSuffix(parts[2], ".*"), "pypi")
			if err != nil || parts[1] == "~=" && !pyRequirementCompatibleReleasePattern.MatchString(version) {
				return false
			}
		}
	}
	return true
}

func splitPyRequirementsOptions(line string) (string, string) {
	var quote byte
	for i := 0; i < len(line); i++ {
		if quote != 0 && line[i] == '\\' && i+1 < len(line) {
			i++
			continue
		}
		if line[i] == '\'' || line[i] == '"' {
			if quote == 0 {
				quote = line[i]
			} else if quote == line[i] {
				quote = 0
			}
		}
		if quote == 0 && i > 0 && (line[i-1] == ' ' || line[i-1] == '\t') && strings.HasPrefix(line[i:], "--") {
			return strings.TrimSpace(line[:i]), line[i:]
		}
	}
	return line, ""
}

func parsePyRequirementsOptions(options string) (map[string]string, bool) {
	attributes := make(map[string]string)
	words, valid := splitPyRequirementsOptionWords(options)
	if !valid {
		return nil, false
	}
	for i := 0; i < len(words); i++ {
		key, value, assigned := strings.Cut(words[i], "=")
		switch key {
		case "--hash", "--config-settings", "--global-option", "--install-option":
		default:
			return nil, false
		}
		if !assigned {
			i++
			if i >= len(words) || strings.HasPrefix(words[i], "--") {
				return nil, false
			}
			value = words[i]
		}
		if value == "" {
			return nil, false
		}
		if key == "--hash" {
			algorithm, digest, ok := strings.Cut(value, ":")
			if !ok || algorithm == "" || digest == "" {
				return nil, false
			}
			if attributes["hashes"] != "" {
				attributes["hashes"] += ","
			}
			attributes["hashes"] += value
		}
	}
	return attributes, true
}

func splitPyRequirementsOptionWords(options string) ([]string, bool) {
	var words []string
	var word strings.Builder
	var quote byte
	started := false
	for i := 0; i < len(options); i++ {
		c := options[i]
		switch {
		case c == '\\' && quote != '\'':
			if i+1 == len(options) {
				return nil, false
			}
			next := options[i+1]
			if quote == '"' && next != '\\' && next != '"' && next != '$' && next != '`' {
				word.WriteByte(c)
			} else {
				i++
				word.WriteByte(next)
			}
			started = true
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				word.WriteByte(c)
			}
		case c == '\'' || c == '"':
			quote = c
			started = true
		case c == ' ' || c == '\t':
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
		default:
			word.WriteByte(c)
			started = true
		}
	}
	if started {
		words = append(words, word.String())
	}
	return words, quote == 0
}
