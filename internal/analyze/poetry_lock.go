package analyze

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type poetryLockParser struct{}

type poetryLockFile struct {
	Metadata *poetryLockMetadata `toml:"metadata"`
	Packages []poetryLockPackage `toml:"package"`
}

type poetryLockMetadata struct {
	LockVersion    string `toml:"lock-version"`
	PythonVersions string `toml:"python-versions"`
	ContentHash    string `toml:"content-hash"`
}

type poetryLockPackage struct {
	Name    string            `toml:"name"`
	Version string            `toml:"version"`
	Source  *poetryLockSource `toml:"source"`
}

type poetryLockSource struct {
	Type      string `toml:"type"`
	URL       string `toml:"url"`
	Reference string `toml:"reference"`
}

func newPoetryLockParser(raw poetryLockMatcherConfig) (manifestParser, error) {
	return poetryLockParser{}, nil
}

func (p poetryLockParser) Match(path string, content []byte) (manifestParserResult, error) {
	var file poetryLockFile
	if err := toml.Unmarshal(content, &file); err != nil {
		return manifestParserResult{}, fmt.Errorf("parse toml file %q: %w", path, err)
	}

	if len(file.Packages) == 0 && !hasPoetryLockMetadata(file.Metadata) {
		return manifestParserResult{}, nil
	}

	dependencies := make([]Dependency, 0, len(file.Packages))
	seen := make(map[string]struct{}, len(file.Packages))
	for _, pkg := range file.Packages {
		if pkg.Name == "" {
			continue
		}
		if pkg.Source != nil && strings.TrimSpace(pkg.Source.Type) == "git" {
			dep := Dependency{
				Raw:    pkg.Name,
				Name:   pkg.Name,
				Source: "git",
			}
			if pkg.Source.URL != "" || pkg.Source.Reference != "" {
				dep.Extras = make(map[string]string)
				if u := strings.TrimSpace(pkg.Source.URL); u != "" {
					dep.Extras["source_url"] = u
				}
				if r := strings.TrimSpace(pkg.Source.Reference); r != "" {
					dep.Extras["source_ref"] = r
				}
			}
			key := "git:" + pkg.Name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			dependencies = append(dependencies, dep)
			continue
		}
		if shouldIgnorePoetryLockPackage(pkg) {
			continue
		}
		if pkg.Version == "" {
			continue
		}
		raw := pkg.Name + "==" + pkg.Version
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		dependencies = append(dependencies, Dependency{
			Raw:     raw,
			Name:    pkg.Name,
			Version: pkg.Version,
		})
	}

	if len(dependencies) == 0 {
		return manifestParserResult{
			Matched:         true,
			HasDependencies: boolPtr(false),
		}, nil
	}

	return manifestParserResult{
		Dependencies:    dependencies,
		Matched:         true,
		HasDependencies: boolPtr(true),
	}, nil
}

func hasPoetryLockMetadata(metadata *poetryLockMetadata) bool {
	if metadata == nil {
		return false
	}

	return metadata.LockVersion != "" || metadata.PythonVersions != "" || metadata.ContentHash != ""
}

func shouldIgnorePoetryLockPackage(pkg poetryLockPackage) bool {
	if pkg.Source == nil {
		return false
	}
	if strings.TrimSpace(pkg.Source.Type) == "directory" {
		return isSelfDirectorySource(pkg.Source.URL)
	}
	return false
}

func isSelfDirectorySource(value string) bool {
	switch strings.TrimSpace(value) {
	case ".", "./":
		return true
	default:
		return false
	}
}
