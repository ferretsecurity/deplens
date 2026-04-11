package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/ferretsecurity/deplens/internal/analyze"
)

type HumanOptions struct {
	ShowEmpty bool
}

func Human(result analyze.ScanResult, supportedTypes []analyze.ManifestType, opts HumanOptions) string {
	if len(result.Manifests) == 0 {
		return fmt.Sprintf("Root: %s\nNo manifests found.\n", result.Root)
	}

	_ = supportedTypes

	manifests := slices.Clone(result.Manifests)
	slices.SortFunc(manifests, func(a, b analyze.ManifestMatch) int {
		if a.Path == b.Path {
			switch {
			case a.Type < b.Type:
				return -1
			case a.Type > b.Type:
				return 1
			default:
				return 0
			}
		}
		if a.Path < b.Path {
			return -1
		}
		return 1
	})

	summary := summarizeManifests(manifests)
	visibleManifests := filterVisibleManifests(manifests, opts)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Root: %s\n", result.Root))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Found %d %s:\n", len(manifests), pluralize(len(manifests), "manifest", "manifests")))
	for _, line := range summary.lines() {
		b.WriteString(fmt.Sprintf("- %s\n", line))
	}
	for _, manifest := range visibleManifests {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("%s %s\n", manifest.Path, manifestStatusLabel(manifest)))
		b.WriteString(renderDependencies(manifest.Dependencies))
		b.WriteString(renderWarnings(manifest.Warnings))
	}
	return b.String()
}

func JSON(result analyze.ScanResult) ([]byte, error) {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

type manifestSummary struct {
	extracted           int
	confirmedEmpty      int
	presentNotExtracted int
	unknown             int
}

func summarizeManifests(manifests []analyze.ManifestMatch) manifestSummary {
	var summary manifestSummary
	for _, manifest := range manifests {
		switch manifestState(manifest) {
		case "extracted":
			summary.extracted++
		case "empty":
			summary.confirmedEmpty++
		case "present-not-extracted":
			summary.presentNotExtracted++
		default:
			summary.unknown++
		}
	}
	return summary
}

func (s manifestSummary) lines() []string {
	lines := make([]string, 0, 4)
	if s.extracted > 0 {
		lines = append(lines, fmt.Sprintf("%d with extracted dependencies", s.extracted))
	}
	if s.confirmedEmpty > 0 {
		lines = append(lines, fmt.Sprintf("%d confirmed empty", s.confirmedEmpty))
	}
	if s.presentNotExtracted > 0 {
		lines = append(lines, fmt.Sprintf("%d with dependencies present, not extracted", s.presentNotExtracted))
	}
	if s.unknown > 0 {
		lines = append(lines, fmt.Sprintf("%d with dependency status unknown", s.unknown))
	}
	return lines
}

func manifestStatusLabel(manifest analyze.ManifestMatch) string {
	switch manifestState(manifest) {
	case "extracted":
		return fmt.Sprintf("[%d %s]", len(manifest.Dependencies), pluralize(len(manifest.Dependencies), "dep", "deps"))
	case "empty":
		return "[no dependencies]"
	case "present-not-extracted":
		return "[dependencies present, not extracted]"
	default:
		return "[matched]"
	}
}

func manifestState(manifest analyze.ManifestMatch) string {
	if len(manifest.Dependencies) > 0 {
		return "extracted"
	}
	if manifest.HasDependencies == nil {
		return "unknown"
	}
	if *manifest.HasDependencies {
		return "present-not-extracted"
	}
	return "empty"
}

func renderDependencies(dependencies []analyze.Dependency) string {
	if len(dependencies) == 0 {
		return ""
	}

	allUnsectioned := true
	for _, dependency := range dependencies {
		if dependency.Section != "" {
			allUnsectioned = false
			break
		}
	}
	if allUnsectioned {
		var b strings.Builder
		for _, dependency := range dependencies {
			b.WriteString(fmt.Sprintf("  - %s\n", dependency.Name))
		}
		return b.String()
	}

	order := make([]string, 0, len(dependencies))
	grouped := make(map[string][]string, len(dependencies))
	for _, dependency := range dependencies {
		groupName := dependency.Section
		if groupName == "" {
			groupName = "[default group]"
		}
		if _, exists := grouped[groupName]; !exists {
			order = append(order, groupName)
		}
		grouped[groupName] = append(grouped[groupName], dependency.Name)
	}

	var b strings.Builder
	for _, groupName := range order {
		if groupName == "[default group]" {
			b.WriteString("  [default group]\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s:\n", groupName))
		}
		for _, dependency := range grouped[groupName] {
			b.WriteString(fmt.Sprintf("    - %s\n", dependency))
		}
	}
	return b.String()
}

func renderWarnings(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}

	var b strings.Builder
	for _, warning := range warnings {
		b.WriteString(fmt.Sprintf("  warning: %s\n", warning))
	}
	return b.String()
}

func filterVisibleManifests(manifests []analyze.ManifestMatch, opts HumanOptions) []analyze.ManifestMatch {
	if opts.ShowEmpty {
		return manifests
	}

	filtered := make([]analyze.ManifestMatch, 0, len(manifests))
	for _, manifest := range manifests {
		if manifestState(manifest) == "empty" {
			continue
		}
		filtered = append(filtered, manifest)
	}
	return filtered
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
