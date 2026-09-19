package analyze

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// ComposeRules loads a replacement base, or the embedded defaults when basePath
// is empty, then appends extension documents in occurrence order.
func ComposeRules(basePath string, extensionPaths []string) (Ruleset, error) {
	origin := basePath
	var result Ruleset
	var err error
	if basePath == "" {
		origin = "embedded default rules"
		result, err = LoadDefaultRules()
	} else {
		result, err = LoadRulesFile(basePath)
	}
	if err != nil {
		return Ruleset{}, err
	}
	detectorOrigins := make(map[DetectorID]string)
	checkOrigins := make(map[CheckID]string)
	for _, d := range result.detectors {
		detectorOrigins[d.ID] = origin
	}
	for _, c := range result.checks {
		checkOrigins[c.ID] = origin
	}
	for _, path := range extensionPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			return Ruleset{}, fmt.Errorf("read rules extension %q: %w", path, err)
		}
		extension, err := loadRulesDocument(path, data, true)
		if err != nil {
			return Ruleset{}, err
		}
		for _, d := range extension.detectors {
			if previous, exists := detectorOrigins[d.ID]; exists {
				return Ruleset{}, fmt.Errorf("duplicate detector ID %q in %q and %q", d.ID, previous, path)
			}
			detectorOrigins[d.ID] = path
		}
		for _, c := range extension.checks {
			if previous, exists := checkOrigins[c.ID]; exists {
				return Ruleset{}, fmt.Errorf("duplicate check ID %q in %q and %q", c.ID, previous, path)
			}
			checkOrigins[c.ID] = path
		}
		result.detectors = append(result.detectors, extension.detectors...)
		result.checks = append(result.checks, extension.checks...)
	}
	result.detectorIDs = detectorIDsFromDetectors(result.detectors)
	slices.SortFunc(result.checks, func(a, b check) int { return strings.Compare(string(a.ID), string(b.ID)) })
	return result, nil
}
