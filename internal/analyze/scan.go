package analyze

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type ManifestMatch struct {
	Type            ManifestType `json:"type"`
	Path            string       `json:"path"`
	Dependencies    []Dependency `json:"dependencies,omitempty"`
	HasDependencies *bool        `json:"has_dependencies"`
	Warnings        []string     `json:"warnings,omitempty"`
}

type ScanResult struct {
	Root      string          `json:"root"`
	Manifests []ManifestMatch `json:"manifests"`
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
		if dir == "" {
			continue
		}
		ignoreSet[dir] = struct{}{}
	}

	result := ScanResult{Root: absRoot}
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

		manifestType, dependencies, hasDependencies, warnings, ok, err := ruleset.DetectManifestFileAtRelativePath(path, d.Name(), relPath)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		result.Manifests = append(result.Manifests, ManifestMatch{
			Type:            manifestType,
			Path:            relPath,
			Dependencies:    dependencies,
			HasDependencies: hasDependencies,
			Warnings:        warnings,
		})
		return nil
	})
	if err != nil {
		return ScanResult{}, fmt.Errorf("walk root: %w", err)
	}

	slices.SortFunc(result.Manifests, func(a, b ManifestMatch) int {
		if a.Path == b.Path {
			return compareManifestType(a.Type, b.Type)
		}
		if a.Path < b.Path {
			return -1
		}
		return 1
	})

	return result, nil
}

func compareManifestType(a, b ManifestType) int {
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

type Dependency struct {
	Raw        string            `json:"raw"`
	Name       string            `json:"name,omitempty"`
	Version    string            `json:"version,omitempty"`
	Constraint string            `json:"constraint,omitempty"`
	Section    string            `json:"section,omitempty"`
	Source     string            `json:"source,omitempty"`
	Extras     map[string]string `json:"extras,omitempty"`
}

func dependenciesFromStrings(values []string) []Dependency {
	if len(values) == 0 {
		return nil
	}

	dependencies := make([]Dependency, 0, len(values))
	for _, value := range values {
		dependencies = append(dependencies, Dependency{Raw: value})
	}
	return dependencies
}
