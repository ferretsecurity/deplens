package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/ferretsecurity/deplens/internal/analyze"
)

func displayDependency(d analyze.DependencyReference) string {
	if d.Name != "" && d.Version != "" {
		return d.Name + "@" + d.Version
	}
	if d.Name != "" && d.VersionConstraint != "" {
		return d.Name + d.VersionConstraint
	}
	if d.Name != "" {
		return d.Name
	}
	return d.Raw
}

type HumanOptions struct {
	ShowWithoutDependencies bool
}

func Human(result analyze.ScanResult, opts HumanOptions) string {
	if len(result.Sources) == 0 && len(result.Findings) == 0 {
		return fmt.Sprintf("Root: %s\nNo dependency sources found.\n", result.Root)
	}

	sources := slices.Clone(result.Sources)
	slices.SortFunc(sources, func(a, b analyze.DependencySourceResult) int {
		if a.Path == b.Path {
			switch {
			case a.Detector < b.Detector:
				return -1
			case a.Detector > b.Detector:
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

	visibleSources := filterVisibleSources(sources, opts)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Root: %s\n\n", result.Root))
	b.WriteString(fmt.Sprintf("Found %d dependency %s:\n", len(sources), pluralize(len(sources), "source", "sources")))
	for _, source := range visibleSources {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("%s %s\n", source.Path, sourceStatusLabel(source)))
		b.WriteString(renderDependencies(source.Dependencies))
		b.WriteString(renderDiagnostics(source.Diagnostics))
	}
	b.WriteString(renderFindings(result.Findings))
	return b.String()
}

func JSON(result analyze.ScanResult) ([]byte, error) {
	if result.SchemaVersion == 0 {
		result.SchemaVersion = 1
	}
	if result.Sources == nil {
		result.Sources = make([]analyze.DependencySourceResult, 0)
	}
	if result.CheckRuns == nil {
		result.CheckRuns = make([]analyze.CheckRun, 0)
	}
	if result.Findings == nil {
		result.Findings = make([]analyze.Finding, 0)
	}
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func renderFindings(findings []analyze.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	ordered := slices.Clone(findings)
	slices.SortFunc(ordered, func(a, b analyze.Finding) int {
		if a.Subject.ProjectRoot != b.Subject.ProjectRoot {
			return strings.Compare(a.Subject.ProjectRoot, b.Subject.ProjectRoot)
		}
		return strings.Compare(string(a.CheckID), string(b.CheckID))
	})
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\nFound %d policy %s:\n", len(ordered), pluralize(len(ordered), "finding", "findings")))
	for _, finding := range ordered {
		location := finding.Subject.ProjectRoot
		if len(finding.Locations) > 0 {
			location = finding.Locations[0].Path
		}
		b.WriteString(fmt.Sprintf("\n%s [%s] %s\n", location, finding.Severity, finding.Summary))
		b.WriteString(fmt.Sprintf("  check: %s\n", finding.CheckID))
		if expected := finding.Evidence["expected_lockfile"]; expected != "" {
			b.WriteString(fmt.Sprintf("  expected: %s\n", expected))
		}
		b.WriteString(fmt.Sprintf("  remediation: %s\n", finding.Remediation))
	}
	return b.String()
}

func sourceStatusLabel(source analyze.DependencySourceResult) string {
	form := string(source.Form)
	if source.Analysis.Presence == analyze.PresenceAbsent {
		return fmt.Sprintf("[%s · no dependency references]", form)
	}
	switch source.Analysis {
	case analyze.SourceAnalysis{Presence: analyze.PresencePresent, Extraction: analyze.ExtractionComplete}:
		return fmt.Sprintf("[%s · %d %s]", form, len(source.Dependencies), pluralize(len(source.Dependencies), "dependency", "dependencies"))
	case analyze.SourceAnalysis{Presence: analyze.PresencePresent, Extraction: analyze.ExtractionPartial}:
		return fmt.Sprintf("[%s · %d %s · partial]", form, len(source.Dependencies), pluralize(len(source.Dependencies), "dependency", "dependencies"))
	case analyze.SourceAnalysis{Presence: analyze.PresencePresent, Extraction: analyze.ExtractionUnsupported}:
		return fmt.Sprintf("[%s · references present, not extracted]", form)
	case analyze.SourceAnalysis{Presence: analyze.PresenceUnknown, Extraction: analyze.ExtractionFailed}:
		return fmt.Sprintf("[%s · analysis failed]", form)
	default:
		return fmt.Sprintf("[%s · identified only]", form)
	}
}

func renderDependencies(dependencies []analyze.DependencyReference) string {
	if len(dependencies) == 0 {
		return ""
	}

	allUngrouped := true
	for _, dependency := range dependencies {
		if dependency.SourceGroup != "" {
			allUngrouped = false
			break
		}
	}
	if allUngrouped {
		var b strings.Builder
		for _, dependency := range dependencies {
			b.WriteString(fmt.Sprintf("  - %s\n", displayDependency(dependency)))
		}
		return b.String()
	}

	order := make([]string, 0, len(dependencies))
	grouped := make(map[string][]string, len(dependencies))
	for _, dependency := range dependencies {
		groupName := dependency.SourceGroup
		if groupName == "" {
			groupName = "[default group]"
		}
		if _, exists := grouped[groupName]; !exists {
			order = append(order, groupName)
		}
		grouped[groupName] = append(grouped[groupName], displayDependency(dependency))
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

func renderDiagnostics(diagnostics []analyze.Diagnostic) string {
	var b strings.Builder
	for _, diagnostic := range diagnostics {
		b.WriteString(fmt.Sprintf("  %s [%s]: %s\n", diagnostic.Severity, diagnostic.Code, diagnostic.Message))
	}
	return b.String()
}

func filterVisibleSources(sources []analyze.DependencySourceResult, opts HumanOptions) []analyze.DependencySourceResult {
	if opts.ShowWithoutDependencies {
		return sources
	}
	filtered := make([]analyze.DependencySourceResult, 0, len(sources))
	for _, source := range sources {
		if source.Analysis.Presence != analyze.PresenceAbsent {
			filtered = append(filtered, source)
		}
	}
	return filtered
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
