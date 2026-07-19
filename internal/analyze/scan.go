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

	RelationshipDirect       Relationship = "direct"
	RelationshipTransitive   Relationship = "transitive"
	RelationshipInconclusive Relationship = "inconclusive"

	ScopeRuntime     DependencyScope = "runtime"
	ScopeDevelopment DependencyScope = "development"
	ScopeTest        DependencyScope = "test"
	ScopeBuild       DependencyScope = "build"
	ScopeOptional    DependencyScope = "optional"

	OriginRegistry OriginKind = "registry"
	OriginGit      OriginKind = "git"
	OriginPath     OriginKind = "path"
	OriginURL      OriginKind = "url"
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
}

type ScanResult struct {
	Root    string                   `json:"root"`
	Sources []DependencySourceResult `json:"sources"`
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

	result := ScanResult{Root: absRoot, Sources: make([]DependencySourceResult, 0)}
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

		source, ok, err := ruleset.AnalyzeDependencySourceAtRelativePath(path, d.Name(), relPath)
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

	return result, nil
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
