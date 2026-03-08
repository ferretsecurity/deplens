package analyze

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

type ManifestMatch struct {
	Type ManifestType `json:"type"`
	Path string       `json:"path"`
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

		manifestType, ok, err := ruleset.DetectManifestFile(path, d.Name())
		if err != nil {
			return err
		}
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

func compareManifestType(a, b ManifestType) int {
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}
