package analyze

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type dockerfileParser struct{}
type dockerComposeParser struct{}

var (
	dockerVariable = regexp.MustCompile(`\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))`)
	dockerFrom     = regexp.MustCompile(`(?i)^FROM\s+(.+)$`)
	dockerCopyFrom = regexp.MustCompile(`(?i)^COPY\s+(?:[^\n]*?\s)?--from=([^\s]+)`)
)

func newDockerfileParser(dockerfileMatcherConfig) (sourceAnalyzer, error) {
	return dockerfileParser{}, nil
}

func newDockerComposeParser(dockerComposeMatcherConfig) (sourceAnalyzer, error) {
	return dockerComposeParser{}, nil
}

func (dockerfileParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	lines, err := dockerfileLogicalLines(path, string(content))
	if err != nil {
		return sourceAnalyzerResult{}, err
	}

	args := make(map[string]string)
	stages := make(map[string]struct{})
	images := make([]DependencyReference, 0)
	copyImages := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	finalImageIndex := -1

	for lineNumber, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "ARG":
			if len(fields) < 2 {
				return sourceAnalyzerResult{}, fmt.Errorf("parse Dockerfile %q logical line %d: ARG requires a name", path, lineNumber+1)
			}
			name, value, hasValue := strings.Cut(fields[1], "=")
			if name == "" {
				return sourceAnalyzerResult{}, fmt.Errorf("parse Dockerfile %q logical line %d: ARG requires a name", path, lineNumber+1)
			}
			if hasValue {
				args[name] = value
			}
		case "FROM":
			match := dockerFrom.FindStringSubmatch(line)
			if match == nil {
				return sourceAnalyzerResult{}, fmt.Errorf("parse Dockerfile %q logical line %d: malformed FROM instruction", path, lineNumber+1)
			}
			fromFields := strings.Fields(match[1])
			platform := ""
			for len(fromFields) > 0 && strings.HasPrefix(fromFields[0], "--") {
				if value, ok := strings.CutPrefix(fromFields[0], "--platform="); ok {
					platform = value
				}
				fromFields = fromFields[1:]
			}
			if len(fromFields) == 0 {
				return sourceAnalyzerResult{}, fmt.Errorf("parse Dockerfile %q logical line %d: FROM requires an image", path, lineNumber+1)
			}
			rawImage := fromFields[0]
			image, unresolved := resolveDockerVariables(rawImage, args)
			stage := ""
			if len(fromFields) >= 3 && strings.EqualFold(fromFields[1], "AS") {
				stage = fromFields[2]
			}
			if unresolved {
				finalImageIndex = -1
				incomplete = append(incomplete, fmt.Sprintf("Dockerfile FROM image contains unresolved interpolation: %s", rawImage))
				if stage != "" {
					stages[strings.ToLower(stage)] = struct{}{}
				}
				continue
			}
			if strings.EqualFold(image, "scratch") {
				finalImageIndex = -1
				if stage != "" {
					stages[strings.ToLower(stage)] = struct{}{}
				}
				continue
			}
			if _, isPriorStage := stages[strings.ToLower(image)]; isPriorStage {
				finalImageIndex = -1
				if stage != "" {
					stages[strings.ToLower(stage)] = struct{}{}
				}
				continue
			}
			dependency, ok := dockerImageDependency(image)
			if !ok {
				finalImageIndex = -1
				incomplete = append(incomplete, fmt.Sprintf("Dockerfile FROM image could not be parsed: %s", rawImage))
				continue
			}
			dependency.SourceGroup = "FROM"
			dependency.Relationship = RelationshipDirect
			dependency.Scope = ScopeBuild
			if platform != "" {
				if dependency.Attributes == nil {
					dependency.Attributes = make(map[string]string)
				}
				dependency.Attributes["platform"] = platform
			}
			if stage != "" {
				if dependency.Attributes == nil {
					dependency.Attributes = make(map[string]string)
				}
				dependency.Attributes["stage"] = stage
				stages[strings.ToLower(stage)] = struct{}{}
			}
			images = append(images, dependency)
			finalImageIndex = len(images) - 1
		case "COPY":
			match := dockerCopyFrom.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			rawSource := strings.Trim(match[1], `"'`)
			source, unresolved := resolveDockerVariables(rawSource, args)
			if unresolved {
				incomplete = append(incomplete, fmt.Sprintf("Dockerfile COPY --from contains unresolved interpolation: %s", rawSource))
				continue
			}
			if _, isStage := stages[strings.ToLower(source)]; isStage {
				continue
			}
			if _, numericStage := strconv.Atoi(source); numericStage == nil {
				continue
			}
			dependency, ok := dockerImageDependency(source)
			if !ok {
				incomplete = append(incomplete, fmt.Sprintf("Dockerfile COPY --from image could not be parsed: %s", rawSource))
				continue
			}
			dependency.SourceGroup = "COPY --from"
			dependency.Relationship = RelationshipDirect
			dependency.Scope = ScopeBuild
			copyImages = append(copyImages, dependency)
		}
	}

	if finalImageIndex >= 0 {
		images[finalImageIndex].Scope = ScopeRuntime
	}
	dependencies := make([]DependencyReference, 0, len(images)+len(copyImages))
	indexByRaw := make(map[string]int)
	for _, image := range images {
		dependencies = appendOrMergeDockerDependency(dependencies, indexByRaw, image)
	}
	for _, dependency := range copyImages {
		dependencies = appendOrMergeDockerDependency(dependencies, indexByRaw, dependency)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func dockerfileLogicalLines(path, content string) ([]string, error) {
	physical := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	logical := make([]string, 0, len(physical))
	var current strings.Builder
	continuing := false
	for _, line := range physical {
		trimmed := strings.TrimSpace(line)
		if !continuing && (trimmed == "" || strings.HasPrefix(trimmed, "#")) {
			continue
		}
		if continuing && trimmed == "" {
			continue
		}
		hasContinuation := strings.HasSuffix(strings.TrimRight(line, " \t"), "\\")
		if hasContinuation {
			line = strings.TrimSuffix(strings.TrimRight(line, " \t"), "\\")
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(strings.TrimSpace(line))
		continuing = hasContinuation
		if !hasContinuation {
			logical = append(logical, current.String())
			current.Reset()
		}
	}
	if continuing {
		return nil, fmt.Errorf("parse Dockerfile %q: unterminated line continuation", path)
	}
	return logical, nil
}

func resolveDockerVariables(value string, args map[string]string) (string, bool) {
	unresolved := false
	resolved := dockerVariable.ReplaceAllStringFunc(value, func(match string) string {
		submatch := dockerVariable.FindStringSubmatch(match)
		name := submatch[1]
		if name == "" {
			name = submatch[2]
		}
		replacement, exists := args[name]
		if !exists {
			unresolved = true
			return match
		}
		return replacement
	})
	if strings.Contains(resolved, "$") {
		unresolved = true
	}
	return resolved, unresolved
}

func appendOrMergeDockerDependency(dependencies []DependencyReference, indexByRaw map[string]int, dependency DependencyReference) []DependencyReference {
	index, exists := indexByRaw[dependency.Raw]
	if !exists {
		indexByRaw[dependency.Raw] = len(dependencies)
		return append(dependencies, dependency)
	}
	existing := &dependencies[index]
	if dependency.Scope == ScopeRuntime {
		existing.Scope = ScopeRuntime
	}
	if existing.Attributes == nil && dependency.Attributes != nil {
		existing.Attributes = make(map[string]string)
	}
	for key, value := range dependency.Attributes {
		existing.Attributes[key] = value
	}
	return dependencies
}

func dockerImageDependency(raw string) (DependencyReference, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, " \t") {
		return DependencyReference{}, false
	}
	nameAndTag := raw
	digest := ""
	if before, after, ok := strings.Cut(raw, "@"); ok {
		nameAndTag = before
		digest = after
		if digest == "" {
			return DependencyReference{}, false
		}
	}
	name := nameAndTag
	version := ""
	lastSlash := strings.LastIndex(nameAndTag, "/")
	lastColon := strings.LastIndex(nameAndTag, ":")
	if lastColon > lastSlash {
		name = nameAndTag[:lastColon]
		version = nameAndTag[lastColon+1:]
	}
	if name == "" || version == "" && strings.HasSuffix(nameAndTag, ":") {
		return DependencyReference{}, false
	}
	dependency := DependencyReference{
		PackageType: PackageType("docker"),
		Raw:         raw,
		Name:        name,
		Version:     version,
		OriginKind:  OriginRegistry,
	}
	if digest != "" {
		dependency.Attributes = map[string]string{"digest": digest}
	}
	return dependency, true
}

func (dockerComposeParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root map[string]any
	if err := yaml.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Docker Compose file %q: %w", path, err)
	}
	rawServices, exists := root["services"]
	if !exists {
		return semanticAnalyzerResult(nil, nil), nil
	}
	services, ok := asStringMap(rawServices)
	if !ok {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Docker Compose file %q: services must be a mapping", path)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	serviceNames := make([]string, 0, len(services))
	for name := range services {
		serviceNames = append(serviceNames, name)
	}
	slices.Sort(serviceNames)
	for _, serviceName := range serviceNames {
		service, ok := asStringMap(services[serviceName])
		if !ok {
			continue
		}
		rawImage, exists := service["image"]
		if !exists {
			continue
		}
		image, ok := rawImage.(string)
		if !ok || strings.TrimSpace(image) == "" {
			incomplete = append(incomplete, fmt.Sprintf("services.%s.image is not a non-empty string", serviceName))
			continue
		}
		if strings.Contains(image, "$") {
			incomplete = append(incomplete, fmt.Sprintf("services.%s.image contains unresolved interpolation: %s", serviceName, image))
			continue
		}
		dependency, ok := dockerImageDependency(image)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("services.%s.image could not be parsed: %s", serviceName, image))
			continue
		}
		dependency.SourceGroup = "services." + serviceName + ".image"
		dependency.Relationship = RelationshipDirect
		dependency.Scope = ScopeRuntime
		dependencies = append(dependencies, dependency)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}
