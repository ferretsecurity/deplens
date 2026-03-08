package analyze

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

var supportedManifestTypes = []ManifestType{
	RequirementsTXT,
	UVLock,
	PackageJSON,
	YarnLock,
	PomXML,
}

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
	switch name {
	case string(RequirementsTXT):
		return RequirementsTXT, true
	case string(UVLock):
		return UVLock, true
	case string(PackageJSON):
		return PackageJSON, true
	case string(YarnLock):
		return YarnLock, true
	case string(PomXML):
		return PomXML, true
	default:
		return "", false
	}
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
