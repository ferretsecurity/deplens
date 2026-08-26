package analyze

import (
	"fmt"
	"strings"
)

type swiftPackageParser struct{}

func newSwiftPackageParser(swiftPackageMatcherConfig) (sourceAnalyzer, error) {
	return swiftPackageParser{}, nil
}

func (swiftPackageParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	cleaned, err := stripExecutableDSLComments(path, string(content))
	if err != nil {
		return sourceAnalyzerResult{}, err
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seen := make(map[string]struct{})
	for _, call := range swiftPackageCalls(cleaned) {
		dependency, ok := swiftPackageDependency(call.arguments)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("Package.swift .package declaration on line %d could not be extracted", call.line))
			continue
		}
		key := dependency.SourceGroup + "\x00" + dependency.Raw
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

type swiftPackageCall struct {
	arguments string
	line      int
}

func swiftPackageCalls(content string) []swiftPackageCall {
	calls := make([]swiftPackageCall, 0)
	for index := 0; index < len(content); index++ {
		if content[index] == '"' {
			index = swiftStringEnd(content, index)
			continue
		}
		if !strings.HasPrefix(content[index:], ".package") || !swiftPackageCallBoundary(content, index+len(".package")) {
			continue
		}
		open := index + len(".package")
		for open < len(content) && (content[open] == ' ' || content[open] == '\t' || content[open] == '\n' || content[open] == '\r') {
			open++
		}
		if open == len(content) || content[open] != '(' {
			continue
		}
		close, ok := swiftClosingParenthesis(content, open)
		if !ok {
			continue
		}
		calls = append(calls, swiftPackageCall{
			arguments: content[open+1 : close],
			line:      strings.Count(content[:index], "\n") + 1,
		})
		index = close
	}
	return calls
}

func swiftPackageCallBoundary(content string, index int) bool {
	if index == len(content) {
		return true
	}
	character := content[index]
	return character == '(' || character == ' ' || character == '\t' || character == '\n' || character == '\r'
}

func swiftClosingParenthesis(content string, open int) (int, bool) {
	depth := 0
	for index := open; index < len(content); index++ {
		if content[index] == '"' {
			index = swiftStringEnd(content, index)
			continue
		}
		switch content[index] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return index, true
			}
		}
	}
	return 0, false
}

func swiftStringEnd(content string, start int) int {
	if strings.HasPrefix(content[start:], "\"\"\"") {
		for index := start + 3; index+2 < len(content); index++ {
			if content[index] == '\\' {
				index++
				continue
			}
			if strings.HasPrefix(content[index:], "\"\"\"") {
				return index + 2
			}
		}
		return len(content) - 1
	}
	for index := start + 1; index < len(content); index++ {
		if content[index] == '\\' {
			index++
			continue
		}
		if content[index] == '"' {
			return index
		}
	}
	return len(content) - 1
}

func swiftPackageDependency(arguments string) (DependencyReference, bool) {
	values := swiftPackageArgumentValues(arguments)
	name := values["name"]
	dependency := DependencyReference{
		SourceGroup:  "dependencies",
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}

	switch {
	case values["url"] != "":
		dependency.OriginKind = OriginGit
		dependency.Attributes = map[string]string{"source_url": values["url"]}
		if name == "" {
			name = swiftPackageLocationName(values["url"])
		}
	case values["path"] != "":
		dependency.OriginKind = OriginPath
		dependency.Attributes = map[string]string{"source_path": values["path"]}
		if name == "" {
			name = swiftPackageLocationName(values["path"])
		}
	case values["id"] != "":
		dependency.OriginKind = OriginRegistry
		dependency.Attributes = map[string]string{"package_id": values["id"]}
		if name == "" {
			name = values["id"]
		}
	default:
		return DependencyReference{}, false
	}
	if name == "" {
		return DependencyReference{}, false
	}

	dependency.Name = name
	dependency.Raw = name
	for _, key := range []string{"from", "exact", "branch", "revision"} {
		if value := values[key]; value != "" {
			dependency.VersionConstraint = value
			dependency.Raw += "@" + value
			break
		}
	}
	return dependency, true
}

func swiftPackageArgumentValues(arguments string) map[string]string {
	values := make(map[string]string)
	for _, argument := range swiftPackageArguments(arguments) {
		label, value, ok := strings.Cut(argument, ":")
		if !ok {
			continue
		}
		label = strings.TrimSpace(label)
		value = strings.TrimSpace(value)
		if !swiftPackageArgumentLabel(label) || !swiftPackageStringLiteral(value) {
			continue
		}
		values[label] = value[1 : len(value)-1]
	}
	return values
}

func swiftPackageArguments(arguments string) []string {
	parts := make([]string, 0)
	start, depth := 0, 0
	for index := 0; index < len(arguments); index++ {
		if arguments[index] == '"' {
			index = swiftStringEnd(arguments, index)
			continue
		}
		switch arguments[index] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, arguments[start:index])
				start = index + 1
			}
		}
	}
	return append(parts, arguments[start:])
}

func swiftPackageArgumentLabel(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if !(character == '_' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || index > 0 && character >= '0' && character <= '9') {
			return false
		}
	}
	return true
}

func swiftPackageStringLiteral(value string) bool {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return false
	}
	return swiftStringEnd(value, 0) == len(value)-1
}

func swiftPackageLocationName(location string) string {
	location = strings.TrimSuffix(strings.TrimRight(location, "/"), ".git")
	if index := strings.LastIndexAny(location, "/:"); index >= 0 {
		location = location[index+1:]
	}
	return location
}
