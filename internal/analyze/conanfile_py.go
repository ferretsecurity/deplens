package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

type conanfilePyParser struct{}

var (
	conanfilePyConanImport          = regexp.MustCompile(`(?m)^\s*from\s+conans?\s+import\b.*\bConanFile\b`)
	conanfilePyRequirementCall      = regexp.MustCompile(`(?m)^[\t ]*self\.(requires|build_requires|tool_requires|test_requires|python_requires)[\t ]*\(`)
	conanfilePyRequirementAttribute = regexp.MustCompile(`(?m)^[\t ]+(requires|build_requires|tool_requires|test_requires|python_requires)[\t ]*=`)
)

func newConanfilePyParser(conanfilePyMatcherConfig) (sourceAnalyzer, error) {
	return conanfilePyParser{}, nil
}

func (conanfilePyParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	source := string(content)
	if !conanfilePyConanImport.MatchString(source) {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seen := make(map[string]struct{})
	for _, match := range conanfilePyRequirementCall.FindAllStringSubmatchIndex(source, -1) {
		group := conanfileDependencyGroups[source[match[2]:match[3]]]
		reference, ok := conanfilePyFirstStringArgument(source, match[1])
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("conanfile.py %s call has a dynamic dependency reference", group.name))
			continue
		}
		dependency, ok := conanfileReference(reference)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("conanfile.py %s call has an invalid dependency reference", group.name))
			continue
		}
		dependency.SourceGroup = group.name
		dependency.Scope = group.scope
		key := dependency.SourceGroup + "\x00" + dependency.Raw
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}

	for _, match := range conanfilePyRequirementAttribute.FindAllStringSubmatchIndex(source, -1) {
		group := conanfileDependencyGroups[source[match[2]:match[3]]]
		references, ok := conanfilePyStringTuple(source, match[1])
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("conanfile.py %s attribute has a dynamic dependency reference", group.name))
			continue
		}
		for _, reference := range references {
			dependency, ok := conanfileReference(reference)
			if !ok {
				incomplete = append(incomplete, fmt.Sprintf("conanfile.py %s attribute has an invalid dependency reference", group.name))
				continue
			}
			dependency.SourceGroup = group.name
			dependency.Scope = group.scope
			key := dependency.SourceGroup + "\x00" + dependency.Raw
			dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func conanfilePyFirstStringArgument(source string, offset int) (string, bool) {
	value, next, ok := conanfilePyString(source, offset)
	if !ok {
		return "", false
	}
	if next <= offset {
		return "", false
	}
	return value, true
}

func conanfilePyStringTuple(source string, offset int) ([]string, bool) {
	lineEnd := strings.IndexByte(source[offset:], '\n')
	if lineEnd < 0 {
		lineEnd = len(source)
	} else {
		lineEnd += offset
	}
	value := source[offset:lineEnd]
	references := make([]string, 0)
	for index := 0; ; {
		for index < len(value) && strings.ContainsRune(" \t,[]()", rune(value[index])) {
			index++
		}
		if index == len(value) || value[index] == '#' {
			return references, len(references) > 0
		}
		reference, next, ok := conanfilePyString(value, index)
		if !ok {
			return nil, false
		}
		references = append(references, reference)
		index = next
	}
}

func conanfilePyString(source string, offset int) (string, int, bool) {
	for offset < len(source) {
		if strings.ContainsRune(" \t\r\n", rune(source[offset])) {
			offset++
			continue
		}
		if source[offset] == '#' {
			for offset < len(source) && source[offset] != '\n' {
				offset++
			}
			continue
		}
		break
	}
	start := offset
	for offset < len(source) && strings.ContainsRune("rRuUbBfF", rune(source[offset])) {
		offset++
	}
	if offset == len(source) || (source[offset] != '\'' && source[offset] != '"') || strings.ContainsRune(source[start:offset], 'f') || strings.ContainsRune(source[start:offset], 'F') {
		return "", start, false
	}
	quote := source[offset]
	offset++
	var value strings.Builder
	escaped := false
	for offset < len(source) {
		char := source[offset]
		offset++
		if escaped {
			value.WriteByte(char)
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == quote {
			return value.String(), offset, true
		}
		value.WriteByte(char)
	}
	return "", start, false
}
