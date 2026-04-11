package analyze

import (
	"fmt"

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

func newUVLockParser(raw uvLockMatcherConfig) (manifestParser, error) {
	return uvLockParser{}, nil
}

func (p uvLockParser) Match(path string, content []byte) (manifestParserResult, error) {
	var file uvLockFile
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
		if pkg.Source != nil {
			if pkg.Source.Editable != nil {
				continue
			}
			if pkg.Source.Workspace != nil && *pkg.Source.Workspace {
				continue
			}
			if pkg.Source.Virtual != nil {
				continue
			}
		}

		dependencies = append(dependencies, Dependency{
			Name: pkg.Name + "==" + pkg.Version,
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
