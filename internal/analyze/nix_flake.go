package analyze

import (
	"net/url"
	"strings"
)

type nixFlakeMatcherConfig struct{}

type nixFlakeParser struct{}

func newNixFlakeParser(nixFlakeMatcherConfig) (sourceAnalyzer, error) {
	return nixFlakeParser{}, nil
}

func (nixFlakeParser) Analyze(_ string, content []byte) (sourceAnalyzerResult, error) {
	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})
	for _, source := range nixFlakeInputURLs(lexNix(content)) {
		dependency, ok := nixFlakeURLDependency(source)
		if !ok {
			continue
		}
		dependencies = appendUniqueDependency(dependencies, seen, dependency.Raw, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func nixFlakeInputURLs(tokens []nixToken) []string {
	urls := make([]string, 0)
	for index := 0; index+2 < len(tokens); index++ {
		if tokens[index].kind == nixTokenIdentifier && tokens[index].value == "inputs" {
			if tokens[index+1].kind == nixTokenEquals && tokens[index+2].kind == nixTokenOpenBrace {
				depth := 1
				for index += 3; index < len(tokens) && depth > 0; index++ {
					switch tokens[index].kind {
					case nixTokenOpenBrace:
						depth++
					case nixTokenCloseBrace:
						depth--
					case nixTokenIdentifier:
						if tokens[index].value == "url" && index+2 < len(tokens) &&
							tokens[index+1].kind == nixTokenEquals && tokens[index+2].kind == nixTokenString {
							urls = append(urls, tokens[index+2].value)
						}
					}
				}
				continue
			}

			for pathIndex := index + 1; pathIndex+3 < len(tokens); pathIndex += 2 {
				if tokens[pathIndex].kind != nixTokenDot || tokens[pathIndex+1].kind != nixTokenIdentifier {
					break
				}
				if tokens[pathIndex+1].value == "url" && tokens[pathIndex+2].kind == nixTokenEquals && tokens[pathIndex+3].kind == nixTokenString {
					urls = append(urls, tokens[pathIndex+3].value)
					break
				}
			}
		}
	}
	return urls
}

func nixFlakeURLDependency(source string) (DependencyReference, bool) {
	source = strings.TrimSpace(source)
	if source == "" || strings.HasPrefix(source, "path:") || strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		return DependencyReference{}, false
	}

	if value, ok := strings.CutPrefix(source, "github:"); ok {
		return nixFlakeGitHubDependency(source, value, "")
	}

	parsedSource := strings.TrimPrefix(source, "git+")
	parsed, err := url.Parse(parsedSource)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return DependencyReference{}, false
	}
	if strings.EqualFold(parsed.Host, "github.com") {
		return nixFlakeGitHubDependency(source, strings.Trim(parsed.Path, "/"), parsed.Query().Get("ref"))
	}

	return DependencyReference{
		PackageType:  "generic",
		Raw:          source,
		Name:         source,
		SourceGroup:  "inputs",
		OriginKind:   OriginURL,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
		Attributes:   map[string]string{"source_url": parsedSource},
	}, true
}

func nixFlakeGitHubDependency(raw, path, queryRef string) (DependencyReference, bool) {
	parts := strings.SplitN(strings.Trim(path, "/"), "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return DependencyReference{}, false
	}

	name := parts[0] + "/" + parts[1]
	ref := queryRef
	if ref == "" && len(parts) == 3 {
		ref = parts[2]
	}
	attributes := map[string]string{"source_url": "https://github.com/" + name}
	if ref != "" {
		attributes["source_ref"] = ref
	}

	return DependencyReference{
		PackageType:       "github",
		Raw:               raw,
		Name:              name,
		VersionConstraint: ref,
		SourceGroup:       "inputs",
		OriginKind:        OriginGit,
		Relationship:      RelationshipDirect,
		Scope:             ScopeRuntime,
		Attributes:        attributes,
	}, true
}
