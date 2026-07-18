package analyze

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

type cargoLockParser struct{}

type cargoLockFile struct {
	Version  *int               `toml:"version"`
	Packages []cargoLockPackage `toml:"package"`
}

type cargoLockPackage struct {
	Name     string  `toml:"name"`
	Version  string  `toml:"version"`
	Source   *string `toml:"source"`
	Checksum *string `toml:"checksum"`
}

func newCargoLockParser(raw cargoLockMatcherConfig) (sourceAnalyzer, error) {
	return cargoLockParser{}, nil
}

func (p cargoLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var file cargoLockFile
	if err := toml.Unmarshal(content, &file); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse toml file %q: %w", path, err)
	}
	if file.Version == nil {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(file.Packages))
	for _, pkg := range file.Packages {
		if pkg.Name == "" || pkg.Version == "" {
			continue
		}
		dep := DependencyReference{
			Raw:     pkg.Name + "@" + pkg.Version,
			Name:    pkg.Name,
			Version: pkg.Version,
		}
		if pkg.Source != nil && *pkg.Source != "" {
			originKind, attributes := parseCargoLockOrigin(*pkg.Source, nil)
			dep.OriginKind = originKind
			dep.Attributes = attributes
		}
		if pkg.Checksum != nil && *pkg.Checksum != "" {
			if dep.Attributes == nil {
				dep.Attributes = make(map[string]string)
			}
			dep.Attributes["checksum"] = *pkg.Checksum
		}
		dependencies = append(dependencies, dep)
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

func parseCargoLockOrigin(source string, attributes map[string]string) (OriginKind, map[string]string) {
	if attributes == nil {
		attributes = make(map[string]string)
	}
	switch {
	case strings.HasPrefix(source, "registry+"):
		url := strings.TrimPrefix(source, "registry+")
		attributes["source_url"] = url
		return OriginRegistry, attributes
	case strings.HasPrefix(source, "git+"):
		raw := strings.TrimPrefix(source, "git+")
		if idx := strings.LastIndex(raw, "#"); idx >= 0 {
			attributes["source_url"] = raw[:idx]
			attributes["source_ref"] = raw[idx+1:]
		} else {
			attributes["source_url"] = raw
		}
		return OriginGit, attributes
	default:
		attributes["source_url"] = source
		return OriginURL, attributes
	}
}
