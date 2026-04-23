package analyze

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type cargoLockParser struct{}

type cargoLockFile struct {
	Version  *int               `toml:"version"`
	Packages []cargoLockPackage `toml:"package"`
}

type cargoLockPackage struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
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
		dependencies = append(dependencies, Dependency{
			Name: pkg.Name + "@" + pkg.Version,
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
