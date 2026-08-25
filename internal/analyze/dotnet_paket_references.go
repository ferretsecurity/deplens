package analyze

import "strings"

type dotnetPaketReferencesMatcherConfig struct{}

type dotnetPaketReferencesParser struct{}

func newDotnetPaketReferencesParser(dotnetPaketReferencesMatcherConfig) (sourceAnalyzer, error) {
	return dotnetPaketReferencesParser{}, nil
}

func (dotnetPaketReferencesParser) Analyze(_ string, content []byte) (sourceAnalyzerResult, error) {
	dependencies := make([]DependencyReference, 0)
	group := "default"

	for _, line := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "//") || strings.HasPrefix(fields[0], "#") {
			continue
		}

		if len(fields) == 2 && strings.EqualFold(fields[0], "group") {
			group = fields[1]
			continue
		}
		if len(fields) != 1 || strings.EqualFold(fields[0], "file:") {
			continue
		}

		name := fields[0]
		dependencies = append(dependencies, DependencyReference{
			PackageType:  "nuget",
			Raw:          name,
			Name:         name,
			SourceGroup:  group,
			OriginKind:   OriginRegistry,
			Relationship: RelationshipDirect,
			Scope:        paketGroupScope(group),
		})
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}
