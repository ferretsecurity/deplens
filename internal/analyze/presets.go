package analyze

import (
	"fmt"
	"slices"
)

// coveragePresets is the frozen 2026-09-17 source-format research snapshot.
// Membership and setup evidence: docs/coverage-presets.md.
var coveragePresets = map[string][]string{
	"snyk": {
		"python-requirements",
		"python-requirements-dir",
		"python-constraints",
		"python-poetry-lock",
		"python-pipfile",
		"python-pipfile-lock",
		"python-setup-py",
		"python-uv",
		"js",
		"js-npm-lock",
		"js-npm-shrinkwrap",
		"js-yarn",
		"js-pnpm-lock",
		"java",
		"java-gradle",
		"java-gradle-kts",
		"java-gradle-lockfile",
		"java-gradle-settings",
		"java-gradle-settings-kts",
		"java-gradle-version-catalog",
		"scala-sbt-build",
		"scala-sbt-dependencies",
		"ruby-gemfile",
		"ruby-gemfile-lock",
		"php-composer",
		"php-composer-lock",
		"go-mod",
		"go-gopkg-lock",
		"go-gopkg-toml",
		"dotnet-packages-config",
		"dotnet-csproj",
		"dotnet-directory-packages-props",
		"dotnet-directory-build",
		"dotnet-paket-dependencies",
		"dotnet-paket-lock",
		"ios-podfile",
		"ios-podfile-lock",
		"swift-package",
		"swift-package-resolved",
		"elixir-mix",
		"elixir-mix-lock",
		"dart-pubspec",
		"dart-pubspec-lock",
	},
	"socket": {
		"python-requirements-dir",
		"python-uv",
		"python-poetry-lock",
		"python-pipfile-lock",
		"python-pyproject",
		"python-pipfile",
		"python-setup-py",
		"js",
		"js-npm-shrinkwrap",
		"js-npm-lock",
		"js-yarn",
		"js-pnpm-lock",
		"js-bun-lock",
		"js-bun-lockb",
		"js-pnpm-workspace",
		"js-rush",
		"java",
		"java-gradle-lockfile",
		"java-gradle",
		"java-gradle-kts",
		"java-gradle-settings",
		"java-gradle-settings-kts",
		"java-gradle-version-catalog",
		"scala-sbt-build",
		"scala-sbt-dependencies",
		"ruby-gemfile",
		"ruby-gemfile-lock",
		"ruby-gemspec",
		"php-composer",
		"php-composer-lock",
		"dotnet-packages-config",
		"dotnet-packages-lock",
		"dotnet-fsproj",
		"dotnet-vbproj",
		"dotnet-csproj",
		"go-mod",
		"go-sum",
		"rust-cargo",
		"rust-cargo-lock",
		"github-actions-action",
		"github-actions-workflow",
	},
}

// Only present preset members are exclusions. Absence from a custom base is
// not an explicit removal and must not change policy prerequisite behavior.
func presetExclusions(names []string, rules Ruleset) ([]string, error) {
	for _, name := range names {
		if _, ok := coveragePresets[name]; !ok {
			return nil, fmt.Errorf("unknown coverage preset %q (expected snyk or socket)", name)
		}
	}
	var ids []string
	for _, id := range rules.DetectorIDs() {
		for _, name := range names {
			if slices.Contains(coveragePresets[name], string(id)) {
				ids = append(ids, string(id))
				break
			}
		}
	}
	return ids, nil
}
