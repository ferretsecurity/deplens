package analyze

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func dependencyNames(dependencies []DependencyReference) []string {
	names := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		if dependency.Raw != "" {
			names = append(names, dependency.Raw)
		} else {
			names = append(names, dependency.Name)
		}
	}
	return names
}

func equalDependencies(a, b []DependencyReference) bool {
	return reflect.DeepEqual(a, b)
}

func TestDependenciesFromStringsSetsRaw(t *testing.T) {
	got := dependenciesFromStrings([]string{"requests==2.32.3", "flask==3.0.0"})
	if len(got) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(got))
	}
	for i, dep := range got {
		if dep.Raw == "" {
			t.Errorf("dependency[%d]: expected Raw to be set, got empty", i)
		}
		if dep.Name != "" {
			t.Errorf("dependency[%d]: expected Name to be empty, got %q", i, dep.Name)
		}
	}
}

func TestMatchSelectorOnlySourceMatchesSupportedFiles(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []struct {
		name string
		want DetectorID
	}{
		{name: "bun.lock", want: DetectorID("js-bun-lock")},
		{name: "bun.lockb", want: DetectorID("js-bun-lockb")},
		{name: "settings.gradle", want: DetectorID("java-gradle-settings")},
		{name: "settings.gradle.kts", want: DetectorID("java-gradle-settings-kts")},
		{name: "Package.swift", want: DetectorID("swift-package")},
		{name: "Podfile", want: DetectorID("ios-podfile")},
		{name: "Cartfile", want: DetectorID("ios-cartfile")},
		{name: "rebar.lock", want: DetectorID("erlang-rebar-lock")},
		{name: "stack.yaml", want: DetectorID("haskell-stack")},
		{name: "stack.yaml.lock", want: DetectorID("haskell-stack-lock")},
		{name: "cabal.project", want: DetectorID("haskell-cabal-project")},
		{name: "paket.dependencies", want: DetectorID("dotnet-paket-dependencies")},
		{name: "paket.lock", want: DetectorID("dotnet-paket-lock")},
		{name: "go.sum", want: DetectorID("go-sum")},
		{name: "go.work", want: DetectorID("go-work")},
		{name: "Gopkg.toml", want: DetectorID("go-gopkg-toml")},
		{name: "glide.lock", want: DetectorID("go-glide-lock")},
		{name: "mix.exs", want: DetectorID("elixir-mix")},
		{name: "mix.lock", want: DetectorID("elixir-mix-lock")},
		{name: "demo.cabal", want: DetectorID("haskell-cabal")},
		{name: "demo.gemspec", want: DetectorID("ruby-gemspec")},
		{name: "cpanfile", want: DetectorID("perl-cpanfile")},
		{name: "build.zig.zon", want: DetectorID("zig-build-zon")},
		{name: "demo.nimble", want: DetectorID("nim-nimble")},
		{name: "demo.opam", want: DetectorID("ocaml-opam")},
		{name: "v.mod", want: DetectorID("vlang")},
		{name: "Brewfile", want: DetectorID("homebrew-brewfile")},
		{name: ".terraform.lock.hcl", want: DetectorID("terraform-lock")},
		{name: "action.yml", want: DetectorID("github-actions-action")},
		{name: "action.yaml", want: DetectorID("github-actions-action")},
		// Group 1b: Build systems and monorepo tools
		{name: "WORKSPACE", want: DetectorID("bazel-workspace")},
		{name: "WORKSPACE.bazel", want: DetectorID("bazel-workspace")},
		{name: "MODULE.bazel", want: DetectorID("bazel-module")},
		{name: "MODULE.bazel.lock", want: DetectorID("bazel-module-lock")},
		{name: "BUILD.bazel", want: DetectorID("bazel-build-file")},
		// DRAFT (Group 3): {name: "BUILD", want: DetectorID("bazel-build-file-bare")},
		{name: "nx.json", want: DetectorID("js-nx")},
		// DRAFT (Group 3): {name: "project.json", want: DetectorID("js-nx-project")},
		{name: "lerna.json", want: DetectorID("js-lerna")},
		{name: "rush.json", want: DetectorID("js-rush")},
		{name: "turbo.json", want: DetectorID("js-turbo")},
		{name: "pants.toml", want: DetectorID("pants-config")},
		{name: ".gitmodules", want: DetectorID("git-submodules")},
		// Group 1c: JVM ecosystem extensions
		{name: "build.sbt", want: DetectorID("scala-sbt-build")},
		{name: "build.sc", want: DetectorID("scala-mill")},
		{name: "ivy.xml", want: DetectorID("java-ivy")},
		{name: "ivysettings.xml", want: DetectorID("java-ivy-settings")},
		{name: "build.xml", want: DetectorID("java-ant-build")},
		// Group 1d: C/C++ ecosystem extensions
		{name: "CMakeLists.txt", want: DetectorID("cpp-cmake")},
		{name: "configure.ac", want: DetectorID("cpp-autotools")},
		{name: "configure.in", want: DetectorID("cpp-autotools")},
		// Group 1e: .NET ecosystem extensions
		{name: "Directory.Build.props", want: DetectorID("dotnet-directory-build")},
		{name: "Directory.Build.targets", want: DetectorID("dotnet-directory-build")},
		{name: "paket.references", want: DetectorID("dotnet-paket-references")},
		// Group 1f: JavaScript/Node ecosystem extensions
		{name: ".pnp.cjs", want: DetectorID("js-pnp")},
		{name: ".pnp.loader.mjs", want: DetectorID("js-pnp")},
		{name: "pnpm-workspace.yaml", want: DetectorID("js-pnpm-workspace")},
		{name: "pnpm-workspace.yml", want: DetectorID("js-pnpm-workspace")},
		{name: ".npmrc", want: DetectorID("js-npmrc")},
		{name: ".yarnrc.yml", want: DetectorID("js-yarnrc")},
		{name: "importmap.json", want: DetectorID("js-importmap")},
		// Group 1g: Python ecosystem extensions
		{name: "constraints.txt", want: DetectorID("python-constraints")},
		{name: "conda.yml", want: DetectorID("python-conda-env-alt")},
		{name: "conda.yaml", want: DetectorID("python-conda-env-alt")},
		// Group 1h: Systems languages extensions
		{name: "build.zig", want: DetectorID("zig-build")},
		// Group 1i: Ruby/iOS ecosystem extensions
		{name: "demo.podspec", want: DetectorID("ios-podspec")},
		{name: "Cartfile.resolved", want: DetectorID("ios-cartfile-resolved")},
		// Group 1j: Functional and niche languages
		{name: "cabal.project.freeze", want: DetectorID("haskell-cabal-project-freeze")},
		{name: "demo.rockspec", want: DetectorID("lua-rockspec")},
		{name: "renv.lock", want: DetectorID("r-renv-lock")},
		// DRAFT (Group 3): {name: "DESCRIPTION", want: DetectorID("r-description")},
		{name: "cpanfile.snapshot", want: DetectorID("perl-cpanfile-snapshot")},
		{name: "Makefile.PL", want: DetectorID("perl-makefile-pl")},
		{name: "Build.PL", want: DetectorID("perl-build-pl")},
		{name: "META.json", want: DetectorID("perl-meta")},
		{name: "META.yml", want: DetectorID("perl-meta")},
		{name: "META.yaml", want: DetectorID("perl-meta")},
		{name: "dist.ini", want: DetectorID("perl-dist-ini")},
		{name: "META6.json", want: DetectorID("raku-meta")},
		{name: "demo.opam.locked", want: DetectorID("ocaml-opam-locked")},
		{name: "dune-project", want: DetectorID("ocaml-dune-project")},
		{name: "esy.json", want: DetectorID("ocaml-esy")},
		// DRAFT (Group 3): {name: "dune", want: DetectorID("ocaml-dune")},
		{name: "manifest.toml", want: DetectorID("gleam-manifest")},
		{name: "fpm.toml", want: DetectorID("fortran-fpm")},
		// Group 1k: Nix
		{name: "default.nix", want: DetectorID("nix-default-shell")},
		{name: "shell.nix", want: DetectorID("nix-default-shell")},
		{name: "flake.nix", want: DetectorID("nix-flake")},
		{name: "flake.lock", want: DetectorID("nix-flake-lock")},
		// Group 1l: Infrastructure and ops tooling
		{name: "Chart.lock", want: DetectorID("helm-chart-lock")},
		{name: "Brewfile.lock.json", want: DetectorID("homebrew-brewfile-lock")},
		{name: "Puppetfile", want: DetectorID("puppet-puppetfile")},
		{name: "jsonnetfile.lock.json", want: DetectorID("jsonnet-lock")},
		{name: "Cask", want: DetectorID("emacs-cask")},
		// Group 1m: Game engines
		{name: "MyGame.uproject", want: DetectorID("unreal-uproject")},
		{name: "MyPlugin.uplugin", want: DetectorID("unreal-uplugin")},
		{name: "plugin.cfg", want: DetectorID("godot-plugin-cfg")},
		// Group 1n: Blockchain / Solidity
		{name: "foundry.toml", want: DetectorID("foundry-toml")},
		{name: "remappings.txt", want: DetectorID("foundry-remappings")},
		{name: "soldeer.lock", want: DetectorID("soldeer-lock")},
	}

	for _, tc := range testCases {
		got, ok := ruleset.MatchSelectorOnlySource(tc.name)
		if !ok {
			t.Fatalf("expected %s to be detected", tc.name)
		}
		if got != tc.want {
			t.Fatalf("expected type %q, got %q", tc.want, got)
		}
	}
}

func TestMatchSelectorOnlySourceIgnoresSimilarNames(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []string{
		"myrequirements.txt",
		"myrequirements.in",
		"requirementsdev.txt",
		"requirementsin",
		"requirements.txt.backup",
		"requirements.in.backup",
		"main.tf",
		"pom.xml.backup",
		"package-lock.json.bak",
		"npm-shrinkwrap.json.bak",
		"yarn.lock.old",
		"pnpm-lock.yaml.bak",
		"bun.lock.json",
		"bun.lockb.old",
		"deno.lock.backup",
		"deno.json.backup",
		"gradle.lockfile.tmp",
		"build.gradle.bak",
		"Gemfile.old",
		"Gemfile.lock.old",
		"Package.swift.old",
		"composer.json.backup",
		"composer.lock.old",
		"pubspec.yaml.tmp",
		"go.mod.bak",
		"go.work.sum",
		"uv.lock.json",
		"poetry.lock.toml",
		"go.sum.bak",
		"Cargo.toml.bak",
		"Cargo.lock.old",
		"Gopkg.lock.json",
		"glide.lock.old",
		"service.csproj.user",
		"Directory.Packages.props.user",
		"conan.lock.txt",
		"Package.resolved.backup",
		"Podfile.lock.old",
		"mix.lock.exs",
		"mix.exs.old",
		"project.cabal.bak",
		"foo.gemspec.old",
		"conanfile.txt.bak",
		"vcpkg.json.old",
		"Project.toml.old",
		"Manifest.toml.bak",
		"build.zig.zon.bak",
		"pkg.nimble.backup",
		"shard.yml.old",
		"gleam.toml.bak",
		"v.mod.json",
		"Chart.yaml.bak",
		"buf.yaml.old",
		"jsonnetfile.json.bak",
		".terraform.lock.hcl.bak",
		"Pipfile",
		"Pipfile.lock.bak",
		"conda-lock.yaml",
		"conda-lock.yml.bak",
		"index.html.bak",
		"component.jsx",
		"dockerfile",
		"DOCKERFILE",
		"docker-compose.override.yml",
		"docker-compose.dev.yml",
		"my-compose.yaml",
		"docker-compose.yml.bak",
		"my-action.yml",
		"action.yml.bak",
		"action.json",
		// Group 1b negatives
		"workspace",
		"WORKSPACE.txt",
		"BUILD",
		"build.bazel",
		"module.bazel",
		"MODULE.BAZEL",
		"nx.json.bak",
		"my-nx.json",
		"pants.toml.bak",
		"gitmodules",
		".gitmodule",
		"turbo.json.bak",
		// Group 1c negatives
		"build.sbt.bak",
		"BUILD.sbt",
		"build.sc.bak",
		"ivy.xml.bak",
		"ivysettings.xml.bak",
		"build.xml.bak",
		"BUILD.xml",
		// Group 1d negatives
		"cmakelists.txt",
		"CMakeLists.txt.bak",
		"conanfile.py.bak",
		"myconanfile.py",
		"vcpkg-configuration.json.bak",
		"meson.build.bak",
		"configure.ac.bak",
		"configure.m4",
		// Group 1e negatives
		"demo.fsproj.bak",
		"demo.vbproj.bak",
		"Directory.Build.props.bak",
		"Directory.build.props",
		"paket.references.bak",
		// Group 1f negatives
		".pnp.cjs.bak",
		"pnp.cjs",
		"pnpm-workspace.yaml.bak",
		"my-pnpm-workspace.yaml",
		".npmrc.bak",
		"npmrc",
		".yarnrc.yml.bak",
		"yarnrc.yml",
		"importmap.json.bak",
		"my-importmap.json",
		// Group 1g negatives
		"constraints.txt.bak",
		"myconstraints.txt",
		"conda.yml.bak",
		"my-conda.yml",
		// Group 1h negatives
		"build.zig.bak",
		"BUILD.zig",
		// Group 1i negatives
		"demo.podspec.bak",
		"Cartfile.resolved.bak",
		"cartfile.resolved",
		// Group 1j negatives
		"cabal.project.freeze.bak",
		"build.boot.bak",
		"BUILD.boot",
		"demo.rockspec.bak",
		"renv.lock.bak",
		"cpanfile.snapshot.bak",
		"Makefile.PL.bak",
		"makefile.pl",
		"Build.PL.bak",
		"build.pl",
		"META.json.bak",
		"META.toml",
		"dist.ini.bak",
		"META6.json.bak",
		"meta6.json",
		"pkg.opam.locked.bak",
		"dune-project.bak",
		"esy.json.bak",
		"shard.lock.bak",
		"manifest.toml.bak",
		"fpm.toml.bak",
		// Group 1k negatives
		"default.nix.bak",
		"mydefault.nix",
		"flake.nix.bak",
		"flake.lock.bak",
		// Group 1l negatives
		"Chart.lock.bak",
		"chart.lock",
		"buf.lock.bak",
		"puppetfile",
		"Puppetfile.lock",
		"berksfile",
		"Berksfile.lock.bak",
		"metadata.rb.bak",
		"my-metadata.rb",
		"policyfile.rb",
		"Policyfile.json",
		"Policyfile.lock.json.bak",
		"jsonnetfile.lock.json.bak",
		"cask",
		"Cask.lock",
		// Group 1m negatives
		"MyGame.uproject.bak",
		"MyPlugin.uplugin.bak",
		"plugin.cfg.bak",
		"PLUGIN.cfg",
		// Group 1n negatives
		"foundry.toml.bak",
		"FOUNDRY.toml",
		"remappings.txt.bak",
		"my-remappings.txt",
		"soldeer.lock.bak",
		"soldeer.toml",
	}

	for _, tc := range testCases {
		if _, ok := ruleset.MatchSelectorOnlySource(tc); ok {
			t.Fatalf("expected %s to be ignored", tc)
		}
	}
}

func TestMatchSelectorOnlySourceIgnoresAnalyzerBackedSources(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []string{
		"requirements.txt",
		"requirements.in",
		"my-requirements.txt",
		"my-requirements.in",
		"requirements-dev.txt",
		"requirements.dev.in",
		"requirements.qt6_3.in",
		"my_requirements.prod.txt",
		"package.json",
		"npm-shrinkwrap.json",
		"pnpm-lock.yaml",
		"conda-lock.yml",
		"bower.json",
		"composer.json",
		"composer.lock",
		"deno.json",
		"deno.jsonc",
		"pyproject.toml",
		"environment.yml",
		"environment.yaml",
		"Pipfile",
		"go.mod",
		"glide.yaml",
		"pubspec.yaml",
		"pubspec.lock",
		"Cargo.toml",
		"Cargo.lock",
		"pom.xml",
		"packages.lock.json",
		"requirements.yml",
		"requirements.yaml",
		"buf.yaml",
		"buf.lock",
		"jsonnetfile.json",
		"package.yaml",
		"vcpkg.json",
		"vcpkg-configuration.json",
		"Manifest.toml",
		"Package.resolved",
		"index.html",
		"job.tf",
		"app.js",
		"rebar.config",
		"build.boot",
		"project.clj",
		"Berksfile",
		"Berksfile.lock",
		"metadata.rb",
		"Policyfile.rb",
		"Policyfile.lock.json",
		"conanfile.txt",
		"conanfile.py",
		"conan.lock",
		"meson.build",
		"shard.lock",
	}

	for _, tc := range testCases {
		if _, ok := ruleset.MatchSelectorOnlySource(tc); ok {
			t.Fatalf("expected %s to be ignored by MatchSelectorOnlySource", tc)
		}
	}
}

func TestMatchSelectorOnlySourceIgnoresYarnLockParserRule(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	if _, ok := ruleset.MatchSelectorOnlySource("yarn.lock"); ok {
		t.Fatalf("expected yarn.lock to be ignored by MatchSelectorOnlySource")
	}
}

func TestMatchSelectorOnlySourceIgnoresPathGlobBackedSources(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: js\n      path-glob: 'apps/**/package.json'\n      form: other\n      roles:\n        - inventory\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	if _, ok := ruleset.MatchSelectorOnlySource("package.json"); ok {
		t.Fatalf("expected MatchSelectorOnlySource to ignore path-glob-backed rules")
	}
}

func TestAnalyzeDependencySourceAtRelativePathMatchesPathGlob(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-requirements\n      path-glob: '**/requirements/*.txt'\n      form: other\n      roles:\n        - inventory\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	got, deps, present, diagnosticMessages, ok, err := analyzeSourcePartsAtRelativePath(ruleset, "apps/api/requirements/base.txt", "base.txt", "apps/api/requirements/base.txt")
	if err != nil {
		t.Fatalf("AnalyzeDependencySourceAtRelativePath failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected path-glob rule to match relative path input")
	}
	if got != DetectorID("python-requirements") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if present != nil {
		t.Fatalf("expected unknown presence, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestAnalyzeDependencySourceAtRelativePathMatchesPathGlobWithAbsolutePath(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-requirements\n      path-glob: '**/requirements/*.txt'\n      form: other\n      roles:\n        - inventory\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	absPath := filepath.Join(root, "apps", "api", "requirements", "base.txt")

	got, deps, present, diagnosticMessages, ok, err := analyzeSourcePartsAtRelativePath(ruleset, absPath, "base.txt", "apps/api/requirements/base.txt")
	if err != nil {
		t.Fatalf("AnalyzeDependencySourceAtRelativePath failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected path-glob rule to match absolute path with explicit relative path")
	}
	if got != DetectorID("python-requirements") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if present != nil {
		t.Fatalf("expected unknown presence, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestAnalyzeDependencySourceDoesNotMatchPathGlobWithoutRelativePath(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-requirements\n      path-glob: 'apps/**/requirements/*.txt'\n      form: other\n      roles:\n        - inventory\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, "apps/api/requirements/base.txt", "base.txt")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if ok {
		t.Fatalf("expected no path-glob match without explicit relative path, got detector=%q deps=%+v presence=%+v diagnostics=%+v", got, deps, present, diagnosticMessages)
	}
}

func TestAnalyzeDependencySourceMatchesSelectorOnlyFilenameRuleWithEmptyPath(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, "", "bun.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected filename-only rule to match with empty path")
	}
	if got != DetectorID("js-bun-lock") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if present != nil {
		t.Fatalf("expected unknown presence, got %+v", present)
	}
}

func TestAnalyzeDependencySourceIgnoresParserBackedRuleWithEmptyPath(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, "", "package-lock.json")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if ok {
		t.Fatalf("expected parser-backed rule to be ignored with empty path, got detector=%q deps=%+v presence=%+v diagnostics=%+v", got, deps, present, diagnosticMessages)
	}
}

func TestScanFindsNestedSourcesSortedByRelativePath(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "b", "requirements.dev.in"), "")
	mustWriteFile(t, filepath.Join(root, "a", "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "a", "index.html"), `<script src="https://cdn.example.com/app.js"></script>`)
	mustWriteFile(t, filepath.Join(root, "c", "job.tf"), `
resource "aws_glue_job" "python_shell_example" {
  default_arguments = {
    "--job-language" = "python"
    "--additional-python-modules" = "scikit-learn==1.4.1.post1,pandas==2.2.1"
  }
}
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 4 {
		t.Fatalf("expected 4 dependency sources, got %d", len(result.Sources))
	}
	if result.Sources[0].Path != "a/index.html" || result.Sources[1].Path != "a/package.json" || result.Sources[2].Path != "b/requirements.dev.in" || result.Sources[3].Path != "c/job.tf" {
		t.Fatalf("unexpected dependency source order: %+v", result.Sources)
	}
}

func TestScanFindsRequirementsInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, source := range result.Sources {
		if source.Detector == DetectorID("python-requirements") && source.Path == "requirements.qt6_3.in" {
			if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"PyQt6==6.7.0"}) {
				t.Fatalf("unexpected dependencies: got %+v", source.Dependencies)
			}
			if source.Analysis.Presence != PresencePresent {
				t.Fatalf("expected presence=present, got %+v", source.Analysis)
			}
			return
		}
	}

	t.Fatalf("expected requirements.qt6_3.in fixture to be detected, got %+v", result.Sources)
}

func TestScanMatchesRequirementsFixtureWithExtractedDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "requirements-static"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("python-requirements") || source.Path != "requirements.txt" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
	if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"requests>=2.31", `uvicorn[standard]>=0.30 ; python_version >= "3.11"`}) {
		t.Fatalf("unexpected dependencies: got %+v", source.Dependencies)
	}
	if source.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", source.Analysis)
	}
}

func TestScanMatchesRequirementsFixtureWithOnlyDirectivesAsConclusiveEmpty(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "requirements-directives-only"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("python-requirements") || source.Path != "requirements.txt" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
	if source.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", source.Dependencies)
	}
	if source.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", source.Analysis)
	}
}

func TestScanMatchesRequirementsFixtureWithRecursiveIncludes(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "requirements-recursive"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("python-requirements") || source.Path != "requirements.txt" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
	if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"requests>=2.31", "urllib3<3", "pendulum>=3", "pytest>=8"}) {
		t.Fatalf("unexpected dependencies: got %+v", source.Dependencies)
	}
	if source.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", source.Analysis)
	}
	if len(source.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", source.Diagnostics)
	}
}

func TestScanMatchesRequirementsFixtureWithDiagnostics(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "requirements-missing-include"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"requests>=2.31"}) {
		t.Fatalf("unexpected dependencies: got %+v", source.Dependencies)
	}
	if source.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", source.Analysis)
	}
	if len(source.Diagnostics) != 1 {
		t.Fatalf("expected one warning, got %+v", source.Diagnostics)
	}
}

func TestScanMatchesRequirementsFixtureAsUnknownWhenOnlyUnreadableIncludeRemains(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "requirements-missing-include-only"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", source.Dependencies)
	}
	if source.Analysis.Presence != PresenceUnknown {
		t.Fatalf("expected unknown presence, got %+v", source.Analysis)
	}
	if len(source.Diagnostics) != 1 {
		t.Fatalf("expected one warning, got %+v", source.Diagnostics)
	}
}

func TestScanFindsPoetryLockInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, source := range result.Sources {
		if source.Detector == DetectorID("python-poetry-lock") && source.Path == "backend/poetry.lock" {
			return
		}
	}

	t.Fatalf("expected backend/poetry.lock fixture to be detected, got %+v", result.Sources)
}

func TestScanFindsCondaEnvironmentInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "conda-environment"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, source := range result.Sources {
		if source.Detector == DetectorID("python-conda-environment") && source.Path == "environment.yml" {
			if source.Dependencies != nil {
				t.Fatalf("expected no extracted dependencies, got %+v", source.Dependencies)
			}
			if source.Analysis.Presence != PresenceUnknown {
				t.Fatalf("expected presence to remain unknown, got %+v", source.Analysis)
			}
			return
		}
	}

	t.Fatalf("expected environment.yml fixture to be detected, got %+v", result.Sources)
}

func TestScanFindsPackageJSONFixtureWithoutMatchingSections(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "js", "package-json-0-sections"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Path != "package.json" {
		t.Fatalf("expected package.json fixture, got %+v", result.Sources[0])
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanFindsPackageJSONFixtureWithOneMatchingSection(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "js", "package-json-1-section"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Path != "package.json" {
		t.Fatalf("expected package.json fixture, got %+v", result.Sources[0])
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
	if len(result.Sources[0].Dependencies) != 1 ||
		result.Sources[0].Dependencies[0].Raw != "react@^19.0.0" ||
		result.Sources[0].Dependencies[0].PackageType != "npm" ||
		result.Sources[0].Dependencies[0].VERS != "vers:npm/>=19.0.0|<20.0.0" {
		t.Fatalf("expected extracted React dependency, got %+v", result.Sources[0].Dependencies)
	}
}

func TestScanFindsPackageJSONFixtureWithTwoMatchingSections(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "js", "package-json-2-sections"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Path != "package.json" {
		t.Fatalf("expected package.json fixture, got %+v", result.Sources[0])
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
	if got := result.Sources[0].Dependencies; len(got) != 2 || got[0].Raw != "react@^19.0.0" || got[1].Raw != "vitest@^3.0.0" {
		t.Fatalf("expected extracted runtime and development dependencies, got %+v", got)
	}
}

func TestScanMatchesComposerJSONWithRequireSections(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "json", "composer-json-with-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("php-composer") || result.Sources[0].Path != "composer.json" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
	dependencies := result.Sources[0].Dependencies
	if len(dependencies) != 2 || dependencies[0].Name != "monolog/monolog" || dependencies[0].Scope != ScopeRuntime ||
		dependencies[1].Name != "phpunit/phpunit" || dependencies[1].Scope != ScopeDevelopment {
		t.Fatalf("expected extracted require groups, got %+v", dependencies)
	}
}

func TestScanMatchesComposerJSONWithoutRequireSections(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "json", "composer-json-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesDenoJSONWithImports(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "json", "deno-json-with-imports"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("deno-json") || result.Sources[0].Path != "deno.json" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesDenoJSONCWithoutImports(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "json", "deno-jsonc-no-imports"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("deno-jsonc") || result.Sources[0].Path != "deno.jsonc" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesCargoTOMLWithDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "toml", "cargo-with-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("rust-cargo") || result.Sources[0].Path != "Cargo.toml" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
	dependencies := result.Sources[0].Dependencies
	if len(dependencies) != 1 || dependencies[0].Name != "nix" || dependencies[0].Relationship != RelationshipDirect ||
		dependencies[0].Scope != ScopeRuntime || dependencies[0].Attributes["target"] != "cfg(unix)" {
		t.Fatalf("expected extracted target dependency, got %+v", dependencies)
	}
}

func TestScanMatchesCargoTOMLWithoutDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "toml", "cargo-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesPubspecYAMLWithDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "yaml", "pubspec-with-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("dart-pubspec") || result.Sources[0].Path != "pubspec.yaml" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
	dependencies := result.Sources[0].Dependencies
	if len(dependencies) != 1 || dependencies[0].Name != "http" || dependencies[0].VersionConstraint != "^1.2.0" ||
		dependencies[0].OriginKind != OriginRegistry || dependencies[0].Scope != ScopeRuntime {
		t.Fatalf("expected extracted Pub dependency, got %+v", dependencies)
	}
}

func TestScanMatchesPubspecYAMLWithoutDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "yaml", "pubspec-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
	if len(result.Sources[0].Dependencies) != 0 || result.Sources[0].Analysis.Extraction != ExtractionComplete {
		t.Fatalf("expected complete empty extraction, got %+v", result.Sources[0])
	}
}

func TestScanMatchesMavenPOMWithDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "xml", "pom-with-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("java") || result.Sources[0].Path != "pom.xml" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
	dependencies := result.Sources[0].Dependencies
	if len(dependencies) != 1 || dependencies[0].Name != "org.slf4j:slf4j-api" ||
		dependencies[0].VersionConstraint != "[2.0.17]" || dependencies[0].Relationship != RelationshipDirect {
		t.Fatalf("expected extracted Maven dependency, got %+v", dependencies)
	}
}

func TestScanMatchesMavenPOMWithoutDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "xml", "pom-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesNamespacedMavenPOMWithDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "xml", "pom-with-namespace-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesMavenPOMWithDependencyManagementAsInconclusiveConstraint(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "xml", "pom-dependency-management-only"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
	dependencies := result.Sources[0].Dependencies
	if len(dependencies) != 1 || dependencies[0].Relationship != RelationshipInconclusive ||
		dependencies[0].SourceGroup != "dependencyManagement" || dependencies[0].Scope != ScopeBuild {
		t.Fatalf("expected inconclusive managed constraint, got %+v", dependencies)
	}
}

func TestScanMarksPresencePresentWhenDependenciesAreExtracted(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "toml", "pipfile"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanFindsPipfileLockInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, source := range result.Sources {
		if source.Detector == DetectorID("python-pipfile-lock") && source.Path == "backend/Pipfile.lock" {
			return
		}
	}

	t.Fatalf("expected backend/Pipfile.lock fixture to be detected, got %+v", result.Sources)
}

func TestScanFindsPdmLockInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, source := range result.Sources {
		if source.Detector == DetectorID("python-pdm-lock") && source.Path == "backend/pdm.lock" {
			if source.Analysis.Presence != PresencePresent {
				t.Fatalf("expected backend/pdm.lock fixture to have presence=present, got %+v", source.Analysis)
			}
			if source.Dependencies != nil {
				t.Fatalf("expected backend/pdm.lock fixture to remain non-extracting, got %+v", source.Dependencies)
			}
			return
		}
	}

	t.Fatalf("expected backend/pdm.lock fixture to be detected, got %+v", result.Sources)
}

func TestScanFindsGopkgLockInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, source := range result.Sources {
		if source.Detector == DetectorID("go-gopkg-lock") && source.Path == "go-service/Gopkg.lock" {
			if source.Analysis.Presence != PresencePresent {
				t.Fatalf("expected go-service/Gopkg.lock fixture to have presence=present, got %+v", source.Analysis)
			}
			if source.Dependencies != nil {
				t.Fatalf("expected go-service/Gopkg.lock fixture to remain non-extracting, got %+v", source.Dependencies)
			}
			return
		}
	}

	t.Fatalf("expected go-service/Gopkg.lock fixture to be detected, got %+v", result.Sources)
}

func TestScanFindsCondaLockInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, source := range result.Sources {
		if source.Detector == DetectorID("python-conda-lock") && source.Path == "backend/conda-lock.yml" {
			return
		}
	}

	t.Fatalf("expected backend/conda-lock.yml fixture to be detected, got %+v", result.Sources)
}

func TestScanFindsAdditionalLockfilesInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	want := map[string]DetectorID{
		"frontend/package-lock.json":   DetectorID("js-npm-lock"),
		"frontend/pnpm-lock.yaml":      DetectorID("js-pnpm-lock"),
		"frontend/bun.lock":            DetectorID("js-bun-lock"),
		"frontend/bun.lockb":           DetectorID("js-bun-lockb"),
		"frontend/deno.lock":           DetectorID("deno-lock"),
		"java-service/gradle.lockfile": DetectorID("java-gradle-lockfile"),
		"ruby-app/Gemfile":             DetectorID("ruby-gemfile"),
		"ruby-app/Gemfile.lock":        DetectorID("ruby-gemfile-lock"),
		"php-app/composer.json":        DetectorID("php-composer"),
		"php-app/composer.lock":        DetectorID("php-composer-lock"),
		"go-service/go.mod":            DetectorID("go-mod"),
		"go-service/go.sum":            DetectorID("go-sum"),
		"rust-app/Cargo.toml":          DetectorID("rust-cargo"),
		"rust-app/Cargo.lock":          DetectorID("rust-cargo-lock"),
		"go-service/Gopkg.lock":        DetectorID("go-gopkg-lock"),
		"go-service/glide.lock":        DetectorID("go-glide-lock"),
		"dotnet-app/app.csproj":        DetectorID("dotnet-csproj"),
		"cpp-app/conan.lock":           DetectorID("cpp-conan-lock"),
		"ios-app/Package.resolved":     DetectorID("swift-package-resolved"),
		"ios-app/Podfile.lock":         DetectorID("ios-podfile-lock"),
		"elixir-app/mix.lock":          DetectorID("elixir-mix-lock"),
	}

	for _, source := range result.Sources {
		wantType, ok := want[source.Path]
		if !ok {
			continue
		}
		if source.Detector != wantType {
			t.Fatalf("expected %s to be detected as %q, got %q", source.Path, wantType, source.Detector)
		}
		delete(want, source.Path)
	}

	if len(want) != 0 {
		t.Fatalf("expected all additional lockfile fixtures to be detected, missing %+v", want)
	}
}

func TestDefaultRulesScanPackageLockExtractedFixtures(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []struct {
		name string
		root string
		want []string
	}{
		{
			name: "lockfile v1",
			root: filepath.Join("..", "..", "testdata", "javascript", "package-lock-v1-with-deps"),
			want: []string{"left-pad@1.3.0", "lodash@4.17.21"},
		},
		{
			name: "lockfile v2",
			root: filepath.Join("..", "..", "testdata", "javascript", "package-lock-v2-with-deps"),
			want: []string{"left-pad@1.3.0", "lodash@4.17.21", "@types/node@20.12.7"},
		},
		{
			name: "lockfile v3",
			root: filepath.Join("..", "..", "testdata", "javascript", "package-lock-v3-with-deps"),
			want: []string{"left-pad@1.3.0", "lodash@4.17.21", "@types/node@20.12.7"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(tc.root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan failed: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
			}

			source := result.Sources[0]
			if source.Path != "package-lock.json" {
				t.Fatalf("unexpected dependency source path: %+v", source)
			}
			if source.Detector != DetectorID("js-npm-lock") {
				t.Fatalf("unexpected dependency source type: %+v", source)
			}
			got := dependencyNames(source.Dependencies)
			if !equalStringSets(got, tc.want) {
				t.Fatalf("unexpected dependencies: got %+v want %+v (order-independent)", got, tc.want)
			}
			if source.Analysis.Presence != PresencePresent {
				t.Fatalf("expected presence=present, got %+v", source.Analysis)
			}
		})
	}
}

func TestDefaultRulesScanPackageLockEmptyFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "package-lock-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Path != "package-lock.json" {
		t.Fatalf("unexpected dependency source path: %+v", source)
	}
	if source.Detector != DetectorID("js-npm-lock") {
		t.Fatalf("unexpected dependency source type: %+v", source)
	}
	if source.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", source.Dependencies)
	}
	if source.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", source.Analysis)
	}
}

func TestDefaultRulesScanNpmShrinkwrapExtractedFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "npm-shrinkwrap-v3-with-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Path != "npm-shrinkwrap.json" {
		t.Fatalf("unexpected dependency source path: %+v", source)
	}
	if source.Detector != DetectorID("js-npm-shrinkwrap") {
		t.Fatalf("unexpected dependency source type: %+v", source)
	}
	want := []string{"left-pad@1.3.0", "@types/node@20.12.7"}
	if got := dependencyNames(source.Dependencies); !equalStringSets(got, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v (order-independent)", source.Dependencies, want)
	}
	if source.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", source.Analysis)
	}
}

func TestDefaultRulesScanNpmShrinkwrapEmptyFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "npm-shrinkwrap-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Path != "npm-shrinkwrap.json" {
		t.Fatalf("unexpected dependency source path: %+v", source)
	}
	if source.Detector != DetectorID("js-npm-shrinkwrap") {
		t.Fatalf("unexpected dependency source type: %+v", source)
	}
	if source.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", source.Dependencies)
	}
	if source.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", source.Analysis)
	}
}

func TestScanDefaultRulesMarkStructuredPriorityOneFixturesWithDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []struct {
		name      string
		root      string
		path      string
		typ       DetectorID
		extracted bool
	}{
		{
			name: "helm chart",
			root: filepath.Join("..", "..", "testdata", "helm", "chart"),
			path: "Chart.yaml",
			typ:  DetectorID("helm-chart"),
		},
		{
			name:      "crystal shard",
			root:      filepath.Join("..", "..", "testdata", "crystal", "shard"),
			path:      "shard.yml",
			typ:       DetectorID("crystal-shard"),
			extracted: true,
		},
		{
			name: "julia project",
			root: filepath.Join("..", "..", "testdata", "julia", "project"),
			path: "Project.toml",
			typ:  DetectorID("julia-project"),
		},
		{
			name: "julia manifest",
			root: filepath.Join("..", "..", "testdata", "julia", "manifest"),
			path: "Manifest.toml",
			typ:  DetectorID("julia-manifest"),
		},
		{
			name: "gleam project",
			root: filepath.Join("..", "..", "testdata", "gleam", "project"),
			path: "gleam.toml",
			typ:  DetectorID("gleam"),
		},
		{
			name:      "dotnet csproj",
			root:      filepath.Join("..", "..", "testdata", "sample-monorepo", "dotnet-app"),
			path:      "app.csproj",
			typ:       DetectorID("dotnet-csproj"),
			extracted: true,
		},
		{
			name:      "directory packages props",
			root:      filepath.Join("..", "..", "testdata", "dotnet", "directory-packages-props-with-deps"),
			path:      "Directory.Packages.props",
			typ:       DetectorID("dotnet-directory-packages-props"),
			extracted: true,
		},
		{
			name:      "packages config",
			root:      filepath.Join("..", "..", "testdata", "dotnet", "packages-config-with-deps"),
			path:      "packages.config",
			typ:       DetectorID("dotnet-packages-config"),
			extracted: true,
		},
		{
			name: "unity packages manifest",
			root: filepath.Join("..", "..", "testdata", "unity", "packages-manifest-with-deps"),
			path: "Packages/manifest.json",
			typ:  DetectorID("unity-packages-manifest"),
		},
		{
			name: "bower manifest",
			root: filepath.Join("..", "..", "testdata", "js", "bower-with-deps"),
			path: "bower.json",
			typ:  DetectorID("js-bower"),
		},
		{
			name:      "pubspec lock",
			root:      filepath.Join("..", "..", "testdata", "yaml", "pubspec-lock-with-deps"),
			path:      "pubspec.lock",
			typ:       DetectorID("dart-pubspec-lock"),
			extracted: true,
		},
		{
			name: "glide yaml",
			root: filepath.Join("..", "..", "testdata", "go", "glide-yaml-with-deps"),
			path: "glide.yaml",
			typ:  DetectorID("go-glide-yaml"),
		},
		{
			name: "gopkg lock",
			root: filepath.Join("..", "..", "testdata", "go", "gopkg-lock-with-deps"),
			path: "Gopkg.lock",
			typ:  DetectorID("go-gopkg-lock"),
		},
		{
			name: "swift package resolved",
			root: filepath.Join("..", "..", "testdata", "swift", "package-resolved-with-deps"),
			path: "Package.resolved",
			typ:  DetectorID("swift-package-resolved"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(tc.root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan failed: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
			}

			source := result.Sources[0]
			if source.Detector != tc.typ || source.Path != tc.path {
				t.Fatalf("unexpected dependency source: %+v", source)
			}
			if source.Analysis.Presence != PresencePresent {
				t.Fatalf("expected presence=present, got %+v", source.Analysis)
			}
			if tc.extracted && len(source.Dependencies) == 0 {
				t.Fatalf("expected extracted dependencies, got %+v", source.Dependencies)
			}
			if !tc.extracted && source.Dependencies != nil {
				t.Fatalf("expected no extracted dependencies, got %+v", source.Dependencies)
			}
		})
	}
}

func TestStructuredPriorityOneTestdataIncludesWithAndWithoutDependencyExamples(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []struct {
		name        string
		withRoot    string
		withPath    string
		withoutRoot string
		withoutPath string
		typ         DetectorID
	}{
		{
			name:        "python pdm lock",
			withRoot:    filepath.Join("..", "..", "testdata", "python", "pdm-lock-with-deps"),
			withPath:    "pdm.lock",
			withoutRoot: filepath.Join("..", "..", "testdata", "python", "pdm-lock-no-deps"),
			withoutPath: "pdm.lock",
			typ:         DetectorID("python-pdm-lock"),
		},
		{
			name:        "python conda lock",
			withRoot:    filepath.Join("..", "..", "testdata", "python", "conda-lock-with-deps"),
			withPath:    "conda-lock.yml",
			withoutRoot: filepath.Join("..", "..", "testdata", "python", "conda-lock-no-deps"),
			withoutPath: "conda-lock.yml",
			typ:         DetectorID("python-conda-lock"),
		},
		{
			name:        "helm chart",
			withRoot:    filepath.Join("..", "..", "testdata", "helm", "chart"),
			withPath:    "Chart.yaml",
			withoutRoot: filepath.Join("..", "..", "testdata", "helm", "chart-no-deps"),
			withoutPath: "Chart.yaml",
			typ:         DetectorID("helm-chart"),
		},
		{
			name:        "crystal shard",
			withRoot:    filepath.Join("..", "..", "testdata", "crystal", "shard"),
			withPath:    "shard.yml",
			withoutRoot: filepath.Join("..", "..", "testdata", "crystal", "shard-no-deps"),
			withoutPath: "shard.yml",
			typ:         DetectorID("crystal-shard"),
		},
		{
			name:        "julia project",
			withRoot:    filepath.Join("..", "..", "testdata", "julia", "project"),
			withPath:    "Project.toml",
			withoutRoot: filepath.Join("..", "..", "testdata", "julia", "project-no-deps"),
			withoutPath: "Project.toml",
			typ:         DetectorID("julia-project"),
		},
		{
			name:        "julia manifest",
			withRoot:    filepath.Join("..", "..", "testdata", "julia", "manifest"),
			withPath:    "Manifest.toml",
			withoutRoot: filepath.Join("..", "..", "testdata", "julia", "manifest-no-deps"),
			withoutPath: "Manifest.toml",
			typ:         DetectorID("julia-manifest"),
		},
		{
			name:        "gleam project",
			withRoot:    filepath.Join("..", "..", "testdata", "gleam", "project"),
			withPath:    "gleam.toml",
			withoutRoot: filepath.Join("..", "..", "testdata", "gleam", "project-no-deps"),
			withoutPath: "gleam.toml",
			typ:         DetectorID("gleam"),
		},
		{
			name:        "dotnet csproj",
			withRoot:    filepath.Join("..", "..", "testdata", "sample-monorepo", "dotnet-app"),
			withPath:    "app.csproj",
			withoutRoot: filepath.Join("..", "..", "testdata", "dotnet", "csproj-no-deps"),
			withoutPath: "app.csproj",
			typ:         DetectorID("dotnet-csproj"),
		},
		{
			name:        "directory packages props",
			withRoot:    filepath.Join("..", "..", "testdata", "dotnet", "directory-packages-props-with-deps"),
			withPath:    "Directory.Packages.props",
			withoutRoot: filepath.Join("..", "..", "testdata", "dotnet", "directory-packages-props-no-deps"),
			withoutPath: "Directory.Packages.props",
			typ:         DetectorID("dotnet-directory-packages-props"),
		},
		{
			name:        "packages config",
			withRoot:    filepath.Join("..", "..", "testdata", "dotnet", "packages-config-with-deps"),
			withPath:    "packages.config",
			withoutRoot: filepath.Join("..", "..", "testdata", "dotnet", "packages-config-no-deps"),
			withoutPath: "packages.config",
			typ:         DetectorID("dotnet-packages-config"),
		},
		{
			name:        "packages lock",
			withRoot:    filepath.Join("..", "..", "testdata", "dotnet", "packages-lock-with-deps"),
			withPath:    "packages.lock.json",
			withoutRoot: filepath.Join("..", "..", "testdata", "dotnet", "packages-lock-no-deps"),
			withoutPath: "packages.lock.json",
			typ:         DetectorID("dotnet-packages-lock"),
		},
		{
			name:        "unity packages manifest",
			withRoot:    filepath.Join("..", "..", "testdata", "unity", "packages-manifest-with-deps"),
			withPath:    "Packages/manifest.json",
			withoutRoot: filepath.Join("..", "..", "testdata", "unity", "packages-manifest-no-deps"),
			withoutPath: "Packages/manifest.json",
			typ:         DetectorID("unity-packages-manifest"),
		},
		{
			name:        "bower manifest",
			withRoot:    filepath.Join("..", "..", "testdata", "js", "bower-with-deps"),
			withPath:    "bower.json",
			withoutRoot: filepath.Join("..", "..", "testdata", "js", "bower-no-deps"),
			withoutPath: "bower.json",
			typ:         DetectorID("js-bower"),
		},
		{
			name:        "pubspec lock",
			withRoot:    filepath.Join("..", "..", "testdata", "yaml", "pubspec-lock-with-deps"),
			withPath:    "pubspec.lock",
			withoutRoot: filepath.Join("..", "..", "testdata", "yaml", "pubspec-lock-no-deps"),
			withoutPath: "pubspec.lock",
			typ:         DetectorID("dart-pubspec-lock"),
		},
		{
			name:        "glide yaml",
			withRoot:    filepath.Join("..", "..", "testdata", "go", "glide-yaml-with-deps"),
			withPath:    "glide.yaml",
			withoutRoot: filepath.Join("..", "..", "testdata", "go", "glide-yaml-no-deps"),
			withoutPath: "glide.yaml",
			typ:         DetectorID("go-glide-yaml"),
		},
		{
			name:        "gopkg lock",
			withRoot:    filepath.Join("..", "..", "testdata", "go", "gopkg-lock-with-deps"),
			withPath:    "Gopkg.lock",
			withoutRoot: filepath.Join("..", "..", "testdata", "go", "gopkg-lock-no-deps"),
			withoutPath: "Gopkg.lock",
			typ:         DetectorID("go-gopkg-lock"),
		},
		{
			name:        "swift package resolved",
			withRoot:    filepath.Join("..", "..", "testdata", "swift", "package-resolved-with-deps"),
			withPath:    "Package.resolved",
			withoutRoot: filepath.Join("..", "..", "testdata", "swift", "package-resolved-no-deps"),
			withoutPath: "Package.resolved",
			typ:         DetectorID("swift-package-resolved"),
		},
		{
			name:        "ios podfile lock",
			withRoot:    filepath.Join("..", "..", "testdata", "ios", "podfile-lock-with-deps"),
			withPath:    "Podfile.lock",
			withoutRoot: filepath.Join("..", "..", "testdata", "ios", "podfile-lock-no-deps"),
			withoutPath: "Podfile.lock",
			typ:         DetectorID("ios-podfile-lock"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			withResult, err := Scan(tc.withRoot, nil, ruleset)
			if err != nil {
				t.Fatalf("scan with-deps fixture failed: %v", err)
			}
			if len(withResult.Sources) != 1 {
				t.Fatalf("expected 1 with-deps dependency source, got %+v", withResult.Sources)
			}
			if withResult.Sources[0].Detector != tc.typ || withResult.Sources[0].Path != tc.withPath {
				t.Fatalf("unexpected with-deps dependency source: %+v", withResult.Sources[0])
			}
			if withResult.Sources[0].Analysis.Presence != PresencePresent {
				t.Fatalf("expected with-deps fixture to have presence=present, got %+v", withResult.Sources[0].Analysis)
			}

			withoutResult, err := Scan(tc.withoutRoot, nil, ruleset)
			if err != nil {
				t.Fatalf("scan no-deps fixture failed: %v", err)
			}
			if len(withoutResult.Sources) != 1 {
				t.Fatalf("expected 1 no-deps dependency source, got %+v", withoutResult.Sources)
			}
			if withoutResult.Sources[0].Detector != tc.typ || withoutResult.Sources[0].Path != tc.withoutPath {
				t.Fatalf("unexpected no-deps dependency source: %+v", withoutResult.Sources[0])
			}
			if withoutResult.Sources[0].Analysis.Presence != PresenceAbsent {
				t.Fatalf("expected no-deps fixture to have presence=absent, got %+v", withoutResult.Sources[0].Analysis)
			}
		})
	}
}

func TestScanDefaultRulesMarkStructuredPriorityOneFilesWithoutDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []struct {
		name    string
		relPath string
		content string
		typ     DetectorID
	}{
		{
			name:    "helm chart missing dependencies",
			relPath: "Chart.yaml",
			content: "apiVersion: v2\nname: demo\nversion: 0.1.0\n",
			typ:     DetectorID("helm-chart"),
		},
		{
			name:    "crystal shard empty dependencies",
			relPath: "shard.yml",
			content: "name: demo\nversion: 0.1.0\ndependencies: {}\n",
			typ:     DetectorID("crystal-shard"),
		},
		{
			name:    "julia project empty deps",
			relPath: "Project.toml",
			content: "name = \"Demo\"\nuuid = \"00000000-0000-0000-0000-000000000000\"\n\n[deps]\n",
			typ:     DetectorID("julia-project"),
		},
		{
			name:    "gleam empty dependencies",
			relPath: "gleam.toml",
			content: "name = \"demo\"\nversion = \"0.1.0\"\n\n[dependencies]\n",
			typ:     DetectorID("gleam"),
		},
		{
			name:    "dotnet csproj without package reference",
			relPath: "app.csproj",
			content: "<Project Sdk=\"Microsoft.NET.Sdk\"><ItemGroup><Compile Include=\"Program.cs\" /></ItemGroup></Project>\n",
			typ:     DetectorID("dotnet-csproj"),
		},
		{
			name:    "directory packages props without package version",
			relPath: "Directory.Packages.props",
			content: "<Project><PropertyGroup><ManagePackageVersionsCentrally>true</ManagePackageVersionsCentrally></PropertyGroup></Project>\n",
			typ:     DetectorID("dotnet-directory-packages-props"),
		},
		{
			name:    "packages config empty",
			relPath: "packages.config",
			content: "<?xml version=\"1.0\" encoding=\"utf-8\"?><packages></packages>\n",
			typ:     DetectorID("dotnet-packages-config"),
		},
		{
			name:    "unity manifest empty dependencies",
			relPath: filepath.Join("Packages", "manifest.json"),
			content: "{\n  \"dependencies\": {}\n}\n",
			typ:     DetectorID("unity-packages-manifest"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mustWriteFile(t, filepath.Join(root, tc.relPath), tc.content)

			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan failed: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
			}

			source := result.Sources[0]
			if source.Detector != tc.typ {
				t.Fatalf("unexpected dependency source type: %+v", source)
			}
			if source.Analysis.Presence != PresenceAbsent {
				t.Fatalf("expected presence=absent, got %+v", source.Analysis)
			}
			if len(source.Dependencies) != 0 {
				t.Fatalf("expected no extracted dependencies, got %+v", source.Dependencies)
			}
		})
	}
}

func TestScanDefaultRulesDoNotMatchNonUnityManifestJSONPaths(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "manifest.json"), "{\n  \"dependencies\": {\"com.unity.textmeshpro\": \"3.0.6\"}\n}\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanDefaultRulesMatchRequirementsDirectoriesAnywhere(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "requirements", "base.txt"), "requests>=2.31\n")
	mustWriteFile(t, filepath.Join(root, "apps", "api", "requirements", "base.txt"), "urllib3<3\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 2 {
		t.Fatalf("expected 2 dependency sources, got %+v", result.Sources)
	}
	if result.Sources[0].Detector != DetectorID("python-requirements-dir") || result.Sources[1].Detector != DetectorID("python-requirements-dir") {
		t.Fatalf("unexpected dependency source types: %+v", result.Sources)
	}
	if result.Sources[0].Path != "apps/api/requirements/base.txt" || result.Sources[1].Path != "requirements/base.txt" {
		t.Fatalf("unexpected dependency sources: %+v", result.Sources)
	}
	if got := dependencyNames(result.Sources[0].Dependencies); !slices.Equal(got, []string{"urllib3<3"}) {
		t.Fatalf("unexpected dependencies for nested requirements dir file: %+v", result.Sources[0].Dependencies)
	}
	if got := dependencyNames(result.Sources[1].Dependencies); !slices.Equal(got, []string{"requests>=2.31"}) {
		t.Fatalf("unexpected dependencies for top-level requirements dir file: %+v", result.Sources[1].Dependencies)
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected nested requirements dir file to have dependencies=true, got %+v", result.Sources[0].Analysis)
	}
	if result.Sources[1].Analysis.Presence != PresencePresent {
		t.Fatalf("expected top-level requirements dir file to have dependencies=true, got %+v", result.Sources[1].Analysis)
	}
}

func TestScanDefaultRulesReportEmptyRequirementsFilesConclusiveEmpty(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "requirements.txt"), "\n# comment only\n")
	mustWriteFile(t, filepath.Join(root, "requirements", "base.txt"), "--index-url https://pypi.example.com/simple\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 2 {
		t.Fatalf("expected 2 dependency sources, got %+v", result.Sources)
	}

	for _, source := range result.Sources {
		if source.Dependencies != nil {
			t.Fatalf("expected no extracted dependencies, got %+v", source.Dependencies)
		}
		if source.Analysis.Presence != PresenceAbsent {
			t.Fatalf("expected presence=absent, got %+v", source.Analysis)
		}
	}
}

func TestScanDefaultRulesDoNotMatchNestedRequirementGrandchildren(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "api", "requirements", "nested", "base.txt"), "")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanMatchesPyprojectDependenciesFromFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "toml", "pyproject"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("python-pyproject") || source.Path != "pyproject.toml" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}

	want := []DependencyReference{
		{PackageType: PackageType("pypi"), Raw: "scikit-build-core>=0.10", Name: "scikit-build-core", VersionConstraint: ">=0.10", VERS: "vers:pypi/>=0.10", SourceGroup: "build-system.requires"},
		{PackageType: PackageType("pypi"), Raw: "pybind11>=2.12.0", Name: "pybind11", VersionConstraint: ">=2.12.0", VERS: "vers:pypi/>=2.12.0", SourceGroup: "build-system.requires"},
		{PackageType: PackageType("pypi"), Raw: "requests>=2.31", Name: "requests", VersionConstraint: ">=2.31", VERS: "vers:pypi/>=2.31", SourceGroup: "project.dependencies"},
		{PackageType: PackageType("pypi"), Raw: "fastapi[all]>=0.110; python_version >= '3.10'", Name: "fastapi", VersionConstraint: ">=0.110", VERS: "vers:pypi/>=0.110", SourceGroup: "project.dependencies", Attributes: map[string]string{"extras": "all", "marker": "python_version >= '3.10'"}},
		{PackageType: PackageType("pypi"), Raw: "pytest>=8", Name: "pytest", VersionConstraint: ">=8", VERS: "vers:pypi/>=8", SourceGroup: "project.optional-dependencies.dev"},
		{PackageType: PackageType("pypi"), Raw: "ruff==0.4.8", Name: "ruff", VersionConstraint: "==0.4.8", VERS: "vers:pypi/%3D0.4.8", SourceGroup: "project.optional-dependencies.dev"},
		{PackageType: PackageType("pypi"), Raw: "mypy>=1.10", Name: "mypy", VersionConstraint: ">=1.10", VERS: "vers:pypi/>=1.10", SourceGroup: "dependency-groups.lint"},
		{PackageType: PackageType("pypi"), Raw: "django = \"^5.0\"", SourceGroup: "tool.poetry.dependencies"},
		{PackageType: PackageType("pypi"), Raw: "httpx = { extras = [\"http2\"], version = \"^0.27\" }", SourceGroup: "tool.poetry.dependencies"},
		{PackageType: PackageType("pypi"), Raw: "private-lib = { branch = \"main\", git = \"https://github.com/acme/private-lib.git\" }", SourceGroup: "tool.poetry.dependencies"},
		{PackageType: PackageType("pypi"), Raw: "factory-boy = { markers = \"python_version >= '3.11'\", version = \"^3.3\" }", SourceGroup: "tool.poetry.group.test.dependencies"},
		{PackageType: PackageType("pypi"), Raw: "pytest-cov = \"^5.0\"", SourceGroup: "tool.poetry.group.test.dependencies"},
	}
	if !equalDependencies(source.Dependencies, want) {
		t.Fatalf("unexpected dependencies: %+v", source.Dependencies)
	}
}

func TestScanMatchesPipfileWithStandardAndCustomPackageSections(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "Pipfile"), `
[[source]]
name = "pypi"
url = "https://pypi.org/simple"
verify_ssl = true

[requires]
python_version = "3.12"

[packages]
requests = "*"

[docs]
sphinx = ">=7"
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("python-pipfile") || source.Path != "Pipfile" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}

	want := []string{
		"requests = \"*\"",
		"sphinx = \">=7\"",
	}
	if !slices.Equal(dependencyNames(source.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", source.Dependencies, want)
	}
}

func TestScanIgnoresPipfileWithMetadataOnly(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "Pipfile"), `
[[source]]
name = "pypi"
url = "https://pypi.org/simple"
verify_ssl = true

[requires]
python_version = "3.12"
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanSkipsMalformedPipfileAndContinues(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "Pipfile"), "blarg\n")
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[project]
dependencies = ["requests>=2.31"]
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 2 {
		t.Fatalf("expected 2 dependency sources, got %d", len(result.Sources))
	}

	if result.Sources[0].Detector != DetectorID("python-pipfile") || result.Sources[0].Path != "Pipfile" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if len(result.Sources[0].Diagnostics) != 1 || !strings.Contains(result.Sources[0].Diagnostics[0].Message, "parse toml file") {
		t.Fatalf("expected parse warning on Pipfile, got %+v", result.Sources[0].Diagnostics)
	}

	source := result.Sources[1]
	if source.Detector != DetectorID("python-pyproject") || source.Path != "pyproject.toml" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
}

func TestScanSkipsMalformedPipfileLockAndContinues(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "Pipfile.lock"), "blarg\n")
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[project]
dependencies = ["requests>=2.31"]
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 2 {
		t.Fatalf("expected 2 dependency sources, got %d", len(result.Sources))
	}

	if result.Sources[0].Detector != DetectorID("python-pipfile-lock") || result.Sources[0].Path != "Pipfile.lock" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if len(result.Sources[0].Diagnostics) != 1 || !strings.Contains(result.Sources[0].Diagnostics[0].Message, "parse json file") {
		t.Fatalf("expected parse warning on Pipfile.lock, got %+v", result.Sources[0].Diagnostics)
	}

	source := result.Sources[1]
	if source.Detector != DetectorID("python-pyproject") || source.Path != "pyproject.toml" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
}

func TestDefaultRulesSkipMalformedPyprojectAndContinue(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[project
dependencies = ["requests>=2.31"]
`)
	mustWriteFile(t, filepath.Join(root, "package.json"), `{"dependencies":{"react":"18.3.1"}}`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 2 {
		t.Fatalf("expected 2 dependency sources, got %d", len(result.Sources))
	}

	if result.Sources[0].Detector != DetectorID("js") || result.Sources[0].Path != "package.json" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	source := result.Sources[1]
	if source.Detector != DetectorID("python-pyproject") || source.Path != "pyproject.toml" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
	if len(source.Diagnostics) != 1 || !strings.Contains(source.Diagnostics[0].Message, "parse toml file") {
		t.Fatalf("expected parse warning on pyproject.toml, got %+v", source.Diagnostics)
	}
}

func TestScanMatchesPipfileDependenciesFromFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "toml", "pipfile"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("python-pipfile") || source.Path != "Pipfile" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}

	want := []string{
		"requests = \"*\"",
		"pytest = \">=8\"",
		"sphinx = { extras = [\"docs\"], version = \">=7\" }",
	}
	if !slices.Equal(dependencyNames(source.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", source.Dependencies, want)
	}
}

func TestScanIgnoresPipfileMetadataOnlyFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "toml", "pipfile-metadata-only"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanMatchesPipfilePackagesOnlyFixture(t *testing.T) {
	assertPipfileFixtureDependencies(t, "pipfile-packages-only", []string{"requests = \"*\""})
}

func TestScanMatchesPipfileDevPackagesOnlyFixture(t *testing.T) {
	assertPipfileFixtureDependencies(t, "pipfile-dev-packages-only", []string{`pytest = ">=8"`})
}

func TestScanMatchesPipfileCustomCategoryOnlyFixture(t *testing.T) {
	assertPipfileFixtureDependencies(t, "pipfile-tests-only", []string{`pytest-cov = ">=5"`})
}

func assertPipfileFixtureDependencies(t *testing.T, fixture string, want []string) {
	t.Helper()

	ruleset := mustLoadDefaultRules(t)
	result, err := Scan(filepath.Join("..", "..", "testdata", "toml", fixture), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("python-pipfile") || source.Path != "Pipfile" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
	if !slices.Equal(dependencyNames(source.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", source.Dependencies, want)
	}
}

func TestScanMatchesSetupPyWithInstallRequiresFromFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "setup-py-install-requires"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("python-setup-py") || source.Path != "setup.py" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
	if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"requests>=2.31", "pytest>=8", "ruff>=0.4"}) {
		t.Fatalf("unexpected dependencies: got %+v", got)
	}
}

func TestScanMatchesSetupPyWithExtrasRequireOnly(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "setup-py-extras-require"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if got := dependencyNames(result.Sources[0].Dependencies); !slices.Equal(got, []string{"pytest>=8", "ruff>=0.4", "mkdocs>=1.6"}) {
		t.Fatalf("unexpected dependencies: got %+v", got)
	}
}

func TestScanMatchesSetupPyWithInstallRequiresAndExtrasRequire(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "setup-py-both"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if got := dependencyNames(result.Sources[0].Dependencies); !slices.Equal(got, []string{"requests>=2.31", "pytest>=8"}) {
		t.Fatalf("unexpected dependencies: got %+v", got)
	}
}

func TestScanMatchesSetupPyWithoutExtractingNonLiteralDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "setup-py-nonliteral"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if got := result.Sources[0].Dependencies; len(got) != 0 {
		t.Fatalf("unexpected dependencies: got %+v", got)
	}
}

func assertSetupCfgFixtureDependencies(t *testing.T, fixture string, want []string) {
	t.Helper()

	ruleset := mustLoadDefaultRules(t)
	result, err := Scan(filepath.Join("..", "..", "testdata", "python", fixture), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("python-setup-cfg") || source.Path != "setup.cfg" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
	if got := dependencyNames(source.Dependencies); !slices.Equal(got, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", got, want)
	}
}

func TestScanMatchesSetupCfgWithInstallRequiresFromFixture(t *testing.T) {
	assertSetupCfgFixtureDependencies(t, "setup-cfg-install-requires", []string{"requests>=2.31", "urllib3<3"})
}

func TestScanMatchesSetupCfgWithSetupRequiresFromFixture(t *testing.T) {
	assertSetupCfgFixtureDependencies(t, "setup-cfg-setup-requires", []string{"setuptools_scm>=8", "wheel"})
}

func TestScanMatchesSetupCfgWithExtrasRequireFromFixture(t *testing.T) {
	assertSetupCfgFixtureDependencies(t, "setup-cfg-extras-require", []string{"pytest>=8", "ruff>=0.4", "mkdocs>=1.6"})
}

func TestScanMatchesSetupCfgWithoutExtractingUnsupportedValues(t *testing.T) {
	for _, fixture := range []string{
		"setup-cfg-inline-comma-unsupported",
		"setup-cfg-file-unsupported",
		"setup-cfg-interpolation-unsupported",
	} {
		t.Run(fixture, func(t *testing.T) {
			assertSetupCfgFixtureDependencies(t, fixture, nil)
		})
	}
}

func TestScanMatchesSetupCfgWithCommentsAndBlanks(t *testing.T) {
	assertSetupCfgFixtureDependencies(t, "setup-cfg-comments-and-blanks", []string{"requests>=2.31", "urllib3<3"})
}

func TestScanMatchesSetupCfgWithMixedSupportedAndUnsupportedValues(t *testing.T) {
	assertSetupCfgFixtureDependencies(t, "setup-cfg-mixed", []string{"requests>=2.31", "pytest>=8", "mkdocs>=1.6"})
}

func TestScanMatchesHTMLExternalScripts(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "templates", "index.html"), `
<!DOCTYPE html>
<html>
  <head>
    <script src="https://cdn.jsdelivr.net/npm/dompurify@3.0.8/dist/purify.min.js"></script>
    <script src="/assets/app.js"></script>
    <script>console.log("inline")</script>
    <script SRC="https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js"></script>
  </head>
</html>
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	source := result.Sources[0]
	if source.Detector != DetectorID("html-external-scripts") || source.Path != "templates/index.html" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
	want := []string{
		"https://cdn.jsdelivr.net/npm/dompurify@3.0.8/dist/purify.min.js",
		"https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js",
	}
	if !slices.Equal(dependencyNames(source.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", source.Dependencies, want)
	}
}

func TestScanMatchesHTMLModuleImportFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "html", "module-import"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("html-external-scripts") || source.Path != "index.html" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}

	want := []string{"https://cdn.jsdelivr.net/npm/swiper@12.1.2/+esm"}
	if !slices.Equal(dependencyNames(source.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", source.Dependencies, want)
	}
}

func TestScanMatchesHTMLNamespaceModuleImportFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "html", "module-namespace-import"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("html-external-scripts") || source.Path != "index.html" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}

	want := []string{"https://cdn.jsdelivr.net/npm/d3@7/+esm"}
	if !slices.Equal(dependencyNames(source.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", source.Dependencies, want)
	}
}

func TestScanMatchesHTMLImportMapFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "html", "importmap"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("html-external-scripts") || source.Path != "index.html" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}

	want := []string{
		"https://cdn.jsdelivr.net/npm/super-media-element@1.3/+esm",
		"https://cdn.jsdelivr.net/npm/media-tracks@0.2/+esm",
		"https://cdn.jsdelivr.net/npm/hls.js@1.6.0-beta.1/dist/hls.mjs",
	}
	if !slices.Equal(dependencyNames(source.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", source.Dependencies, want)
	}
}

func TestScanMatchesHTMLImportMapRemoteImports(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "templates", "index.html"), `
<!DOCTYPE html>
<html>
  <head>
    <script type="importmap">
      {
        "imports": {
          "super-media-element": "https://cdn.jsdelivr.net/npm/super-media-element@1.3/+esm",
          "media-tracks": "https://cdn.jsdelivr.net/npm/media-tracks@0.2/+esm",
          "@superstreamer/player": "/packages/player/dist/index.js",
          "hls.js": "https://cdn.jsdelivr.net/npm/hls.js@1.6.0-beta.1/dist/hls.mjs",
          "stylelint-config-recess-order": "https://registry.npmmirror.com/stylelint-config-recess-order/5.0.0/files/groups.js"
        }
      }
    </script>
  </head>
</html>
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	source := result.Sources[0]
	want := []string{
		"https://cdn.jsdelivr.net/npm/super-media-element@1.3/+esm",
		"https://cdn.jsdelivr.net/npm/media-tracks@0.2/+esm",
		"https://cdn.jsdelivr.net/npm/hls.js@1.6.0-beta.1/dist/hls.mjs",
		"https://registry.npmmirror.com/stylelint-config-recess-order/5.0.0/files/groups.js",
	}
	if !slices.Equal(dependencyNames(source.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", source.Dependencies, want)
	}
}

func TestScanMatchesHTMLImportMapESMShImports(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "index.html"), `
<script type="importmap">
{
  "imports": {
    "react/": "https://esm.sh/react@^19.1.0/",
    "react": "https://esm.sh/react@^19.1.0",
    "@google/genai": "https://esm.sh/@google/genai@^1.0.0",
    "recharts": "https://esm.sh/recharts@^2.15.3",
    "react-dom/": "https://esm.sh/react-dom@^19.1.0/"
  }
}
</script>
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	want := []string{
		"https://esm.sh/react@^19.1.0/",
		"https://esm.sh/react@^19.1.0",
		"https://esm.sh/@google/genai@^1.0.0",
		"https://esm.sh/recharts@^2.15.3",
		"https://esm.sh/react-dom@^19.1.0/",
	}
	if !slices.Equal(dependencyNames(result.Sources[0].Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Sources[0].Dependencies, want)
	}
}

func TestScanDoesNotMatchHTMLWithoutExternalScripts(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "page.html"), `
<script src="/assets/app.js"></script>
<script src="//cdn.example.com/app.js"></script>
<script>console.log("inline")</script>
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanMatchesJavaScriptBannerDetectors(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []struct {
		path     string
		wantType DetectorID
		wantDeps []string
	}{
		{
			path:     "jquery.min.js",
			wantType: DetectorID("js-banner-block-start"),
			wantDeps: []string{"jQuery@3.7.1"},
		},
		{
			path:     "purify.min.js",
			wantType: DetectorID("js-banner-plain-block-start"),
			wantDeps: []string{"DOMPurify@3.0.8"},
		},
		{
			path:     "bootstrap.min.js",
			wantType: DetectorID("js-banner-multiline-preserved"),
			wantDeps: []string{"Bootstrap@5.3.3"},
		},
		{
			path:     "mustache.min.js",
			wantType: DetectorID("js-banner-line-comment"),
			wantDeps: []string{"Mustache.js@4.2.0"},
		},
		{
			path:     "htmx.min.js",
			wantType: DetectorID("js-banner-version-tagged"),
			wantDeps: []string{"htmx.org@2.0.4"},
		},
	}

	result, err := Scan(filepath.Join("..", "..", "testdata", "js", "banners"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != len(testCases) {
		t.Fatalf("expected %d dependency sources, got %d", len(testCases), len(result.Sources))
	}

	gotByPath := make(map[string]DependencySourceResult, len(result.Sources))
	for _, source := range result.Sources {
		gotByPath[source.Path] = source
	}

	for _, tc := range testCases {
		source, ok := gotByPath[tc.path]
		if !ok {
			t.Fatalf("expected dependency source for %q, got %+v", tc.path, result.Sources)
		}
		if source.Detector != tc.wantType {
			t.Fatalf("unexpected dependency source type for %q: got %q want %q", tc.path, source.Detector, tc.wantType)
		}
		if !slices.Equal(dependencyNames(source.Dependencies), tc.wantDeps) {
			t.Fatalf("unexpected dependencies for %q: got %+v want %+v", tc.path, source.Dependencies, tc.wantDeps)
		}
	}
}

func TestScanDoesNotMatchJavaScriptWithoutBanner(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "app.js"), "console.log('no banner')\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanDoesNotMatchCSSBannerWithDefaultRules(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "style.css"), "/*! Bootstrap v5.3.3 */\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanMatchesTerraformGluePythonDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "glue", "job.tf"), `
resource "aws_glue_job" "python_shell_example" {
  default_arguments = {
    "--job-language" = "python"
    "--additional-python-modules" = "scikit-learn==1.4.1.post1,pandas==2.2.1"
  }
}
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("terraform.aws_glue_job.python") || result.Sources[0].Path != "glue/job.tf" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if len(result.Sources[0].Dependencies) != 0 {
		t.Fatalf("expected terraform detector to keep dependencies empty, got %+v", result.Sources[0].Dependencies)
	}
}

func TestScanDoesNotMatchTerraformWithoutAdditionalModules(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.tf"), `
resource "aws_glue_job" "python_shell_example" {
  default_arguments = {
    "--job-language" = "python"
  }
}
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanDoesNotMatchTerraformWithoutPythonLanguage(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.tf"), `
resource "aws_glue_job" "scala_job" {
  default_arguments = {
    "--job-language" = "scala"
    "--additional-python-modules" = "pandas==2.2.1"
  }
}
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanDoesNotMatchNonGlueTerraformResource(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "main.tf"), `
resource "aws_s3_bucket" "example" {
  bucket = "example"
}

locals {
  default_arguments = {
    "--job-language" = "python"
    "--additional-python-modules" = "pandas==2.2.1"
  }
}
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanMatchesTypeScriptGluePythonDependenciesWithNamespaceImport(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "glue", "job.ts"), `
import * as glue from "aws-cdk-lib/aws-glue";

new glue.CfnJob(this, "Job", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "pandas==2.2.1, scikit-learn==1.4.1.post1,",
  },
});
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("typescript.cdk.aws_glue_job.python") || result.Sources[0].Path != "glue/job.ts" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	wantDependencies := []string{"pandas==2.2.1", "scikit-learn==1.4.1.post1"}
	if !slices.Equal(dependencyNames(result.Sources[0].Dependencies), wantDependencies) {
		t.Fatalf("unexpected dependencies: got %v want %v", result.Sources[0].Dependencies, wantDependencies)
	}
}

func TestScanMatchesTypeScriptGluePythonDependenciesWithNamedImport(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.ts"), `
import { CfnJob as GlueJob } from "aws-cdk-lib/aws-glue";

new GlueJob(this, "Job", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "pandas==2.2.1",
  },
});
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if got := dependencyNames(result.Sources[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanDoesNotMatchTypeScriptWithoutAdditionalModules(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.ts"), `
import * as glue from "aws-cdk-lib/aws-glue";

new glue.CfnJob(this, "Job", {
  defaultArguments: {
    "--job-language": "python",
  },
});
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanDoesNotMatchTypeScriptWithoutPythonLanguage(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.ts"), `
import { CfnJob } from "aws-cdk-lib/aws-glue";

new CfnJob(this, "Job", {
  defaultArguments: {
    "--job-language": "scala",
    "--additional-python-modules": "pandas==2.2.1",
  },
});
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanDoesNotMatchTypeScriptWithUnrelatedImport(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.ts"), `
import { CfnJob } from "not/aws-glue";

new CfnJob(this, "Job", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "pandas==2.2.1",
  },
});
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanMatchesTypeScriptWithVariableProps(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.ts"), `
import { CfnJob } from "aws-cdk-lib/aws-glue";

const props = {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "pandas==2.2.1",
  },
};

new CfnJob(this, "Job", props);
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if got := dependencyNames(result.Sources[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanMatchesTypeScriptWithVariableAdditionalModules(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.ts"), `
import { CfnJob } from "aws-cdk-lib/aws-glue";

const modules = "pandas==2.2.1";

new CfnJob(this, "Job", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": modules,
  },
});
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if got := dependencyNames(result.Sources[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanMatchesTypeScriptFixtureFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "typescript", "glue-cfnjob-inline")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("typescript.cdk.aws_glue_job.python") || result.Sources[0].Path != "job.ts" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	wantDependencies := []string{"pandas==2.2.1", "scikit-learn==1.4.1.post1"}
	if !slices.Equal(dependencyNames(result.Sources[0].Dependencies), wantDependencies) {
		t.Fatalf("unexpected dependencies: got %v want %v", result.Sources[0].Dependencies, wantDependencies)
	}
}

func TestScanDoesNotMatchTypeScriptNegativeFixturesFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	fixtures := []string{
		filepath.Join("..", "..", "testdata", "typescript", "glue-cfnjob-no-modules"),
	}

	for _, root := range fixtures {
		result, err := Scan(root, nil, ruleset)
		if err != nil {
			t.Fatalf("scan failed for %s: %v", root, err)
		}
		if len(result.Sources) != 0 {
			t.Fatalf("expected no dependency sources for %s, got %+v", root, result.Sources)
		}
	}
}

func TestScanMatchesTypeScriptVariablePropsFixtureFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "typescript", "glue-cfnjob-variable-props")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("typescript.cdk.aws_glue_job.python") || result.Sources[0].Path != "job.ts" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if got := dependencyNames(result.Sources[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanMatchesTypeScriptComputedModulesFixtureFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "typescript", "glue-cfnjob-computed-modules")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("typescript.cdk.aws_glue_job.python") || result.Sources[0].Path != "job.ts" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if got := dependencyNames(result.Sources[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanMatchesTypeScriptFunctionComputedModulesFixtureFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "typescript", "glue-cfnjob-function-computed-modules")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("typescript.cdk.aws_glue_job.python") || result.Sources[0].Path != "job.ts" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if got := result.Sources[0].Dependencies; len(got) != 0 {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanMatchesPythonGlueDependenciesWithNamespaceImport(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "glue", "job.py"), `
from aws_cdk import aws_glue as glue

glue.CfnJob(
    self,
    "Job",
    role="arn:aws:iam::123456789012:role/glue",
    command={"name": "glueetl", "python_version": "3"},
    default_arguments={
        "--job-language": "python",
        "--additional-python-modules": "pandas==2.2.1, scikit-learn==1.4.1.post1,",
    },
)
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("python.cdk.aws_glue_job.python") || result.Sources[0].Path != "glue/job.py" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	wantDependencies := []string{"pandas==2.2.1", "scikit-learn==1.4.1.post1"}
	if !slices.Equal(dependencyNames(result.Sources[0].Dependencies), wantDependencies) {
		t.Fatalf("unexpected dependencies: got %v want %v", result.Sources[0].Dependencies, wantDependencies)
	}
}

func TestScanMatchesPythonGlueDependenciesWithNamedImportAndVariableArgs(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.py"), `
from aws_cdk.aws_glue import CfnJob

modules = "pandas==2.2.1"
default_args = {
    "--job-language": "python",
    "--additional-python-modules": modules,
}

CfnJob(
    self,
    "Job",
    role="arn:aws:iam::123456789012:role/glue",
    command={"name": "glueetl", "python_version": "3"},
    default_arguments=default_args,
)
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if got := dependencyNames(result.Sources[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanDoesNotMatchPythonGlueWithoutAdditionalModules(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.py"), `
from aws_cdk import aws_glue as glue

glue.CfnJob(
    self,
    "Job",
    role="arn:aws:iam::123456789012:role/glue",
    command={"name": "glueetl", "python_version": "3"},
    default_arguments={
        "--job-language": "python",
    },
)
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanMatchesPythonFixtureFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "python", "glue-cfnjob-inline")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("python.cdk.aws_glue_job.python") || result.Sources[0].Path != "job.py" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if got := dependencyNames(result.Sources[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanSkipsIgnoredDirectories(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "node_modules", "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "src", "package.json"), "{}")

	result, err := Scan(root, []string{"node_modules"}, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Path != "src/package.json" {
		t.Fatalf("unexpected dependency source path: %+v", result.Sources[0])
	}
}

func TestScanOverrideIgnoreListChangesTraversalBehavior(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "vendor", "pom.xml"), "")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected vendor dependency source to be found without ignore list, got %d", len(result.Sources))
	}

	result, err = Scan(root, []string{"vendor"}, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected vendor dependency source to be ignored, got %+v", result.Sources)
	}
}

func TestScanRejectsFilePath(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	filePath := filepath.Join(root, "package.json")
	mustWriteFile(t, filePath, "{}")

	_, err := Scan(filePath, nil, ruleset)
	if err == nil {
		t.Fatalf("expected error for file path")
	}
}

func TestLoadRulesRejectsMissingFields(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: \"\"\n      filename-regex: '^package\\.json$'\n      form: other\n      roles:\n        - inventory\n"))
	if err == nil {
		t.Fatalf("expected error for missing rule name")
	}
}

func TestLoadRulesRejectsInvalidRegex(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: js\n      filename-regex: '('\n      form: other\n      roles:\n        - inventory\n"))
	if err == nil {
		t.Fatalf("expected invalid regex error")
	}
}

func TestLoadRulesRejectsMissingSelectors(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: js\n      form: other\n      roles:\n        - inventory\n"))
	if err == nil {
		t.Fatalf("expected error for missing selectors")
	}
}

func TestLoadRulesAcceptsPathGlobSelector(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: js\n      path-glob: 'apps/**/package.json'\n      form: other\n      roles:\n        - inventory\n"))
	if err != nil {
		t.Fatalf("expected path glob rule to load: %v", err)
	}
}

func TestLoadRulesRejectsInvalidPathGlob(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: js\n      path-glob: 'apps/[.json'\n      form: other\n      roles:\n        - inventory\n"))
	if err == nil {
		t.Fatalf("expected invalid path glob error")
	}
}

func TestLoadRulesRejectsPathGlobWithEmptySegment(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: js\n      path-glob: 'apps//package.json'\n      form: other\n      roles:\n        - inventory\n"))
	if err == nil {
		t.Fatalf("expected invalid path glob with empty segment")
	}
}

func TestLoadRulesRejectsPathGlobWithInvalidRecursiveWildcardPlacement(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: js\n      path-glob: 'apps/**b/package.json'\n      form: other\n      roles:\n        - inventory\n"))
	if err == nil {
		t.Fatalf("expected invalid recursive wildcard placement")
	}
}

func TestLoadRulesAcceptsCombinedSelectors(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: js\n      filename-regex: '^package\\.json$'\n      path-glob: 'apps/**/package.json'\n      form: other\n      roles:\n        - inventory\n"))
	if err != nil {
		t.Fatalf("expected combined selector rule to load: %v", err)
	}
}

func TestScanMatchesPathGlobRequirementsAnywhere(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-requirements\n      path-glob: '**/requirements/*.txt'\n      form: other\n      roles:\n        - inventory\n"))
	if err != nil {
		t.Fatalf("expected path glob rule to load: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "requirements", "base.txt"), "")
	mustWriteFile(t, filepath.Join(root, "apps", "api", "requirements", "base.txt"), "")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 2 {
		t.Fatalf("expected 2 dependency sources, got %+v", result.Sources)
	}
	if result.Sources[0].Path != "apps/api/requirements/base.txt" || result.Sources[1].Path != "requirements/base.txt" {
		t.Fatalf("unexpected dependency sources: %+v", result.Sources)
	}
}

func TestScanMatchesQuestionMarkGlobSemantics(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-requirements\n      path-glob: 'apps/**/req?.txt'\n      form: other\n      roles:\n        - inventory\n"))
	if err != nil {
		t.Fatalf("expected path glob rule to load: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "api", "req1.txt"), "")
	mustWriteFile(t, filepath.Join(root, "apps", "api", "req12.txt"), "")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}
	if result.Sources[0].Path != "apps/api/req1.txt" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
}

func TestNormalizeRelativePathConvertsBackslashes(t *testing.T) {
	got := normalizeRelativePath(`apps\api\requirements\base.txt`)
	want := "apps/api/requirements/base.txt"
	if got != want {
		t.Fatalf("unexpected normalized path: got %q want %q", got, want)
	}
}

func TestScanMatchesPathGlobWithSlashNormalizedRelativePath(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-requirements\n      path-glob: 'apps/**/requirements/*.txt'\n      form: other\n      roles:\n        - inventory\n"))
	if err != nil {
		t.Fatalf("expected path glob rule to load: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "api", "requirements", "base.txt"), "")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}
	if result.Sources[0].Path != "apps/api/requirements/base.txt" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
}

func TestScanPathGlobDoesNotMatchNestedGrandchildren(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-requirements\n      path-glob: '**/requirements/*.txt'\n      form: other\n      roles:\n        - inventory\n"))
	if err != nil {
		t.Fatalf("expected path glob rule to load: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "api", "requirements", "nested", "base.txt"), "")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources for nested grandchild path, got %+v", result.Sources)
	}
}

func TestScanCombinedSelectorsRequireBothMatches(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-requirements\n      filename-regex: '^requirements\\.txt$'\n      path-glob: '**/requirements/*.txt'\n      form: other\n      roles:\n        - inventory\n"))
	if err != nil {
		t.Fatalf("expected combined selector rule to load: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "requirements.txt"), "")
	mustWriteFile(t, filepath.Join(root, "apps", "api", "requirements", "notes.txt"), "")
	mustWriteFile(t, filepath.Join(root, "apps", "api", "notes.txt"), "")
	mustWriteFile(t, filepath.Join(root, "apps", "api", "requirements", "requirements.txt"), "")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}
	if result.Sources[0].Path != "apps/api/requirements/requirements.txt" {
		t.Fatalf("unexpected dependency source path: %+v", result.Sources[0])
	}
}

func TestLoadRulesAcceptsBannerRegexParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: js-banner\n      filename-regex: '.*\\.js$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: banner-regex\n        pattern: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)\\s+v?(\\d+\\.\\d+\\.\\d+)'\n"))
	if err != nil {
		t.Fatalf("expected banner regex rule to load: %v", err)
	}
}

func TestLoadRulesAcceptsPyRequirementsParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-requirements\n      filename-regex: '^requirements\\.txt$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: py-requirements\n"))
	if err != nil {
		t.Fatalf("expected py-requirements rule to load: %v", err)
	}
}

func TestLoadRulesAcceptsYarnLockParserRule(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: js-yarn\n      filename-regex: '^yarn\\.lock$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yarn-lock\n"))
	if err != nil {
		t.Fatalf("expected yarn-lock rule to load: %v", err)
	}
}

func TestAnalyzeDependencySourceMatchesYarnLockParserRule(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	root := t.TempDir()
	filePath := filepath.Join(root, "yarn.lock")
	mustWriteFile(t, filePath, "# yarn lockfile v1\n")

	detectorID, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected yarn-lock parser-backed rule to match the file")
	}
	if detectorID != DetectorID("js-yarn") {
		t.Fatalf("unexpected type: %q", detectorID)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if present == nil || *present {
		t.Fatalf("expected presence=absent, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestLoadRulesAcceptsUVLockParser(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-uv\n      filename-regex: '^uv\\.lock$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: uv-lock\n"))
	if err != nil {
		t.Fatalf("expected uv-lock rule to load: %v", err)
	}

	if got := ruleset.DetectorIDs(); !slices.Equal(got, []DetectorID{DetectorID("python-uv")}) {
		t.Fatalf("unexpected supported types: %+v", got)
	}

	root := t.TempDir()
	filePath := filepath.Join(root, "uv.lock")
	mustWriteFile(t, filePath, "version = 1\n")

	detectorID, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "uv.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected uv-lock parser-backed rule to match the file")
	}
	if detectorID != DetectorID("python-uv") {
		t.Fatalf("unexpected type: %q", detectorID)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if present == nil || *present {
		t.Fatalf("expected presence=absent, got %+v", present)
	}
	if len(diagnosticMessages) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestLoadRulesAcceptsPoetryLockParser(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-poetry-lock\n      filename-regex: '^poetry\\.lock$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: poetry-lock\n"))
	if err != nil {
		t.Fatalf("expected poetry-lock rule to load: %v", err)
	}

	if got := ruleset.DetectorIDs(); !slices.Equal(got, []DetectorID{DetectorID("python-poetry-lock")}) {
		t.Fatalf("unexpected supported types: %+v", got)
	}

	root := t.TempDir()
	filePath := filepath.Join(root, "poetry.lock")
	mustWriteFile(t, filePath, "[metadata]\nlock-version = \"2.1\"\n")

	detectorID, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "poetry.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected poetry-lock parser-backed rule to match the file")
	}
	if detectorID != DetectorID("python-poetry-lock") {
		t.Fatalf("unexpected type: %q", detectorID)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if present == nil || *present {
		t.Fatalf("expected presence=absent, got %+v", present)
	}
	if len(diagnosticMessages) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestDefaultRulesScanPoetryLockBasicFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "poetry-lock-basic"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Path != "poetry.lock" {
		t.Fatalf("unexpected dependency source path: %+v", source)
	}
	if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"requests==2.32.3", "urllib3==2.2.2"}) {
		t.Fatalf("unexpected dependencies: got %+v", got)
	}
	if source.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", source.Analysis)
	}
}

func TestDefaultRulesScanPoetryLockEmptyFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "poetry-lock-empty-metadata-only"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Path != "poetry.lock" {
		t.Fatalf("unexpected dependency source path: %+v", source)
	}
	if source.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", source.Dependencies)
	}
	if source.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", source.Analysis)
	}
}

func TestDefaultRulesScanPoetryLockFilteredFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "poetry-lock-git-dependency"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Path != "poetry.lock" {
		t.Fatalf("unexpected dependency source path: %+v", source)
	}
	if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"internal-lib", "requests==2.32.3"}) {
		t.Fatalf("unexpected dependencies: got %+v", got)
	}
	if source.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", source.Analysis)
	}
}

func TestDefaultRulesScanPoetryLockIgnoredSelfFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "poetry-lock-path-dependency-self"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Path != "poetry.lock" {
		t.Fatalf("unexpected dependency source path: %+v", source)
	}
	if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"requests==2.32.3"}) {
		t.Fatalf("unexpected dependencies: got %+v", got)
	}
	if source.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", source.Analysis)
	}
}

func TestDefaultRulesScanUVLockExtractedFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "uv-lock-extracted"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Path != "uv.lock" {
		t.Fatalf("unexpected dependency source path: %+v", source)
	}
	if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"requests==2.32.3", "local-lib"}) {
		t.Fatalf("unexpected dependencies: got %+v", got)
	}
	if source.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", source.Analysis)
	}
}

func TestDefaultRulesScanUVLockEmptyFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "uv-lock-empty"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Path != "uv.lock" {
		t.Fatalf("unexpected dependency source path: %+v", source)
	}
	if source.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", source.Dependencies)
	}
	if source.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", source.Analysis)
	}
}

func TestDefaultRulesScanUVLockFilteredEmptyFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "uv-lock-filtered-empty"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Path != "uv.lock" {
		t.Fatalf("unexpected dependency source path: %+v", source)
	}
	if source.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", source.Dependencies)
	}
	if source.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", source.Analysis)
	}
}

func TestDefaultRulesScanUVLockIgnoredSelfFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "uv-lock-ignored-self"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Path != "uv.lock" {
		t.Fatalf("unexpected dependency source path: %+v", source)
	}
	if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"requests==2.32.3", "local-lib"}) {
		t.Fatalf("unexpected dependencies: got %+v", got)
	}
	if source.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", source.Analysis)
	}
}

func TestLoadRulesRejectsInvalidBannerRegex(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: js-banner\n      filename-regex: '.*\\.js$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: banner-regex\n        pattern: '('\n"))
	if err == nil {
		t.Fatalf("expected invalid banner regex error")
	}
}

func TestLoadRulesRejectsBannerRegexWithoutRequiredCaptureGroups(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: js-banner\n      filename-regex: '.*\\.js$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: banner-regex\n        pattern: '^/\\*!\\s*[A-Za-z0-9._/-]+\\s+v?\\d+\\.\\d+\\.\\d+'\n"))
	if err == nil {
		t.Fatalf("expected missing capture group error")
	}
}

func TestLoadRulesRejectsTerraformParserWithoutResourceType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: terraform.aws_glue_job.python\n      filename-regex: '.*\\.tf$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: terraform\n        conditions:\n            - path: default_arguments.--job-language\n              equals: python\n"))
	if err == nil {
		t.Fatalf("expected missing resource type error")
	}
}

func TestLoadRulesAcceptsTypeScriptParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: typescript.cdk.aws_glue_job.python\n      filename-regex: '.*\\.ts$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: typescript\n        cdk_construct:\n            module: aws-cdk-lib/aws-glue\n            construct: CfnJob\n            props_argument_index: 2\n            within:\n                - defaultArguments\n            conditions:\n                - key: --additional-python-modules\n                  present: true\n            extract:\n                key: --additional-python-modules\n                split: comma\n"))
	if err != nil {
		t.Fatalf("expected typescript parser to load: %v", err)
	}
}

func TestLoadRulesAcceptsPythonParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python.cdk.aws_glue_job.python\n      filename-regex: '.*\\.py$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: python\n        cdk_construct:\n            module: aws_cdk.aws_glue\n            construct: CfnJob\n            keyword_argument: default_arguments\n            conditions:\n                - key: --additional-python-modules\n                  present: true\n            extract:\n                key: --additional-python-modules\n                split: comma\n"))
	if err != nil {
		t.Fatalf("expected python parser to load: %v", err)
	}
}

func TestLoadRulesAcceptsGenericPythonCallParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-setup-py\n      filename-regex: '^setup\\.py$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: python\n        call:\n            module: setuptools\n            function: setup\n            conditions:\n                any_of:\n                    - keyword: install_requires\n                      present: true\n                    - keyword: extras_require\n                      present: true\n            extract:\n                - keyword: install_requires\n                  literal: string_list\n                - keyword: extras_require\n                  literal: dict_string_lists\n"))
	if err != nil {
		t.Fatalf("expected generic python call parser to load: %v", err)
	}
}

func TestLoadRulesRejectsPythonCallParserWithUnsupportedLiteralExtractor(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-setup-py\n      filename-regex: '^setup\\.py$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: python\n        call:\n            module: setuptools\n            function: setup\n            conditions:\n                any_of:\n                    - keyword: install_requires\n                      present: true\n            extract:\n                - keyword: install_requires\n                  literal: string_tuple\n"))
	if err == nil {
		t.Fatalf("expected unsupported literal extractor error")
	}
}

func TestLoadRulesAcceptsHTMLParser(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: html-external-scripts\n      filename-regex: '.*\\.html$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: html\n        external_scripts: true\n"))
	if err != nil {
		t.Fatalf("expected html rule to load: %v", err)
	}
	if got := ruleset.DetectorIDs(); !slices.Equal(got, []DetectorID{DetectorID("html-external-scripts")}) {
		t.Fatalf("unexpected supported types: %+v", got)
	}
}

func TestLoadRulesRejectsHTMLParserWithoutExternalScripts(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: html-external-scripts\n      filename-regex: '.*\\.html$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: html\n"))
	if err == nil {
		t.Fatalf("expected missing html parser configuration error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithoutModule(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: typescript.cdk.aws_glue_job.python\n      filename-regex: '.*\\.ts$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: typescript\n        cdk_construct:\n            construct: CfnJob\n            props_argument_index: 2\n            conditions:\n                - key: --additional-python-modules\n                  present: true\n"))
	if err == nil {
		t.Fatalf("expected missing module error")
	}
}

func TestLoadRulesRejectsPythonParserWithoutKeywordArgument(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python.cdk.aws_glue_job.python\n      filename-regex: '.*\\.py$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: python\n        cdk_construct:\n            module: aws_cdk.aws_glue\n            construct: CfnJob\n            conditions:\n                - key: --additional-python-modules\n                  present: true\n"))
	if err == nil {
		t.Fatalf("expected missing keyword argument error")
	}
}

func TestScanDoesNotMatchSetupPyWithoutTargetKeywords(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-setup-py\n      filename-regex: '^setup\\.py$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: python\n        call:\n            module: setuptools\n            function: setup\n            conditions:\n                any_of:\n                    - keyword: install_requires\n                      present: true\n                    - keyword: extras_require\n                      present: true\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "setup.py"), `
from setuptools import setup

setup(
    name="sample",
    version="0.1.0",
)
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestLoadRulesRejectsTypeScriptParserWithoutConstruct(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: typescript.cdk.aws_glue_job.python\n      filename-regex: '.*\\.ts$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: typescript\n        cdk_construct:\n            module: aws-cdk-lib/aws-glue\n            props_argument_index: 2\n            conditions:\n                - key: --additional-python-modules\n                  present: true\n"))
	if err == nil {
		t.Fatalf("expected missing construct error")
	}
}

func TestLoadRulesRejectsTerraformParserWithoutConditions(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: terraform.aws_glue_job.python\n      filename-regex: '.*\\.tf$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: terraform\n        resource_type: aws_glue_job\n"))
	if err == nil {
		t.Fatalf("expected missing conditions error")
	}
}

func TestLoadRulesRejectsTerraformConditionWithoutMatcher(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: terraform.aws_glue_job.python\n      filename-regex: '.*\\.tf$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: terraform\n        resource_type: aws_glue_job\n        conditions:\n            - path: default_arguments.--job-language\n"))
	if err == nil {
		t.Fatalf("expected invalid terraform condition error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithoutPropsArgumentIndex(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: typescript.cdk.aws_glue_job.python\n      filename-regex: '.*\\.ts$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: typescript\n        cdk_construct:\n            module: aws-cdk-lib/aws-glue\n            construct: CfnJob\n            conditions:\n                - key: --additional-python-modules\n                  present: true\n"))
	if err == nil {
		t.Fatalf("expected missing props argument index error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithoutConditions(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: typescript.cdk.aws_glue_job.python\n      filename-regex: '.*\\.ts$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: typescript\n        cdk_construct:\n            module: aws-cdk-lib/aws-glue\n            construct: CfnJob\n            props_argument_index: 2\n"))
	if err == nil {
		t.Fatalf("expected missing typescript conditions error")
	}
}

func TestLoadRulesRejectsPythonParserWithoutConditions(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python.cdk.aws_glue_job.python\n      filename-regex: '.*\\.py$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: python\n        cdk_construct:\n            module: aws_cdk.aws_glue\n            construct: CfnJob\n            keyword_argument: default_arguments\n"))
	if err == nil {
		t.Fatalf("expected missing python conditions error")
	}
}

func TestLoadRulesRejectsTypeScriptConditionWithoutMatcher(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: typescript.cdk.aws_glue_job.python\n      filename-regex: '.*\\.ts$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: typescript\n        cdk_construct:\n            module: aws-cdk-lib/aws-glue\n            construct: CfnJob\n            props_argument_index: 2\n            conditions:\n                - key: --additional-python-modules\n"))
	if err == nil {
		t.Fatalf("expected invalid typescript condition error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithUnsupportedExtractSplit(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: typescript.cdk.aws_glue_job.python\n      filename-regex: '.*\\.ts$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: typescript\n        cdk_construct:\n            module: aws-cdk-lib/aws-glue\n            construct: CfnJob\n            props_argument_index: 2\n            conditions:\n                - key: --additional-python-modules\n                  present: true\n            extract:\n                key: --additional-python-modules\n                split: space\n"))
	if err == nil {
		t.Fatalf("expected invalid extract split error")
	}
}

func TestLoadRulesRejectsPythonParserWithUnsupportedExtractSplit(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python.cdk.aws_glue_job.python\n      filename-regex: '.*\\.py$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: python\n        cdk_construct:\n            module: aws_cdk.aws_glue\n            construct: CfnJob\n            keyword_argument: default_arguments\n            conditions:\n                - key: --additional-python-modules\n                  present: true\n            extract:\n                key: --additional-python-modules\n                split: space\n"))
	if err == nil {
		t.Fatalf("expected invalid python extract split error")
	}
}

func TestLoadRulesAcceptsYAMLParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: yaml-pip\n      filename-regex: '.*\\.ya?ml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("expected yaml parser to load: %v", err)
	}
}

func TestLoadRulesAcceptsYAMLExistsParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: conda-environment\n      filename-regex: '^environment\\.ya?ml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        exists: dependencies\n"))
	if err != nil {
		t.Fatalf("expected yaml exists parser to load: %v", err)
	}
}

func TestLoadRulesSupportsTOMLTableQueriesAndExcludeKeys(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pipfile\n      filename-regex: '^Pipfile$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        table-queries:\n            - '*'\n        exclude-keys:\n            - source\n            - requires\n"))
	if err != nil {
		t.Fatalf("expected generic toml table config to load: %v", err)
	}
}

func TestLoadRulesAcceptsTOMLExistsAnyParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pdm-lock\n      filename-regex: '^pdm\\.lock$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        exists-any:\n            - package\n"))
	if err != nil {
		t.Fatalf("expected generic toml exists-any config to load: %v", err)
	}
}

func TestLoadRulesRejectsYAMLParserWithoutQuery(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: yaml-pip\n      filename-regex: '.*\\.ya?ml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n"))
	if err == nil {
		t.Fatalf("expected missing yaml query error")
	}
}

func TestLoadRulesRejectsYAMLParserWithQueryAndExists(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: conda-environment\n      filename-regex: '^environment\\.ya?ml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        query: dependencies[]\n        exists: dependencies\n"))
	if err == nil {
		t.Fatalf("expected mutually exclusive yaml query and exists error")
	}
}

func TestLoadRulesRejectsMalformedYAMLQuery(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: yaml-pip\n      filename-regex: '.*\\.ya?ml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        query: workflow..steps[].config.packages.pip[]\n"))
	if err == nil {
		t.Fatalf("expected malformed yaml query error")
	}
}

func TestLoadRulesRejectsMalformedYAMLExistsPath(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: conda-environment\n      filename-regex: '^environment\\.ya?ml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        exists: dependencies..\n"))
	if err == nil {
		t.Fatalf("expected malformed yaml exists path error")
	}
}

func TestLoadRulesAcceptsTOMLParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pyproject\n      filename-regex: '^pyproject\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        queries:\n            - project.dependencies[]\n            - project.optional-dependencies.*[]\n"))
	if err != nil {
		t.Fatalf("expected toml parser to load: %v", err)
	}
}

func TestLoadRulesRejectsTOMLParserWithoutQueries(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pyproject\n      filename-regex: '^pyproject\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n"))
	if err == nil {
		t.Fatalf("expected missing toml queries error")
	}
}

func TestLoadRulesRejectsMalformedTOMLQuery(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pyproject\n      filename-regex: '^pyproject\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        queries:\n            - project..dependencies[]\n"))
	if err == nil {
		t.Fatalf("expected malformed toml query error")
	}
}

func TestLoadRulesRejectsMalformedTOMLExistsAnyQuery(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pdm-lock\n      filename-regex: '^pdm\\.lock$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        exists-any:\n            - package..\n"))
	if err == nil {
		t.Fatalf("expected malformed toml exists-any query error")
	}
}

func TestLoadRulesRejectsTOMLParserWithOtherParserType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: mixed\n      filename-regex: '^pyproject\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        query: workflow.steps[].config.packages.pip[]\n      analyzer:\n        type: toml\n        queries:\n            - project.dependencies[]\n"))
	if err == nil {
		t.Fatalf("expected multiple parser type error")
	}
}

func TestLoadRulesAcceptsXMLParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: java\n      filename-regex: '^pom\\.xml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: xml\n        exists-any:\n            - project.dependencies.dependency\n"))
	if err != nil {
		t.Fatalf("expected xml parser to load: %v", err)
	}
}

func TestLoadRulesRejectsXMLParserWithoutQueries(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: java\n      filename-regex: '^pom\\.xml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: xml\n"))
	if err == nil {
		t.Fatalf("expected missing xml queries error")
	}
}

func TestLoadRulesRejectsMalformedXMLQuery(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: java\n      filename-regex: '^pom\\.xml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: xml\n        exists-any:\n            - project..dependencies.dependency\n"))
	if err == nil {
		t.Fatalf("expected malformed xml query error")
	}
}

func TestLoadRulesAcceptsINIParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-setup-cfg\n      filename-regex: '^setup\\.cfg$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: ini\n        queries:\n            - section: options\n              key: install_requires\n            - section: options.extras_require\n              key: '*'\n"))
	if err != nil {
		t.Fatalf("expected ini parser to load: %v", err)
	}
}

func TestLoadRulesRejectsINIParserWithoutQueries(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-setup-cfg\n      filename-regex: '^setup\\.cfg$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: ini\n"))
	if err == nil {
		t.Fatalf("expected missing ini queries error")
	}
}

func TestLoadRulesRejectsINIQueryWithoutSection(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-setup-cfg\n      filename-regex: '^setup\\.cfg$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: ini\n        queries:\n            - key: install_requires\n"))
	if err == nil {
		t.Fatalf("expected missing ini section error")
	}
}

func TestLoadRulesRejectsINIQueryWithoutKey(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-setup-cfg\n      filename-regex: '^setup\\.cfg$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: ini\n        queries:\n            - section: options\n"))
	if err == nil {
		t.Fatalf("expected missing ini key error")
	}
}

func TestLoadRulesRejectsINIParserWithOtherParserType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: mixed\n      filename-regex: '^setup\\.cfg$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: ini\n        queries:\n            - section: options\n              key: install_requires\n      analyzer:\n        type: toml\n        queries:\n            - project.dependencies[]\n"))
	if err == nil {
		t.Fatalf("expected multiple parser type error")
	}
}

func TestLoadRulesRejectsMultipleParserTypes(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: mixed\n      filename-regex: '.*'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: terraform\n        resource_type: aws_glue_job\n        conditions:\n            - path: default_arguments.--job-language\n              equals: python\n      analyzer:\n        type: yaml\n        query: workflow.steps[].config.packages.pip[]\n"))
	if err == nil {
		t.Fatalf("expected multiple parser type error")
	}
}

func TestLoadRulesRejectsBannerRegexWithOtherParserType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: mixed\n      filename-regex: '.*\\.js$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: banner-regex\n        pattern: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)\\s+v?(\\d+\\.\\d+\\.\\d+)'\n      analyzer:\n        type: html\n        external_scripts: true\n"))
	if err == nil {
		t.Fatalf("expected multiple parser type error")
	}
}

func TestLoadRulesSupportsCustomFirstMatchOrdering(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: broad\n      filename-regex: '.*\\.json$'\n      form: other\n      roles:\n        - inventory\n    - id: specific\n      filename-regex: '^package\\.json$'\n      form: other\n      roles:\n        - inventory\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	got, ok := ruleset.MatchSelectorOnlySource("package.json")
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("broad") {
		t.Fatalf("expected first pattern to win, got %q", got)
	}
}

func TestMatchSelectorOnlySourceIgnoresPackageLockParserRule(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	if _, ok := ruleset.MatchSelectorOnlySource("package-lock.json"); ok {
		t.Fatalf("expected selector-only detection to ignore package-lock parser rule")
	}
}

func TestMatchSelectorOnlySourceIgnoresCargoLockParserRule(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	if _, ok := ruleset.MatchSelectorOnlySource("Cargo.lock"); ok {
		t.Fatalf("expected selector-only detection to ignore cargo-lock parser rule")
	}
}

func TestMatchSelectorOnlySourceIgnoresPodfileLockParserRule(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	if _, ok := ruleset.MatchSelectorOnlySource("Podfile.lock"); ok {
		t.Fatalf("expected selector-only detection to ignore Podfile.lock parser rule")
	}
}

func TestMatchSelectorOnlySourceIgnoresGopkgLockParserRule(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	if _, ok := ruleset.MatchSelectorOnlySource("Gopkg.lock"); ok {
		t.Fatalf("expected selector-only detection to ignore Gopkg.lock parser rule")
	}
}

func TestMatchSelectorOnlySourceIgnoresClojureDepsEDNAnalyzerRule(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	if _, ok := ruleset.MatchSelectorOnlySource("deps.edn"); ok {
		t.Fatalf("expected selector-only detection to ignore deps.edn analyzer rule")
	}
}

func TestLoadDefaultRulesProvidesSupportedTypeOrder(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	want := []DetectorID{
		DetectorID("python-requirements"),
		DetectorID("python-requirements-dir"),
		DetectorID("python-uv"),
		DetectorID("python-poetry-lock"),
		DetectorID("python-pipfile-lock"),
		DetectorID("python-pdm-lock"),
		DetectorID("python-conda-lock"),
		DetectorID("python-conda-env-alt"),
		DetectorID("python-pyproject"),
		DetectorID("python-conda-environment"),
		DetectorID("python-pipfile"),
		DetectorID("python-setup-py"),
		DetectorID("python-setup-cfg"),
		DetectorID("python-constraints"),
		DetectorID("js"),
		DetectorID("js-bower"),
		DetectorID("js-npm-shrinkwrap"),
		DetectorID("js-npm-lock"),
		DetectorID("js-yarn"),
		DetectorID("js-pnpm-lock"),
		DetectorID("js-bun-lock"),
		DetectorID("js-bun-lockb"),
		DetectorID("deno-lock"),
		DetectorID("deno-json"),
		DetectorID("deno-jsonc"),
		DetectorID("js-pnp"),
		DetectorID("js-pnpm-workspace"),
		DetectorID("js-npmrc"),
		DetectorID("js-yarnrc"),
		DetectorID("js-importmap"),
		DetectorID("java"),
		DetectorID("java-gradle-lockfile"),
		DetectorID("java-gradle"),
		DetectorID("java-gradle-kts"),
		DetectorID("java-gradle-settings"),
		DetectorID("java-gradle-settings-kts"),
		DetectorID("java-gradle-version-catalog"),
		DetectorID("java-gradle-wrapper"),
		DetectorID("scala-sbt-build"),
		DetectorID("scala-sbt-plugins"),
		DetectorID("scala-sbt-dependencies"),
		DetectorID("scala-sbt-build-props"),
		DetectorID("scala-mill"),
		DetectorID("java-ant-build"),
		DetectorID("java-ivy"),
		DetectorID("java-ivy-settings"),
		DetectorID("ruby-gemfile"),
		DetectorID("ruby-gemfile-lock"),
		DetectorID("ruby-gemspec"),
		DetectorID("ruby-appraisal"),
		DetectorID("swift-package"),
		DetectorID("ios-podfile"),
		DetectorID("ios-cartfile"),
		DetectorID("ios-podspec"),
		DetectorID("ios-cartfile-resolved"),
		DetectorID("php-composer"),
		DetectorID("php-composer-lock"),
		DetectorID("dart-pubspec"),
		DetectorID("dart-pubspec-lock"),
		DetectorID("erlang-rebar-config"),
		DetectorID("erlang-rebar-lock"),
		DetectorID("clojure-deps-edn"),
		DetectorID("clojure-project-clj"),
		DetectorID("clojure-boot"),
		DetectorID("haskell-stack"),
		DetectorID("haskell-stack-lock"),
		DetectorID("haskell-cabal-project"),
		DetectorID("haskell-cabal-project-freeze"),
		DetectorID("haskell-cabal"),
		DetectorID("haskell-package-yaml"),
		DetectorID("dotnet-packages-config"),
		DetectorID("dotnet-packages-lock"),
		DetectorID("dotnet-directory-packages-props"),
		DetectorID("dotnet-paket-dependencies"),
		DetectorID("dotnet-paket-lock"),
		DetectorID("dotnet-fsproj"),
		DetectorID("dotnet-vbproj"),
		DetectorID("dotnet-directory-build"),
		DetectorID("dotnet-paket-references"),
		DetectorID("dotnet-tools-manifest"),
		DetectorID("go-mod"),
		DetectorID("go-sum"),
		DetectorID("go-work"),
		DetectorID("go-gopkg-toml"),
		DetectorID("go-glide-yaml"),
		DetectorID("go-godep"),
		DetectorID("rust-cargo"),
		DetectorID("rust-cargo-lock"),
		DetectorID("rust-cargo-config"),
		DetectorID("go-gopkg-lock"),
		DetectorID("go-glide-lock"),
		DetectorID("dotnet-csproj"),
		DetectorID("cpp-conanfile"),
		DetectorID("cpp-conan-lock"),
		DetectorID("cpp-vcpkg"),
		DetectorID("cpp-cmake"),
		DetectorID("cpp-conanfile-py"),
		DetectorID("cpp-vcpkg-config"),
		DetectorID("cpp-meson"),
		DetectorID("cpp-autotools"),
		DetectorID("cpp-cmake-modules"),
		DetectorID("cpp-meson-wrap"),
		// DRAFT (Group 3): DetectorID("cpp-gclient-deps"),
		DetectorID("swift-package-resolved"),
		DetectorID("ios-podfile-lock"),
		DetectorID("elixir-mix"),
		DetectorID("elixir-mix-lock"),
		DetectorID("julia-project"),
		DetectorID("julia-manifest"),
		DetectorID("perl-cpanfile"),
		DetectorID("perl-cpanfile-snapshot"),
		DetectorID("perl-makefile-pl"),
		DetectorID("perl-build-pl"),
		DetectorID("perl-meta"),
		DetectorID("perl-dist-ini"),
		DetectorID("raku-meta"),
		DetectorID("r-renv-lock"),
		DetectorID("r-packrat-lock"),
		// DRAFT (Group 3): DetectorID("r-description"),
		DetectorID("lua-rockspec"),
		DetectorID("zig-build-zon"),
		DetectorID("zig-build"),
		DetectorID("nim-nimble"),
		DetectorID("ocaml-opam"),
		DetectorID("ocaml-opam-locked"),
		DetectorID("ocaml-dune-project"),
		DetectorID("ocaml-esy"),
		// DRAFT (Group 3): DetectorID("ocaml-dune"),
		DetectorID("crystal-shard"),
		DetectorID("crystal-shard-lock"),
		DetectorID("gleam"),
		DetectorID("gleam-manifest"),
		DetectorID("fortran-fpm"),
		DetectorID("vlang"),
		DetectorID("helm-chart"),
		DetectorID("ansible-requirements"),
		DetectorID("buf"),
		DetectorID("homebrew-brewfile"),
		DetectorID("jsonnet-bundler"),
		DetectorID("terraform-lock"),
		DetectorID("unity-packages-manifest"),
		DetectorID("unity-packages-lock"),
		DetectorID("docker-dockerfile"),
		DetectorID("docker-compose"),
		DetectorID("github-actions-action"),
		DetectorID("github-actions-workflow"),
		DetectorID("bazel-workspace"),
		DetectorID("bazel-module"),
		DetectorID("bazel-module-lock"),
		DetectorID("bazel-build-file"),
		DetectorID("bazel-third-party-bzl"),
		DetectorID("js-nx"),
		// DRAFT (Group 3): DetectorID("js-nx-project"),
		DetectorID("js-lerna"),
		DetectorID("js-rush"),
		DetectorID("rush-common-versions"),
		DetectorID("js-turbo"),
		DetectorID("pants-config"),
		DetectorID("pants-jvm-build"),
		// DRAFT (Group 3): DetectorID("bazel-build-file-bare"),
		DetectorID("git-submodules"),
		DetectorID("nix-default-shell"),
		DetectorID("nix-flake"),
		DetectorID("nix-flake-lock"),
		DetectorID("helm-chart-lock"),
		DetectorID("homebrew-brewfile-lock"),
		DetectorID("buf-lock"),
		DetectorID("puppet-puppetfile"),
		// DRAFT (Group 3): DetectorID("puppet-cookbook-metadata"),
		DetectorID("chef-berksfile"),
		DetectorID("chef-berksfile-lock"),
		DetectorID("chef-metadata"),
		DetectorID("chef-policyfile"),
		DetectorID("chef-policyfile-lock"),
		DetectorID("jsonnet-lock"),
		DetectorID("emacs-cask"),
		DetectorID("unreal-uproject"),
		DetectorID("unreal-uplugin"),
		DetectorID("godot-plugin-cfg"),
		DetectorID("foundry-toml"),
		DetectorID("foundry-remappings"),
		DetectorID("soldeer-lock"),
		DetectorID("js-banner-block-start"),
		DetectorID("js-banner-plain-block-start"),
		DetectorID("js-banner-multiline-preserved"),
		DetectorID("js-banner-line-comment"),
		DetectorID("js-banner-version-tagged"),
		DetectorID("html-external-scripts"),
		DetectorID("terraform.aws_glue_job.python"),
		DetectorID("typescript.cdk.aws_glue_job.python"),
		DetectorID("python.cdk.aws_glue_job.python"),
	}
	got := ruleset.DetectorIDs()
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected supported type order: got %v want %v", got, want)
	}
}

func TestScanMatchesYAMLDependenciesFromCustomRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: yaml-pip\n      filename-regex: '^workflow\\.yaml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "workflow.yaml"), `
workflow:
  steps:
    - name: step1
      config:
        packages:
          pip:
            - requests
            - pendulum
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if !slices.Equal(dependencyNames(result.Sources[0].Dependencies), []string{"requests", "pendulum"}) {
		t.Fatalf("unexpected dependencies: %+v", result.Sources[0].Dependencies)
	}
}

func TestScanMatchesYAMLExistsRuleWithoutExtractingDependencies(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: conda-environment\n      filename-regex: '^environment\\.ya?ml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        exists: dependencies\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "environment.yml"), `
name: app
dependencies:
  - python=3.12
  - pip
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("conda-environment") || result.Sources[0].Path != "environment.yml" {
		t.Fatalf("unexpected dependency source: %+v", result.Sources[0])
	}
	if result.Sources[0].Dependencies != nil {
		t.Fatalf("expected no extracted dependencies, got %+v", result.Sources[0].Dependencies)
	}
}

func TestScanMatchesYAMLExistsAnyRuleWithDependenciesPresent(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: dart-pubspec\n      filename-regex: '^pubspec\\.yaml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        exists-any:\n            - dependencies\n            - dev_dependencies\n            - dependency_overrides\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pubspec.yaml"), `
name: app
dependencies:
  http: ^1.2.0
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
	if result.Sources[0].Dependencies != nil {
		t.Fatalf("expected no extracted dependencies, got %+v", result.Sources[0].Dependencies)
	}
}

func TestScanMatchesYAMLExistsAnyRuleWithoutDependencies(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: dart-pubspec\n      filename-regex: '^pubspec\\.yaml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        exists-any:\n            - dependencies\n            - dev_dependencies\n            - dependency_overrides\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pubspec.yaml"), `
name: app
environment:
  sdk: ^3.4.0
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesBufYAMLWithDeps(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "buf", "module-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("buf") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMarksBufYAMLEmptyDepsAsNoDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "buf", "module-empty-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("buf") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMarksBufYAMLWithoutDepsKeyAsNoDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "buf", "module-no-deps-key"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("buf") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesAnsibleRequirementsWithRoles(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "ansible", "requirements-roles"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("ansible-requirements") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesAnsibleRequirementsWithCollections(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "ansible", "requirements-collections"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("ansible-requirements") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesAnsibleRequirementsWithRolesAndCollections(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "ansible", "requirements-both"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("ansible-requirements") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMarksAnsibleRequirementsEmptyListsAsNoDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "ansible", "requirements-empty"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("ansible-requirements") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMarksAnsibleRequirementsWithoutKeysAsNoDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "ansible", "requirements-missing"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("ansible-requirements") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesJsonnetBundlerWithDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "jsonnet", "bundler-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("jsonnet-bundler") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMarksJsonnetBundlerEmptyDependenciesAsNoDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "jsonnet", "bundler-empty"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("jsonnet-bundler") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMarksJsonnetBundlerWithoutDependenciesKeyAsNoDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "jsonnet", "bundler-missing"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("jsonnet-bundler") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMarksJsonnetBundlerWrongTypeAsNoDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "jsonnet", "bundler-wrong-type"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("jsonnet-bundler") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesHaskellPackageYAMLWithDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "haskell", "package-yaml"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("haskell-package-yaml") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
	if result.Sources[0].Dependencies != nil {
		t.Fatalf("expected no extracted dependencies, got %+v", result.Sources[0].Dependencies)
	}
}

func TestScanMarksHaskellPackageYAMLWithoutDependenciesAsNoDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "haskell", "package-yaml-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("haskell-package-yaml") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesVcpkgWithDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "cpp", "vcpkg"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("cpp-vcpkg") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
	want := []DependencyReference{
		{PackageType: "vcpkg", Raw: "fmt", Name: "fmt", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
		{PackageType: "vcpkg", Raw: "openssl", Name: "openssl", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
	}
	if !reflect.DeepEqual(result.Sources[0].Dependencies, want) {
		t.Fatalf("dependencies = %#v, want %#v", result.Sources[0].Dependencies, want)
	}
}

func TestScanMarksVcpkgWithoutDependenciesAsNoDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "cpp", "vcpkg-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("cpp-vcpkg") {
		t.Fatalf("unexpected dependency source type: got %q", result.Sources[0].Detector)
	}
	if want := (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}); result.Sources[0].Analysis != want {
		t.Fatalf("analysis = %+v, want %+v", result.Sources[0].Analysis, want)
	}
	if len(result.Sources[0].Dependencies) != 0 {
		t.Fatalf("expected no extracted dependencies, got %+v", result.Sources[0].Dependencies)
	}
}

func TestScanMatchesYAMLDependenciesFromPathGlobRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: yaml-pip\n      path-glob: '**/pipelines/workflow.yaml'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "api", "pipelines", "workflow.yaml"), `
workflow:
  steps:
    - name: step1
      config:
        packages:
          pip:
            - requests
            - pendulum
`)
	mustWriteFile(t, filepath.Join(root, "apps", "api", "workflow.yaml"), `
workflow:
  steps:
    - name: step1
      config:
        packages:
          pip:
            - should-not-match
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	source := result.Sources[0]
	if source.Detector != DetectorID("yaml-pip") || source.Path != "apps/api/pipelines/workflow.yaml" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
	if !slices.Equal(dependencyNames(source.Dependencies), []string{"requests", "pendulum"}) {
		t.Fatalf("unexpected dependencies: %+v", source.Dependencies)
	}
}

func TestScanMatchesTOMLDependenciesFromPathGlobRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pyproject\n      path-glob: '**/pyproject.toml'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        queries:\n            - project.dependencies[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "services", "api", "pyproject.toml"), `
[project]
dependencies = ["requests>=2.31"]
`)
	mustWriteFile(t, filepath.Join(root, "services", "api", "other.toml"), `
[project]
dependencies = ["should-not-match"]
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	source := result.Sources[0]
	if source.Detector != DetectorID("python-pyproject") || source.Path != "services/api/pyproject.toml" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
	if !slices.Equal(dependencyNames(source.Dependencies), []string{"requests>=2.31"}) {
		t.Fatalf("unexpected dependencies: %+v", source.Dependencies)
	}
}

func TestScanMatchesTOMLDependenciesFromCombinedSelectors(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pyproject\n      filename-regex: '^pyproject\\.toml$'\n      path-glob: '**/pipelines/pyproject.toml'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        queries:\n            - project.dependencies[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "api", "pipelines", "pyproject.toml"), `
[project]
dependencies = ["requests>=2.31"]
`)
	mustWriteFile(t, filepath.Join(root, "apps", "api", "pyproject.toml"), `
[project]
dependencies = ["wrong-path"]
`)
	mustWriteFile(t, filepath.Join(root, "apps", "api", "pipelines", "not-pyproject.toml"), `
[project]
dependencies = ["wrong-name"]
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}
	source := result.Sources[0]
	if source.Detector != DetectorID("python-pyproject") || source.Path != "apps/api/pipelines/pyproject.toml" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
	if !slices.Equal(dependencyNames(source.Dependencies), []string{"requests>=2.31"}) {
		t.Fatalf("unexpected dependencies: %+v", source.Dependencies)
	}
}

func TestScanMatchesYAMLDependenciesAcrossNestedLists(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: yaml-pip\n      filename-regex: '^workflow\\.yaml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "workflow.yaml"), `
workflow:
  steps:
    - name: step1
      config:
        packages:
          pip:
            - requests
    - name: step2
      config:
        packages:
          pip:
            - pendulum
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if !slices.Equal(dependencyNames(result.Sources[0].Dependencies), []string{"requests", "pendulum"}) {
		t.Fatalf("unexpected dependencies: %+v", result.Sources[0].Dependencies)
	}
}

func TestScanMatchesYAMLDependenciesFromCombinedSelectors(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: yaml-pip\n      filename-regex: '^workflow\\.yaml$'\n      path-glob: '**/pipelines/workflow.yaml'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "api", "pipelines", "workflow.yaml"), `
workflow:
  steps:
    - name: step1
      config:
        packages:
          pip:
            - requests
            - pendulum
`)
	mustWriteFile(t, filepath.Join(root, "apps", "api", "workflow.yaml"), `
workflow:
  steps:
    - name: step1
      config:
        packages:
          pip:
            - wrong-path
`)
	mustWriteFile(t, filepath.Join(root, "apps", "api", "pipelines", "other.yaml"), `
workflow:
  steps:
    - name: step1
      config:
        packages:
          pip:
            - wrong-name
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}
	source := result.Sources[0]
	if source.Detector != DetectorID("yaml-pip") || source.Path != "apps/api/pipelines/workflow.yaml" {
		t.Fatalf("unexpected dependency source: %+v", source)
	}
	if !slices.Equal(dependencyNames(source.Dependencies), []string{"requests", "pendulum"}) {
		t.Fatalf("unexpected dependencies: %+v", source.Dependencies)
	}
}

func TestScanDoesNotMatchYAMLWhenQueryResolvesToNonStrings(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: yaml-pip\n      filename-regex: '^workflow\\.yaml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "workflow.yaml"), `
workflow:
  steps:
    - name: step1
      config:
        packages:
          pip:
            - 123
            - true
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanDoesNotMatchYAMLWhenQueryMissing(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: yaml-pip\n      filename-regex: '^workflow\\.yaml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "workflow.yaml"), `
workflow:
  jobs:
    - name: step1
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanDoesNotMatchYAMLExistsRuleWhenPathMissing(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: conda-environment\n      filename-regex: '^environment\\.ya?ml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yaml\n        exists: dependencies\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "environment.yaml"), `
name: app
channels:
  - conda-forge
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanMatchesTOMLDependenciesFromCustomRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pyproject\n      filename-regex: '^pyproject\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        queries:\n            - build-system.requires[]\n            - project.dependencies[]\n            - project.optional-dependencies.*[]\n            - dependency-groups.*[]\n            - tool.poetry.dependencies\n            - tool.poetry.group.*.dependencies\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[build-system]
requires = ["scikit-build-core>=0.10", "pybind11>=2.12.0"]

[project]
dependencies = ["requests>=2.31"]

[project.optional-dependencies]
dev = ["pytest>=8"]

[dependency-groups]
lint = ["mypy>=1.10"]

[tool.poetry.dependencies]
python = "^3.12"
django = "^5.0"
httpx = { version = "^0.27", extras = ["http2"] }

[tool.poetry.group.test.dependencies]
pytest-cov = "^5.0"
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	want := []DependencyReference{
		{Raw: "scikit-build-core>=0.10", Name: "scikit-build-core", VersionConstraint: ">=0.10", SourceGroup: "build-system.requires"},
		{Raw: "pybind11>=2.12.0", Name: "pybind11", VersionConstraint: ">=2.12.0", SourceGroup: "build-system.requires"},
		{Raw: "requests>=2.31", Name: "requests", VersionConstraint: ">=2.31", SourceGroup: "project.dependencies"},
		{Raw: "pytest>=8", Name: "pytest", VersionConstraint: ">=8", SourceGroup: "project.optional-dependencies.dev"},
		{Raw: "mypy>=1.10", Name: "mypy", VersionConstraint: ">=1.10", SourceGroup: "dependency-groups.lint"},
		{Raw: "django = \"^5.0\"", SourceGroup: "tool.poetry.dependencies"},
		{Raw: "httpx = { extras = [\"http2\"], version = \"^0.27\" }", SourceGroup: "tool.poetry.dependencies"},
		{Raw: "pytest-cov = \"^5.0\"", SourceGroup: "tool.poetry.group.test.dependencies"},
	}
	if !equalDependencies(result.Sources[0].Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Sources[0].Dependencies, want)
	}
}

func TestScanMatchesTOMLDependencyTablesFromCustomRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pipfile\n      filename-regex: '^Pipfile$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        table-queries:\n            - '*'\n        exclude-keys:\n            - source\n            - requires\n            - scripts\n            - pipenv\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "Pipfile"), `
[[source]]
name = "pypi"
url = "https://pypi.org/simple"
verify_ssl = true

[requires]
python_version = "3.12"

[packages]
requests = "*"

[tests]
pytest-cov = ">=5"
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	want := []string{
		"requests = \"*\"",
		"pytest-cov = \">=5\"",
	}
	if !slices.Equal(dependencyNames(result.Sources[0].Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Sources[0].Dependencies, want)
	}
}

func TestScanMatchesTOMLTableExistsAnyRuleWithDependenciesPresent(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: rust-cargo\n      filename-regex: '^Cargo\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        table-exists-any:\n            - dependencies\n            - dev-dependencies\n            - build-dependencies\n            - workspace.dependencies\n            - target.*.dependencies\n            - target.*.dev-dependencies\n            - target.*.build-dependencies\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "Cargo.toml"), `
[package]
name = "demo"
version = "0.1.0"

[target.'cfg(unix)'.dependencies]
nix = "0.29"
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
	if result.Sources[0].Dependencies != nil {
		t.Fatalf("expected no extracted dependencies, got %+v", result.Sources[0].Dependencies)
	}
}

func TestScanMatchesTOMLTableExistsAnyRuleWithoutDependencies(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: rust-cargo\n      filename-regex: '^Cargo\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        table-exists-any:\n            - dependencies\n            - dev-dependencies\n            - build-dependencies\n            - workspace.dependencies\n            - target.*.dependencies\n            - target.*.dev-dependencies\n            - target.*.build-dependencies\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "Cargo.toml"), `
[package]
name = "demo"
version = "0.1.0"
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
}

func TestScanMatchesTOMLExistsAnyRuleWithDependenciesPresent(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pdm-lock\n      filename-regex: '^pdm\\.lock$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        exists-any:\n            - package\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pdm.lock"), `
version = "4.5.0"

[[package]]
name = "requests"
version = "2.32.0"
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Sources[0].Analysis)
	}
	if result.Sources[0].Dependencies != nil {
		t.Fatalf("expected no extracted dependencies, got %+v", result.Sources[0].Dependencies)
	}
}

func TestScanMatchesTOMLExistsAnyRuleWithoutDependencies(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pdm-lock\n      filename-regex: '^pdm\\.lock$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        exists-any:\n            - package\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pdm.lock"), `
version = "4.5.0"
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
	}
	if result.Sources[0].Dependencies != nil {
		t.Fatalf("expected no extracted dependencies, got %+v", result.Sources[0].Dependencies)
	}
}

func TestScanDoesNotMatchTOMLWhenQueryResolvesToNoUsableValues(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pyproject\n      filename-regex: '^pyproject\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        queries:\n            - project.dependencies[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[project]
dependencies = [123, true]
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanIgnoresInlineTablesInExpandedTOMLArrays(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pyproject\n      filename-regex: '^pyproject\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        queries:\n            - dependency-groups.*[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[dependency-groups]
dev = [
  { include-group = "lint" },
  "pytest>=8",
]
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	want := []string{"pytest>=8"}
	if !slices.Equal(dependencyNames(result.Sources[0].Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Sources[0].Dependencies, want)
	}
}

func TestScanDoesNotMatchTOMLWhenExpandedArrayContainsOnlyInlineTables(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pyproject\n      filename-regex: '^pyproject\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        queries:\n            - dependency-groups.*[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[dependency-groups]
dev = [
  { include-group = "lint" },
]
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanPreservesPythonKeyOutsidePoetryDependencies(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: custom-toml\n      filename-regex: '^pyproject\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        queries:\n            - tool.custom.dependencies\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[tool.custom.dependencies]
python = "^3.12"
django = "^5.0"
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	want := []string{
		"django = \"^5.0\"",
		"python = \"^3.12\"",
	}
	if !slices.Equal(dependencyNames(result.Sources[0].Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Sources[0].Dependencies, want)
	}
}

func TestScanSkipsPythonInConcretePoetryDependencyGroupTable(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pyproject\n      filename-regex: '^pyproject\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        queries:\n            - tool.poetry.group.test.dependencies\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[tool.poetry.group.test.dependencies]
python = "^3.12"
django = "^5.0"
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}

	want := []string{"django = \"^5.0\""}
	if !slices.Equal(dependencyNames(result.Sources[0].Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Sources[0].Dependencies, want)
	}
}

func TestScanReportsTOMLParseErrorsAsDiagnostics(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pyproject\n      filename-regex: '^pyproject\\.toml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: toml\n        queries:\n            - project.dependencies[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[project
dependencies = ["requests>=2.31"]
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("expected scan to continue, got %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if len(result.Sources[0].Diagnostics) != 1 || !strings.Contains(result.Sources[0].Diagnostics[0].Message, "parse toml file") {
		t.Fatalf("expected toml parse warning, got %+v", result.Sources[0].Diagnostics)
	}
}

func TestScanBannerRegexRequiresNonEmptyCaptureGroups(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: js-banner\n      filename-regex: '^app\\.js$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: banner-regex\n        pattern: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)?\\s+v?(\\d+\\.\\d+\\.\\d+)?'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "app.js"), "/*! Demo */\nconsole.log('x')\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no dependency sources, got %+v", result.Sources)
	}
}

func TestScanBannerRegexUsesFirstMatchingRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: first\n      filename-regex: '^app\\.js$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: banner-regex\n        pattern: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)\\s+v?(\\d+\\.\\d+\\.\\d+)'\n    - id: second\n      filename-regex: '^app\\.js$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: banner-regex\n        pattern: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)\\s+v?(\\d+\\.\\d+\\.\\d+)'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "app.js"), "/*! jQuery v3.7.1 */\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %d", len(result.Sources))
	}
	if result.Sources[0].Detector != DetectorID("first") {
		t.Fatalf("expected first matching rule to win, got %+v", result.Sources[0])
	}
}

func mustLoadDefaultRules(t *testing.T) Ruleset {
	t.Helper()
	ruleset, err := LoadDefaultRules()
	if err != nil {
		t.Fatalf("load default rules failed: %v", err)
	}
	return ruleset
}

func TestScanDefaultRulesDetectsPathGlobSources(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []struct {
		name string
		path string
		typ  DetectorID
	}{
		// Group 2a: JVM ecosystem
		{name: "gradle version catalog", path: "gradle/libs.versions.toml", typ: "java-gradle-version-catalog"},
		{name: "gradle wrapper properties", path: "gradle/wrapper/gradle-wrapper.properties", typ: "java-gradle-wrapper"},
		{name: "sbt plugins", path: "project/plugins.sbt", typ: "scala-sbt-plugins"},
		{name: "sbt dependencies object", path: "project/Dependencies.scala", typ: "scala-sbt-dependencies"},
		{name: "sbt build properties", path: "project/build.properties", typ: "scala-sbt-build-props"},
		// Group 2b: Ruby / Go / Rust / .NET
		{name: "ruby appraisal gemfile", path: "gemfiles/rails_60.gemfile", typ: "ruby-appraisal"},
		{name: "go godep", path: "Godeps/Godeps.json", typ: "go-godep"},
		{name: "rust cargo config", path: ".cargo/config.toml", typ: "rust-cargo-config"},
		{name: "dotnet tools manifest", path: ".config/dotnet-tools.json", typ: "dotnet-tools-manifest"},
		// Group 2c: C++ / R / GitHub / Unity
		{name: "cmake module", path: "cmake/FindMyLib.cmake", typ: "cpp-cmake-modules"},
		{name: "meson wrap", path: "subprojects/zlib.wrap", typ: "cpp-meson-wrap"},
		{name: "r packrat lock", path: "packrat/packrat.lock", typ: "r-packrat-lock"},
		{name: "github actions workflow", path: ".github/workflows/ci.yml", typ: "github-actions-workflow"},
		{name: "unity packages lock", path: "Packages/packages-lock.json", typ: "unity-packages-lock"},
		// Group 2d: Bazel / Rush / Pants
		{name: "bazel third party bzl", path: "third_party/deps.bzl", typ: "bazel-third-party-bzl"},
		{name: "rush common versions", path: "common/config/rush/common-versions.json", typ: "rush-common-versions"},
		{name: "pants jvm build", path: "3rdparty/jvm/BUILD", typ: "pants-jvm-build"},
		// DRAFT (Group 3): ambiguous filenames — uncomment when rules are activated
		// {name: "gclient deps", path: "DEPS", typ: "cpp-gclient-deps"},
		// {name: "puppet cookbook metadata", path: "cookbooks/mypkg/metadata.json", typ: "puppet-cookbook-metadata"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mustWriteFile(t, filepath.Join(root, filepath.FromSlash(tc.path)), "")
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan failed: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
			}
			if result.Sources[0].Detector != tc.typ {
				t.Fatalf("expected type %q, got %q", tc.typ, result.Sources[0].Detector)
			}
			if result.Sources[0].Path != tc.path {
				t.Fatalf("expected path %q, got %q", tc.path, result.Sources[0].Path)
			}
		})
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}
