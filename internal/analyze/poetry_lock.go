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
	Type string `toml:"type"`
	URL  string `toml:"url"`
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
		if pkg.Name == "" || pkg.Version == "" {
			continue
		}
		if shouldIgnorePoetryLockPackage(pkg) {
			continue
		}

		name := pkg.Name + "==" + pkg.Version
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		dependencies = append(dependencies, Dependency{Name: name})
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

	switch strings.TrimSpace(pkg.Source.Type) {
	case "git":
		return true
	case "directory":
		return isSelfDirectorySource(pkg.Source.URL)
	default:
		return false
	}
}

func isSelfDirectorySource(value string) bool {
	switch strings.TrimSpace(value) {
	case ".", "./":
		return true
	default:
		return false
	}
}
