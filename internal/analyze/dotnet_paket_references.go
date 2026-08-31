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

	for _, line := range strings.Split(strings.TrimPrefix(string(content), "\ufeff"), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "//") || strings.HasPrefix(fields[0], "#") {
			continue
		}

		if len(fields) == 2 && strings.EqualFold(fields[0], "group") {
			group = fields[1]
			continue
		}

		first := fields[0]
		if strings.HasPrefix(strings.ToLower(first), "file:") || strings.EqualFold(first, "exclude") || strings.EqualFold(first, "alias") {
			continue
		}
		if strings.EqualFold(first, "nuget") {
			if len(fields) < 2 {
				continue
			}
			first = fields[1]
		}

		name := first
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
