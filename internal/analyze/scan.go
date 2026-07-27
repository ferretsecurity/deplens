package analyze

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type DetectorID string
type SourceForm string
type SourceRole string
type PackageType string
type DependencyPresence string
type ExtractionState string
type Relationship string
type DependencyScope string
type OriginKind string
type DiagnosticSeverity string
type FindingSeverity string
type CheckRunStatus string

const (
	PresenceUnknown DependencyPresence = "unknown"
	PresenceAbsent  DependencyPresence = "absent"
	PresencePresent DependencyPresence = "present"

	ExtractionUnsupported ExtractionState = "unsupported"
	ExtractionComplete    ExtractionState = "complete"
	ExtractionPartial     ExtractionState = "partial"
	ExtractionFailed      ExtractionState = "failed"

	DiagnosticWarning DiagnosticSeverity = "warning"
	DiagnosticError   DiagnosticSeverity = "error"

	SeverityInfo     FindingSeverity = "info"
	SeverityLow      FindingSeverity = "low"
	SeverityMedium   FindingSeverity = "medium"
	SeverityHigh     FindingSeverity = "high"
	SeverityCritical FindingSeverity = "critical"

	CheckCompleted CheckRunStatus = "completed"
	CheckSkipped   CheckRunStatus = "skipped"
	CheckFailed    CheckRunStatus = "failed"

	RelationshipDirect       Relationship = "direct"
	RelationshipTransitive   Relationship = "transitive"
	RelationshipInconclusive Relationship = "inconclusive"

	ScopeRuntime     DependencyScope = "runtime"
	ScopeDevelopment DependencyScope = "development"
	ScopeTest        DependencyScope = "test"
	ScopeBuild       DependencyScope = "build"
	ScopeOptional    DependencyScope = "optional"

	OriginRegistry  OriginKind = "registry"
	OriginGit       OriginKind = "git"
	OriginPath      OriginKind = "path"
	OriginURL       OriginKind = "url"
	OriginWorkspace OriginKind = "workspace"
)

type DependencyReference struct {
	PackageType       PackageType       `json:"package_type,omitempty"`
	Raw               string            `json:"raw"`
	Name              string            `json:"name,omitempty"`
	Version           string            `json:"version,omitempty"`
	VersionConstraint string            `json:"version_constraint,omitempty"`
	VERS              string            `json:"vers,omitempty"`
	SourceGroup       string            `json:"source_group,omitempty"`
	OriginKind        OriginKind        `json:"origin_kind,omitempty"`
	Relationship      Relationship      `json:"relationship,omitempty"`
	Scope             DependencyScope   `json:"scope,omitempty"`
	Attributes        map[string]string `json:"attributes,omitempty"`
}

type SourceAnalysis struct {
	Presence   DependencyPresence `json:"presence"`
	Extraction ExtractionState    `json:"extraction"`
}

type Diagnostic struct {
	Severity DiagnosticSeverity `json:"severity"`
	Code     string             `json:"code"`
	Message  string             `json:"message"`
}

type DependencySourceResult struct {
	Detector     DetectorID            `json:"detector"`
	Path         string                `json:"path"`
	Form         SourceForm            `json:"form"`
	Roles        []SourceRole          `json:"roles"`
	Analysis     SourceAnalysis        `json:"analysis"`
	Dependencies []DependencyReference `json:"dependencies,omitempty"`
	Diagnostics  []Diagnostic          `json:"diagnostics,omitempty"`
	content      []byte
	facts        []sourceFact
}

type FindingSubject struct {
	ProjectRoot string `json:"project_root"`
}

type FindingLocation struct {
	Path string `json:"path"`
}

type Finding struct {
	CheckID     CheckID           `json:"check_id"`
	Severity    FindingSeverity   `json:"severity"`
	Summary     string            `json:"summary"`
	Subject     FindingSubject    `json:"subject"`
	Locations   []FindingLocation `json:"locations"`
	Evidence    map[string]string `json:"evidence,omitempty"`
	Remediation string            `json:"remediation"`
	Fingerprint string            `json:"fingerprint"`
}

type CheckRun struct {
	CheckID    CheckID        `json:"check_id"`
	Subject    FindingSubject `json:"subject"`
	Status     CheckRunStatus `json:"status"`
	ReasonCode string         `json:"reason_code,omitempty"`
	Detail     string         `json:"detail,omitempty"`
}

type ScanResult struct {
	SchemaVersion int                      `json:"schema_version"`
	Root          string                   `json:"root"`
	Sources       []DependencySourceResult `json:"sources"`
	CheckRuns     []CheckRun               `json:"check_runs"`
	Findings      []Finding                `json:"findings"`
}

func Scan(root string, ignoreDirs []string, ruleset Ruleset) (ScanResult, error) {
	cleanRoot := filepath.Clean(root)
	info, err := os.Stat(cleanRoot)
	if err != nil {
		return ScanResult{}, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return ScanResult{}, fmt.Errorf("path is not a directory: %s", cleanRoot)
	}

	absRoot, err := filepath.Abs(cleanRoot)
	if err != nil {
		return ScanResult{}, fmt.Errorf("resolve root: %w", err)
	}

	ignoreSet := make(map[string]struct{}, len(ignoreDirs))
	for _, dir := range ignoreDirs {
		if dir != "" {
			ignoreSet[dir] = struct{}{}
		}
	}

	result := ScanResult{SchemaVersion: 1, Root: absRoot, Sources: make([]DependencySourceResult, 0), CheckRuns: make([]CheckRun, 0), Findings: make([]Finding, 0)}
	discoveredPaths := make(map[string]struct{})
	policyInputs := make([]policyInput, 0)
	collectPyprojects := ruleset.hasEvaluator("uv-lockfile-missing")
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != absRoot {
				if _, ignored := ignoreSet[d.Name()]; ignored {
					return filepath.SkipDir
				}
			}
			return nil
		}

		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return fmt.Errorf("relative path for %s: %w", path, err)
		}
		relPath = normalizeRelativePath(relPath)
		discoveredPaths[relPath] = struct{}{}

		var source DependencySourceResult
		var ok bool
		if collectPyprojects && d.Name() == "pyproject.toml" {
			content, readErr := os.ReadFile(path)
			policyInputs = append(policyInputs, policyInput{path: relPath, content: content, readError: readErr})
			if readErr == nil {
				source, ok, _, err = ruleset.analyzeDependencySourceWithContent(path, d.Name(), relPath, content, true)
			} else {
				source, ok, _, err = ruleset.analyzeDependencySource(path, d.Name(), relPath)
			}
		} else {
			source, ok, _, err = ruleset.analyzeDependencySource(path, d.Name(), relPath)
		}
		if err != nil {
			return err
		}
		if ok {
			result.Sources = append(result.Sources, source)
		}
		return nil
	})
	if err != nil {
		return ScanResult{}, fmt.Errorf("walk root: %w", err)
	}

	slices.SortFunc(result.Sources, func(a, b DependencySourceResult) int {
		if a.Path == b.Path {
			return compareDetectorID(a.Detector, b.Detector)
		}
		if a.Path < b.Path {
			return -1
		}
		return 1
	})

	result.CheckRuns, result.Findings = evaluateChecks(result.Sources, policyInputs, discoveredPaths, ruleset.checks)
	for index := range result.Sources {
		result.Sources[index].content = nil
		result.Sources[index].facts = nil
	}

	return result, nil
}

func validFindingSeverity(severity FindingSeverity) bool {
	switch severity {
	case SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return true
	default:
		return false
	}
}

func compareDetectorID(a, b DetectorID) int {
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

func normalizeRelativePath(relPath string) string {
	return strings.ReplaceAll(filepath.ToSlash(relPath), "\\", "/")
}

func dependenciesFromStrings(values []string) []DependencyReference {
	if len(values) == 0 {
		return nil
	}

	dependencies := make([]DependencyReference, 0, len(values))
	for _, value := range values {
		dependencies = append(dependencies, DependencyReference{Raw: value})
	}
	return dependencies
}

func completeAnalysis(dependencies []DependencyReference) SourceAnalysis {
	if len(dependencies) == 0 {
		return SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}
	}
	return SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}
}

func presenceAnalysis(present bool) SourceAnalysis {
	if present {
		return SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionUnsupported}
	}
	return SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionUnsupported}
}

func identifiedAnalysis() SourceAnalysis {
	return SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}
}

func failedAnalysis() SourceAnalysis {
	return SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionFailed}
}

func diagnosticsFromMessages(severity DiagnosticSeverity, code string, messages []string) []Diagnostic {
	if len(messages) == 0 {
		return nil
	}
	diagnostics := make([]Diagnostic, 0, len(messages))
	for _, message := range messages {
		diagnostics = append(diagnostics, Diagnostic{Severity: severity, Code: code, Message: message})
	}
	return diagnostics
}
