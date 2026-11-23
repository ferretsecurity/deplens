package analyze

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"go.yaml.in/yaml/v3"
)

type dockerfileParser struct{}
type dockerComposeParser struct{}

var dockerVariable = regexp.MustCompile(`\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))`)

func newDockerfileParser(dockerfileMatcherConfig) (sourceAnalyzer, error) {
	return dockerfileParser{}, nil
}

func newDockerComposeParser(dockerComposeMatcherConfig) (sourceAnalyzer, error) {
	return dockerComposeParser{}, nil
}

func (dockerfileParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	tree, err := parser.Parse(bytes.NewReader(content))
	if err != nil {
		if err.Error() == "file with no instructions" && len(lastDockerfileLine(content)) == 0 {
			return semanticAnalyzerResult(nil, nil), nil
		}
		return sourceAnalyzerResult{}, fmt.Errorf("parse Dockerfile %q: %w", path, err)
	}
	if dockerfileHasTrailingContinuation(tree, content) {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Dockerfile %q: unterminated line continuation", path)
	}

	args := make(map[string]string)
	stages := make(map[string]struct{})
	images := make([]DependencyReference, 0)
	copyImages := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	finalImageIndex := -1

	for _, node := range tree.AST.Children {
		var imageFlag string
		switch strings.ToUpper(node.Value) {
		case "FROM":
			imageFlag = "--platform="
		case "COPY":
			imageFlag = "--from="
		case "ARG":
		default:
			continue
		}
		// Frontends can add flags, such as FROM --reset in dockerfile:1-labs.
		// Only pass flags used for image extraction to the typed parser.
		imageNode := *node
		imageNode.Flags = nil
		for _, flag := range node.Flags {
			if imageFlag != "" && strings.HasPrefix(flag, imageFlag) {
				imageNode.Flags = append(imageNode.Flags, flag)
			}
		}
		instruction, err := instructions.ParseInstruction(&imageNode)
		if err != nil {
			return sourceAnalyzerResult{}, fmt.Errorf("parse Dockerfile %q line %d: %w", path, node.StartLine, err)
		}
		switch cmd := instruction.(type) {
		case *instructions.ArgCommand:
			for _, arg := range cmd.Args {
				if arg.Value != nil {
					args[arg.Key] = *arg.Value
				}
			}
		case *instructions.Stage:
			rawImage, platform, stage := cmd.BaseName, cmd.Platform, ""
			if cmd.Name != "" {
				stage = node.Next.Next.Next.Value
			}
			image, unresolved := resolveDockerVariables(rawImage, args)
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
		case *instructions.CopyCommand:
			rawSource := strings.Trim(cmd.From, `"'`)
			if rawSource == "" {
				continue
			}
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

// BuildKit accepts an unfinished continuation at EOF. Keep reporting it as a
// syntax error, while leaving heredoc contents to BuildKit's parser.
func dockerfileHasTrailingContinuation(tree *parser.Result, content []byte) bool {
	last := tree.AST.Children[len(tree.AST.Children)-1]
	if len(last.Heredocs) > 0 {
		return false
	}
	line := lastDockerfileLine(content)
	escape := byte(tree.EscapeToken)
	return len(line) > 0 && line[len(line)-1] == escape && (len(line) == 1 || line[len(line)-2] != escape)
}

func lastDockerfileLine(content []byte) []byte {
	content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))
	for len(content) > 0 {
		start := bytes.LastIndexByte(content, '\n')
		line := bytes.TrimSpace(content[start+1:])
		if len(line) > 0 && line[0] != '#' {
			return line
		}
		content = content[:max(start, 0)]
	}
	return nil
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
