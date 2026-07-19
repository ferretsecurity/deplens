package analyze

// analyzeSourceParts keeps table-driven analyzer tests compact while they
// assert the public result object's individual values.
func analyzeSourceParts(ruleset Ruleset, path, name string) (DetectorID, []DependencyReference, *bool, []string, bool, error) {
	result, ok, err := ruleset.AnalyzeDependencySource(path, name)
	return sourceParts(result, ok, err)
}

func analyzeSourcePartsAtRelativePath(ruleset Ruleset, path, name, relPath string) (DetectorID, []DependencyReference, *bool, []string, bool, error) {
	result, ok, err := ruleset.AnalyzeDependencySourceAtRelativePath(path, name, relPath)
	return sourceParts(result, ok, err)
}

func sourceParts(result DependencySourceResult, ok bool, err error) (DetectorID, []DependencyReference, *bool, []string, bool, error) {
	var present *bool
	switch result.Analysis.Presence {
	case PresencePresent:
		present = boolPtr(true)
	case PresenceAbsent:
		present = boolPtr(false)
	}
	var messages []string
	for _, diagnostic := range result.Diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	return result.Detector, result.Dependencies, present, messages, ok, err
}

func boolPtr(value bool) *bool {
	return &value
}
