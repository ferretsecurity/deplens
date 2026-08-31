package analyze

import (
	"regexp"
	"strings"
)

type mesonParser struct{}

var (
	mesonDependencyCall     = regexp.MustCompile(`(?m)(?:^|[^A-Za-z0-9_.])dependency\s*\(`)
	mesonFindLibraryCall    = regexp.MustCompile(`(?m)\b[A-Za-z_][A-Za-z0-9_]*\s*\.\s*find_library\s*\(`)
	mesonPythonInstallation = regexp.MustCompile(`(?m)^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*import\s*\(\s*['"]python['"]\s*\)\s*\.\s*find_installation\s*\(`)
	mesonVersionArgument    = regexp.MustCompile(`(?m)\bversion\s*:\s*['"]([^'"\r\n]*)['"]`)
)

func newMesonParser(mesonMatcherConfig) (sourceAnalyzer, error) {
	return mesonParser{}, nil
}

func (mesonParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	source := mesonWithoutComments(string(content))
	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})

	for _, match := range mesonDependencyCall.FindAllStringIndex(source, -1) {
		openParen := match[0] + strings.LastIndexByte(source[match[0]:match[1]], '(')
		body, ok := mesonCallBody(source, openParen)
		if !ok {
			continue
		}
		name, ok := mesonFirstStringArgument(body)
		if !ok {
			continue
		}
		dependency := mesonDependency(name, "dependency", body)
		key := dependency.SourceGroup + "\x00" + dependency.Name + "\x00" + dependency.VersionConstraint
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	for _, match := range mesonFindLibraryCall.FindAllStringIndex(source, -1) {
		openParen := match[0] + strings.LastIndexByte(source[match[0]:match[1]], '(')
		body, ok := mesonCallBody(source, openParen)
		if !ok {
			continue
		}
		name, ok := mesonFirstStringArgument(body)
		if !ok {
			continue
		}
		dependency := mesonDependency(name, "find_library", "")
		key := dependency.SourceGroup + "\x00" + dependency.Name
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	for _, match := range mesonPythonInstallation.FindAllStringSubmatch(source, -1) {
		pythonDependency := regexp.MustCompile(`(?m)\b` + regexp.QuoteMeta(match[1]) + `\s*\.\s*dependency\s*\(\s*\)`)
		if !pythonDependency.MatchString(source) {
			continue
		}
		dependency := mesonDependency("python", "dependency", "")
		key := dependency.SourceGroup + "\x00" + dependency.Name
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func mesonDependency(name, group, body string) DependencyReference {
	dependency := DependencyReference{
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if version := mesonStaticVersion(body); version != "" {
		dependency.Raw += "@" + version
		dependency.VersionConstraint = version
	}
	return dependency
}

func mesonStaticVersion(body string) string {
	match := mesonVersionArgument.FindStringSubmatch(body)
	if match == nil {
		return ""
	}
	return match[1]
}

func mesonFirstStringArgument(body string) (string, bool) {
	trimmed := strings.TrimLeft(body, " \t\r\n")
	if len(trimmed) < 2 || (trimmed[0] != '\'' && trimmed[0] != '"') {
		return "", false
	}
	quote := trimmed[0]
	for index := 1; index < len(trimmed); index++ {
		if trimmed[index] == '\\' {
			index++
			continue
		}
		if trimmed[index] == quote {
			return trimmed[1:index], true
		}
	}
	return "", false
}

func mesonCallBody(source string, openParen int) (string, bool) {
	depth := 1
	var quote byte
	escaped := false
	for index := openParen + 1; index < len(source); index++ {
		current := source[index]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == quote {
				quote = 0
			}
			continue
		}
		switch current {
		case '\'', '"':
			quote = current
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return source[openParen+1 : index], true
			}
		}
	}
	return "", false
}

func mesonWithoutComments(source string) string {
	var output strings.Builder
	output.Grow(len(source))
	var quote byte
	escaped := false
	for index := 0; index < len(source); index++ {
		current := source[index]
		if quote != 0 {
			output.WriteByte(current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == quote {
				quote = 0
			}
			continue
		}
		if current == '\'' || current == '"' {
			quote = current
			output.WriteByte(current)
			continue
		}
		if current == '#' {
			for index < len(source) && source[index] != '\n' {
				output.WriteByte(' ')
				index++
			}
			if index < len(source) {
				output.WriteByte('\n')
			}
			continue
		}
		output.WriteByte(current)
	}
	return output.String()
}
