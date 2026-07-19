package analyze

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type uvLockParser struct{}

type uvLockFile struct {
	Version  *int            `toml:"version"`
	Packages []uvLockPackage `toml:"package"`
}

type uvLockPackage struct {
	Name    string        `toml:"name"`
	Version string        `toml:"version"`
	Source  *uvLockSource `toml:"source"`
}

type uvLockSource struct {
	Editable  *string `toml:"editable"`
	Workspace *bool   `toml:"workspace"`
	Virtual   *string `toml:"virtual"`
	Path      *string `toml:"path"`
}

func newUVLockParser(raw uvLockMatcherConfig) (sourceAnalyzer, error) {
	return uvLockParser{}, nil
}

func (p uvLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var file uvLockFile
	if err := toml.Unmarshal(content, &file); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse toml file %q: %w", path, err)
	}
	if file.Version == nil {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(file.Packages))
	for _, pkg := range file.Packages {
		if pkg.Name == "" {
			continue
		}
		if pkg.Source != nil {
			if pkg.Source.Editable != nil && isSelfEditableSource(*pkg.Source.Editable) {
				continue
			}
			if pkg.Source.Workspace != nil && *pkg.Source.Workspace {
				continue
			}
			if pkg.Source.Virtual != nil {
				continue
			}
			if pkg.Source.Path != nil {
				dependencies = append(dependencies, DependencyReference{
					Raw:        pkg.Name,
					Name:       pkg.Name,
					OriginKind: OriginPath,
				})
				continue
			}
		}
		if pkg.Version == "" {
			continue
		}
		dependencies = append(dependencies, DependencyReference{
			Raw:     pkg.Name + "==" + pkg.Version,
			Name:    pkg.Name,
			Version: pkg.Version,
		})
	}

	if len(dependencies) == 0 {
		return sourceAnalyzerResult{
			Recognized: true,
			Analysis:   SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
		}, nil
	}

	return sourceAnalyzerResult{
		Dependencies: dependencies,
		Recognized:   true,
		Analysis:     SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
	}, nil
}

func isSelfEditableSource(value string) bool {
	switch strings.TrimSpace(value) {
	case ".", "./":
		return true
	default:
		return false
	}
}
