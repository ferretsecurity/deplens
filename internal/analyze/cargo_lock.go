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

func newCargoLockParser(raw cargoLockMatcherConfig) (manifestParser, error) {
	return cargoLockParser{}, nil
}

func (p cargoLockParser) Match(path string, content []byte) (manifestParserResult, error) {
	var file cargoLockFile
	if err := toml.Unmarshal(content, &file); err != nil {
		return manifestParserResult{}, fmt.Errorf("parse toml file %q: %w", path, err)
	}
	if file.Version == nil {
		return manifestParserResult{}, nil
	}

	dependencies := make([]Dependency, 0, len(file.Packages))
	for _, pkg := range file.Packages {
		if pkg.Name == "" || pkg.Version == "" {
			continue
		}
		dep := Dependency{
			Raw:     pkg.Name + "@" + pkg.Version,
			Name:    pkg.Name,
			Version: pkg.Version,
		}
		if pkg.Source != nil && *pkg.Source != "" {
			sourceType, extras := parseCargoLockSource(*pkg.Source, nil)
			dep.Source = sourceType
			dep.Extras = extras
		}
		if pkg.Checksum != nil && *pkg.Checksum != "" {
			if dep.Extras == nil {
				dep.Extras = make(map[string]string)
			}
			dep.Extras["checksum"] = *pkg.Checksum
		}
		dependencies = append(dependencies, dep)
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

func parseCargoLockSource(source string, extras map[string]string) (sourceType string, updatedExtras map[string]string) {
	if extras == nil {
		extras = make(map[string]string)
	}
	switch {
	case strings.HasPrefix(source, "registry+"):
		url := strings.TrimPrefix(source, "registry+")
		extras["source_url"] = url
		return "registry", extras
	case strings.HasPrefix(source, "git+"):
		raw := strings.TrimPrefix(source, "git+")
		if idx := strings.LastIndex(raw, "#"); idx >= 0 {
			extras["source_url"] = raw[:idx]
			extras["source_ref"] = raw[idx+1:]
		} else {
			extras["source_url"] = raw
		}
		return "git", extras
	default:
		extras["source_url"] = source
		return "url", extras
	}
}
