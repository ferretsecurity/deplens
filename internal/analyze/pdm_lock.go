package analyze

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type pdmLockParserConfig struct{}

type pdmLockParser struct{}

type pdmLockFile struct {
	Version  string           `toml:"version"`
	Metadata *pdmLockMetadata `toml:"metadata"`
	Packages []pdmLockPackage `toml:"package"`
}

type pdmLockMetadata struct {
	LockVersion string `toml:"lock_version"`
}

type pdmLockPackage struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

func newPDMLockParser(pdmLockParserConfig) (sourceAnalyzer, error) {
	return pdmLockParser{}, nil
}

func (pdmLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var file pdmLockFile
	if err := toml.Unmarshal(content, &file); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse PDM lockfile %q: %w", path, err)
	}
	if !hasPDMLockVersion(file) {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(file.Packages))
	seen := make(map[string]struct{}, len(file.Packages))
	for _, pkg := range file.Packages {
		if pkg.Name == "" || pkg.Version == "" {
			continue
		}
		raw := pkg.Name + "==" + pkg.Version
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		dependencies = append(dependencies, DependencyReference{
			Raw:     raw,
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

func hasPDMLockVersion(file pdmLockFile) bool {
	return file.Version != "" || (file.Metadata != nil && file.Metadata.LockVersion != "")
}
