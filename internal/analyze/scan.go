package analyze

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
)

type ManifestType string

const (
	RequirementsTXT ManifestType = "requirements.txt"
	UVLock          ManifestType = "uv.lock"
	PackageJSON     ManifestType = "package.json"
	YarnLock        ManifestType = "yarn.lock"
	PomXML          ManifestType = "pom.xml"
)

type Rule struct {
	Name     string
	Patterns []manifestPattern
}

type manifestPattern struct {
	Type   ManifestType
	Regexp *regexp.Regexp
}

var requirementsPattern = regexp.MustCompile(`(^|.*[^A-Za-z])requirements([^A-Za-z].*)?\.txt$`)

var rules = []Rule{
	{
		Name: "python",
		Patterns: []manifestPattern{
			{Type: RequirementsTXT, Regexp: requirementsPattern},
			{Type: UVLock, Regexp: regexp.MustCompile(`^uv\.lock$`)},
		},
	},
	{
		Name: "js/ts",
		Patterns: []manifestPattern{
			{Type: PackageJSON, Regexp: regexp.MustCompile(`^package\.json$`)},
			{Type: YarnLock, Regexp: regexp.MustCompile(`^yarn\.lock$`)},
		},
	},
	{
		Name: "java",
		Patterns: []manifestPattern{
			{Type: PomXML, Regexp: regexp.MustCompile(`^pom\.xml$`)},
		},
	},
}

var supportedManifestTypes = supportedTypesFromRules(rules)

type ManifestMatch struct {
	Type ManifestType `json:"type"`
	Path string       `json:"path"`
}

type ScanResult struct {
	Root      string          `json:"root"`
	Manifests []ManifestMatch `json:"manifests"`
}

func SupportedManifestTypes() []ManifestType {
	return append([]ManifestType(nil), supportedManifestTypes...)
}

func Scan(root string, ignoreDirs []string) (ScanResult, error) {
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

		manifestType, ok := detectManifest(d.Name())
		if !ok {
			return nil
		}

		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return fmt.Errorf("relative path for %s: %w", path, err)
		}

		result.Manifests = append(result.Manifests, ManifestMatch{
			Type: manifestType,
			Path: filepath.ToSlash(relPath),
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

func detectManifest(name string) (ManifestType, bool) {
	for _, rule := range rules {
		for _, pattern := range rule.Patterns {
			if pattern.Regexp.MatchString(name) {
				return pattern.Type, true
			}
		}
	}
	return "", false
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

func supportedTypesFromRules(rules []Rule) []ManifestType {
	types := make([]ManifestType, 0)
	seen := make(map[ManifestType]struct{})
	for _, rule := range rules {
		for _, pattern := range rule.Patterns {
			if _, ok := seen[pattern.Type]; ok {
				continue
			}
			seen[pattern.Type] = struct{}{}
			types = append(types, pattern.Type)
		}
	}
	return types
}
