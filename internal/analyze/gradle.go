package analyze

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

type gradleBuildParser struct{}
type gradleLockParser struct{}
type gradleVersionCatalogParser struct{}

var (
	gradleDoubleQuotedCall = regexp.MustCompile(`(?m)(?:^|\n|;)\s*([A-Za-z_][A-Za-z0-9_.]*)\s*(?:\(\s*)?"([^"\n]+)"`)
	gradleSingleQuotedCall = regexp.MustCompile(`(?m)(?:^|\n|;)\s*([A-Za-z_][A-Za-z0-9_.]*)\s*(?:\(\s*)?'([^'\n]+)'`)
	gradlePlatformCall     = regexp.MustCompile(`(?m)(?:^|\n|;)\s*([A-Za-z_][A-Za-z0-9_.]*)\s*(?:\(\s*)?(?:enforcedPlatform|platform)\s*\(\s*["']([^"'\n]+)["']`)
	gradleAliasCall        = regexp.MustCompile(`(?m)(?:^|\n|;)\s*([A-Za-z_][A-Za-z0-9_.]*)\s*\(?\s*(libs(?:\.[A-Za-z_][A-Za-z0-9_-]*)+)`)
	gradleMapCall          = regexp.MustCompile(`(?ms)^\s*([A-Za-z_][A-Za-z0-9_.]*)\s*(?:\(([^)]*)\)|([^\n]+))`)
	gradleMapPair          = regexp.MustCompile(`\b(group|name|version)\s*[:=]\s*["']([^"']+)["']`)
	gradleMapKey           = regexp.MustCompile(`\b(group|name|version)\s*[:=]`)
	gradleRichVersionToken = regexp.MustCompile(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|\bversion\s*\{`)
	gradleDeclarationHead  = regexp.MustCompile(`(?m)(?:^|\n|;)[ \t]*([A-Za-z_][A-Za-z0-9_.]*)`)
	gradleLocalDependency  = regexp.MustCompile(`^\s*\(?\s*(?:project|files|fileTree|testFixtures)\s*\(`)
	gradleContainerCall    = regexp.MustCompile(`^(?:if|else|for|while|when|switch|catch|synchronized|with|run|let|apply|also|use|listOf|setOf|sequenceOf|arrayOf)$`)
)

func newGradleBuildParser(gradleBuildMatcherConfig) (sourceAnalyzer, error) {
	return gradleBuildParser{}, nil
}

func newGradleLockParser(gradleLockMatcherConfig) (sourceAnalyzer, error) {
	return gradleLockParser{}, nil
}

func newGradleVersionCatalogParser(gradleVersionCatalogMatcherConfig) (sourceAnalyzer, error) {
	return gradleVersionCatalogParser{}, nil
}

func (gradleBuildParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	cleaned, err := stripExecutableDSLComments(path, string(content))
	if err != nil {
		return sourceAnalyzerResult{}, err
	}

	segments, err := gradleDependencySegments(path, cleaned)
	if err != nil {
		return sourceAnalyzerResult{}, err
	}
	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})
	incomplete := make([]string, 0)
	dynamicSeen := make(map[string]struct{})

	for _, segment := range segments {
		masked := []byte(segment.text)
		for _, location := range gradleRichVersionToken.FindAllStringIndex(segment.text, -1) {
			if masked[location[0]] != 'v' {
				continue
			}
			end, ok := executableDSLMatchingBrace(segment.text, location[1])
			if !ok {
				return sourceAnalyzerResult{}, fmt.Errorf("parse Gradle build %q: unterminated rich version block", path)
			}
			for index := location[0]; index <= end; index++ {
				if masked[index] != '\n' {
					masked[index] = ' '
				}
			}
			incomplete = appendUniqueMessage(incomplete, dynamicSeen, "Gradle rich version declarations could not be extracted")
		}
		segment.text = string(masked)

		for _, location := range gradleDeclarationHead.FindAllStringSubmatchIndex(segment.text, -1) {
			configuration := segment.text[location[2]:location[3]]
			if gradleContainerCall.MatchString(lastIdentifier(configuration)) {
				continue
			}
			start := gradleAttachedClosure(segment.text, location[3])
			if start < 0 {
				continue
			}
			head := segment.text[location[0]:start]
			dependencyCall := gradleMapKey.MatchString(head) || gradleAliasCall.MatchString(head) ||
				gradleLocalDependency.MatchString(segment.text[location[3]:start])
			for _, matcher := range []*regexp.Regexp{gradleDoubleQuotedCall, gradleSingleQuotedCall, gradlePlatformCall} {
				if match := matcher.FindStringSubmatch(head); match != nil {
					if _, ok := gradleCoordinateDependency(match[2], configuration, segment.context); ok {
						dependencyCall = true
					}
				}
			}
			if !dependencyCall {
				continue
			}
			end, ok := executableDSLMatchingBrace(segment.text, start+1)
			if !ok {
				return sourceAnalyzerResult{}, fmt.Errorf("parse Gradle build %q: unterminated dependency configuration block", path)
			}
			for index := start; index <= end; index++ {
				if masked[index] != '\n' {
					masked[index] = ' '
				}
			}
		}
		segment.text = string(masked)

		for _, matcher := range []*regexp.Regexp{gradleDoubleQuotedCall, gradleSingleQuotedCall, gradlePlatformCall} {
			for _, match := range matcher.FindAllStringSubmatch(segment.text, -1) {
				configuration := lastIdentifier(match[1])
				if gradleContainerCall.MatchString(configuration) {
					continue
				}
				coordinate := strings.TrimSpace(match[2])
				if strings.Count(coordinate, ":") < 1 {
					continue
				}
				if strings.ContainsAny(coordinate, "$") {
					message := fmt.Sprintf("dynamic Gradle dependency in configuration %s could not be extracted: %s", configuration, coordinate)
					incomplete = appendUniqueMessage(incomplete, dynamicSeen, message)
					continue
				}
				dependency, ok := gradleCoordinateDependency(coordinate, configuration, segment.context)
				if !ok {
					continue
				}
				key := dependency.SourceGroup + "\x00" + dependency.Name + "\x00" + dependency.VersionConstraint
				dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
			}
		}

		for _, match := range gradleMapCall.FindAllStringSubmatch(segment.text, -1) {
			configuration := lastIdentifier(match[1])
			body := match[2]
			if body == "" {
				body = match[3]
			}
			values := make(map[string]string)
			for _, pair := range gradleMapPair.FindAllStringSubmatch(body, -1) {
				values[pair[1]] = pair[2]
			}
			mapKeys := make(map[string]struct{})
			for _, keyMatch := range gradleMapKey.FindAllStringSubmatch(body, -1) {
				mapKeys[keyMatch[1]] = struct{}{}
			}
			if values["group"] == "" || values["name"] == "" {
				if len(mapKeys) > 0 {
					message := fmt.Sprintf("dynamic Gradle map dependency in configuration %s could not be fully extracted", configuration)
					incomplete = appendUniqueMessage(incomplete, dynamicSeen, message)
				}
				continue
			}
			coordinate := values["group"] + ":" + values["name"]
			if values["version"] != "" {
				coordinate += ":" + values["version"]
			}
			dependency, ok := gradleCoordinateDependency(coordinate, configuration, segment.context)
			if !ok {
				continue
			}
			key := dependency.SourceGroup + "\x00" + dependency.Name + "\x00" + dependency.VersionConstraint
			dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
			if _, declaresVersion := mapKeys["version"]; declaresVersion && values["version"] == "" {
				message := fmt.Sprintf("dynamic Gradle version in configuration %s could not be extracted", configuration)
				incomplete = appendUniqueMessage(incomplete, dynamicSeen, message)
			}
		}

		for _, match := range gradleAliasCall.FindAllStringSubmatch(segment.text, -1) {
			message := fmt.Sprintf("version-catalog alias %s could not be resolved from this build file", match[2])
			incomplete = appendUniqueMessage(incomplete, dynamicSeen, message)
		}
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func gradleAttachedClosure(content string, start int) int {
	tail := strings.TrimLeft(content[start:], " \t\r\n")
	if tail == "" || tail[0] == '{' {
		return -1
	}
	start = len(content) - len(tail)
	var quote byte
	escaped := false
	depth := 0
	for index := start; index < len(content); index++ {
		current := content[index]
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
				tail := content[index+1:]
				trimmed := strings.TrimLeft(tail, " \t\r\n")
				if strings.HasPrefix(trimmed, "{") {
					return index + 1 + len(tail) - len(trimmed)
				}
				return -1
			}
		case '{':
			if depth == 0 {
				return index
			}
		case ';', '}':
			if depth == 0 {
				return -1
			}
		case '\n':
			if depth == 0 {
				tail := content[index:]
				trimmed := strings.TrimLeft(tail, " \t\r\n")
				if strings.HasPrefix(trimmed, "{") {
					return index + len(tail) - len(trimmed)
				}
				return -1
			}
		}
	}
	return -1
}

func executableDSLMatchingBrace(content string, start int) (int, bool) {
	depth := 1
	var quote byte
	escaped := false
	for index := start; index < len(content); index++ {
		current := content[index]
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
		if current == '"' || current == '\'' {
			quote = current
			continue
		}
		switch current {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index, true
			}
		}
	}
	return 0, false
}

type gradleDependencyContext uint8

const (
	gradleOutsideDependencies gradleDependencyContext = iota
	gradleDirectDependencies
	gradleConstraintDependencies
)

type gradleDependencySegment struct {
	text    string
	context gradleDependencyContext
}

var gradleContextBlock = regexp.MustCompile(`\b(dependencies|constraints)\s*$`)

func gradleDependencySegments(path, content string) ([]gradleDependencySegment, error) {
	segments := make([]gradleDependencySegment, 0)
	stack := []gradleDependencyContext{gradleOutsideDependencies}
	start := 0
	var quote byte
	escaped := false
	for index := 0; index < len(content); index++ {
		current := content[index]
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
		if current == '"' || current == '\'' {
			quote = current
			continue
		}
		if current != '{' && current != '}' {
			continue
		}
		context := stack[len(stack)-1]
		nextContext := context
		if current == '{' {
			if match := gradleContextBlock.FindStringSubmatch(content[start:index]); match != nil {
				switch match[1] {
				case "dependencies":
					if context == gradleOutsideDependencies {
						nextContext = gradleDirectDependencies
					}
				case "constraints":
					if context != gradleOutsideDependencies {
						nextContext = gradleConstraintDependencies
					}
				}
			}
			stack = append(stack, nextContext)
		} else if len(stack) > 1 {
			stack = stack[:len(stack)-1]
			nextContext = stack[len(stack)-1]
		}
		if context != nextContext {
			if context != gradleOutsideDependencies {
				segments = append(segments, gradleDependencySegment{text: content[start:index], context: context})
			}
			start = index + 1
		}
	}
	for _, context := range stack {
		if context != gradleOutsideDependencies {
			return nil, fmt.Errorf("parse Gradle build %q: unterminated dependencies block", path)
		}
	}
	return segments, nil
}

func gradleCoordinateDependency(coordinate, configuration string, context gradleDependencyContext) (DependencyReference, bool) {
	parts := strings.Split(coordinate, ":")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return DependencyReference{}, false
	}
	dependency := DependencyReference{
		Raw:          coordinate,
		Name:         parts[0] + ":" + parts[1],
		SourceGroup:  configuration,
		Relationship: RelationshipDirect,
		Scope:        gradleConfigurationScope(configuration),
	}
	if context == gradleConstraintDependencies {
		dependency.SourceGroup = "constraints." + configuration
		dependency.Relationship = RelationshipInconclusive
	}
	if len(parts) >= 3 && parts[2] != "" {
		version := parts[2]
		if before, extension, ok := strings.Cut(version, "@"); ok {
			version = before
			if extension != "" {
				dependency.Attributes = map[string]string{"extension": extension}
			}
		}
		dependency.VersionConstraint = normalizeGradleMavenConstraint(version)
	}
	if len(parts) >= 4 && parts[3] != "" {
		if dependency.Attributes == nil {
			dependency.Attributes = make(map[string]string)
		}
		dependency.Attributes["classifier"] = strings.Join(parts[3:], ":")
	}
	return dependency, true
}

func normalizeGradleMavenConstraint(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "(") ||
		strings.ContainsAny(value, "+*") {
		return value
	}
	return "[" + value + "]"
}

func gradleConfigurationScope(configuration string) DependencyScope {
	lower := strings.ToLower(configuration)
	switch {
	case strings.Contains(lower, "test"):
		return ScopeTest
	case strings.Contains(lower, "classpath"), strings.Contains(lower, "annotationprocessor"),
		strings.Contains(lower, "kapt"), strings.Contains(lower, "compileonly"):
		return ScopeBuild
	case strings.Contains(lower, "runtime"), strings.Contains(lower, "implementation"), lower == "api":
		return ScopeRuntime
	default:
		return ""
	}
}

func lastIdentifier(value string) string {
	if index := strings.LastIndex(value, "."); index >= 0 {
		return value[index+1:]
	}
	return value
}

func stripExecutableDSLComments(path, content string) (string, error) {
	var output strings.Builder
	output.Grow(len(content))
	var quote rune
	escaped := false
	blockComment := false
	runes := []rune(content)
	for index := 0; index < len(runes); index++ {
		current := runes[index]
		var next rune
		if index+1 < len(runes) {
			next = runes[index+1]
		}
		if blockComment {
			if current == '*' && next == '/' {
				blockComment = false
				output.WriteString("  ")
				index++
			} else if current == '\n' {
				output.WriteRune('\n')
			} else {
				output.WriteRune(' ')
			}
			continue
		}
		if quote != 0 {
			output.WriteRune(current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == quote {
				quote = 0
			}
			continue
		}
		if current == '"' || current == '\'' {
			quote = current
			output.WriteRune(current)
			continue
		}
		if current == '/' && next == '*' {
			blockComment = true
			output.WriteString("  ")
			index++
			continue
		}
		if current == '/' && next == '/' || current == '#' {
			for index < len(runes) && runes[index] != '\n' {
				output.WriteRune(' ')
				index++
			}
			if index < len(runes) {
				output.WriteRune('\n')
			}
			continue
		}
		output.WriteRune(current)
	}
	if quote != 0 {
		return "", fmt.Errorf("parse executable dependency file %q: unterminated quoted string", path)
	}
	if blockComment {
		return "", fmt.Errorf("parse executable dependency file %q: unterminated block comment", path)
	}
	return output.String(), nil
}

func (gradleLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})
	for lineNumber, rawLine := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || line == "empty=" {
			continue
		}
		coordinate, configurations, ok := strings.Cut(line, "=")
		if !ok {
			return sourceAnalyzerResult{}, fmt.Errorf("parse Gradle lockfile %q line %d: expected coordinate=configurations", path, lineNumber+1)
		}
		parts := strings.Split(strings.TrimSpace(coordinate), ":")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return sourceAnalyzerResult{}, fmt.Errorf("parse Gradle lockfile %q line %d: invalid module coordinate %q", path, lineNumber+1, coordinate)
		}
		name := parts[0] + ":" + parts[1]
		version := parts[2]
		dependency := DependencyReference{
			Raw:          name + "@" + version,
			Name:         name,
			Version:      version,
			Relationship: RelationshipInconclusive,
		}
		configurations = strings.TrimSpace(configurations)
		if configurations != "" {
			dependency.Attributes = map[string]string{"configurations": configurations}
			if !strings.Contains(configurations, ",") {
				dependency.Scope = gradleConfigurationScope(configurations)
			}
		}
		dependencies = appendUniqueDependency(dependencies, seen, dependency.Raw, dependency)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

type gradleVersionCatalogFile struct {
	Versions  map[string]any `toml:"versions"`
	Libraries map[string]any `toml:"libraries"`
}

func (gradleVersionCatalogParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var catalog gradleVersionCatalogFile
	if err := toml.Unmarshal(content, &catalog); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Gradle version catalog %q: %w", path, err)
	}

	dependencies := make([]DependencyReference, 0, len(catalog.Libraries))
	incomplete := make([]string, 0)
	seen := make(map[string]struct{})
	for alias, raw := range catalog.Libraries {
		module, version, versionRef, ok := gradleCatalogLibrary(raw, catalog.Versions)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("Gradle version-catalog alias %s could not be statically extracted", alias))
			continue
		}
		parts := strings.Split(module, ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			incomplete = append(incomplete, fmt.Sprintf("Gradle version-catalog alias %s has invalid module %q", alias, module))
			continue
		}
		dependency := DependencyReference{
			Raw:          module,
			Name:         module,
			SourceGroup:  "libraries",
			Relationship: RelationshipInconclusive,
			Attributes:   map[string]string{"alias": alias},
		}
		if version != "" {
			dependency.Raw += ":" + version
			dependency.VersionConstraint = normalizeGradleMavenConstraint(version)
		}
		if versionRef != "" {
			dependency.Attributes["version_ref"] = versionRef
		}
		key := dependency.Name + "\x00" + dependency.VersionConstraint
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func gradleCatalogLibrary(raw any, versions map[string]any) (module, version, versionRef string, ok bool) {
	switch value := raw.(type) {
	case string:
		parts := strings.Split(value, ":")
		if len(parts) < 2 {
			return "", "", "", false
		}
		module = parts[0] + ":" + parts[1]
		if len(parts) > 2 {
			version = strings.Join(parts[2:], ":")
		}
		return module, version, "", true
	case map[string]any:
		if rawModule, exists := value["module"].(string); exists {
			module = rawModule
		} else {
			group, groupOK := value["group"].(string)
			name, nameOK := value["name"].(string)
			if !groupOK || !nameOK {
				return "", "", "", false
			}
			module = group + ":" + name
		}
		switch rawVersion := value["version"].(type) {
		case string:
			version = rawVersion
		case map[string]any:
			if ref, exists := rawVersion["ref"].(string); exists {
				versionRef = ref
				version = gradleCatalogVersion(versions[ref])
				if version == "" {
					return "", "", "", false
				}
			} else {
				version = gradleCatalogVersion(rawVersion)
			}
		case nil:
		default:
			return "", "", "", false
		}
		return module, version, versionRef, true
	default:
		return "", "", "", false
	}
}

func gradleCatalogVersion(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case map[string]any:
		for _, key := range []string{"strictly", "require", "prefer"} {
			if version, ok := value[key].(string); ok {
				return version
			}
		}
	}
	return ""
}
