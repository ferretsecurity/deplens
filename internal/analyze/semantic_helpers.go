package analyze

import (
	"cmp"
	"slices"
)

const incompleteExtractionCode = "dependency-extraction-incomplete"

func semanticAnalyzerResult(dependencies []DependencyReference, incompleteMessages []string) sourceAnalyzerResult {
	if len(incompleteMessages) == 0 {
		return sourceAnalyzerResult{
			Recognized:   true,
			Analysis:     completeAnalysis(dependencies),
			Dependencies: dependencies,
		}
	}

	analysis := SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionUnsupported}
	if len(dependencies) > 0 {
		analysis.Extraction = ExtractionPartial
	}
	return sourceAnalyzerResult{
		Recognized:   true,
		Analysis:     analysis,
		Dependencies: dependencies,
		Diagnostics:  diagnosticsFromMessages(DiagnosticWarning, incompleteExtractionCode, incompleteMessages),
	}
}

func sortDependencyReferences(dependencies []DependencyReference) {
	slices.SortFunc(dependencies, func(a, b DependencyReference) int {
		if value := cmp.Compare(a.SourceGroup, b.SourceGroup); value != 0 {
			return value
		}
		if value := cmp.Compare(a.Name, b.Name); value != 0 {
			return value
		}
		if value := cmp.Compare(a.Version, b.Version); value != 0 {
			return value
		}
		return cmp.Compare(a.VersionConstraint, b.VersionConstraint)
	})
}

func appendUniqueDependency(dependencies []DependencyReference, seen map[string]struct{}, key string, dependency DependencyReference) []DependencyReference {
	if _, exists := seen[key]; exists {
		return dependencies
	}
	seen[key] = struct{}{}
	return append(dependencies, dependency)
}

func appendUniqueMessage(messages []string, seen map[string]struct{}, message string) []string {
	if _, exists := seen[message]; exists {
		return messages
	}
	seen[message] = struct{}{}
	return append(messages, message)
}
