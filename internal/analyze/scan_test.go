package analyze

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func dependencyNames(dependencies []Dependency) []string {
	names := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		names = append(names, dependency.Name)
	}
	return names
}

func TestDetectManifestMatchesSupportedFiles(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []struct {
		name string
		want ManifestType
	}{
		{name: "requirements.txt", want: ManifestType("python-requirements")},
		{name: "requirements.in", want: ManifestType("python-requirements")},
		{name: "my-requirements.txt", want: ManifestType("python-requirements")},
		{name: "my-requirements.in", want: ManifestType("python-requirements")},
		{name: "requirements-dev.txt", want: ManifestType("python-requirements")},
		{name: "requirements.dev.in", want: ManifestType("python-requirements")},
		{name: "requirements.qt6_3.in", want: ManifestType("python-requirements")},
		{name: "my_requirements.prod.txt", want: ManifestType("python-requirements")},
		{name: "uv.lock", want: ManifestType("python-uv")},
		{name: "poetry.lock", want: ManifestType("python-poetry-lock")},
		{name: "Pipfile.lock", want: ManifestType("python-pipfile-lock")},
		{name: "pdm.lock", want: ManifestType("python-pdm-lock")},
		{name: "conda-lock.yml", want: ManifestType("python-conda-lock")},
		{name: "package-lock.json", want: ManifestType("js-npm-lock")},
		{name: "yarn.lock", want: ManifestType("js-yarn")},
		{name: "pnpm-lock.yaml", want: ManifestType("js-pnpm-lock")},
		{name: "bun.lock", want: ManifestType("js-bun-lock")},
		{name: "bun.lockb", want: ManifestType("js-bun-lockb")},
		{name: "deno.lock", want: ManifestType("deno-lock")},
		{name: "deno.json", want: ManifestType("deno-json")},
		{name: "deno.jsonc", want: ManifestType("deno-jsonc")},
		{name: "bower.json", want: ManifestType("js-bower")},
		{name: "npm-shrinkwrap.json", want: ManifestType("js-npm-shrinkwrap")},
		{name: "gradle.lockfile", want: ManifestType("java-gradle-lockfile")},
		{name: "build.gradle", want: ManifestType("java-gradle")},
		{name: "build.gradle.kts", want: ManifestType("java-gradle-kts")},
		{name: "settings.gradle", want: ManifestType("java-gradle-settings")},
		{name: "settings.gradle.kts", want: ManifestType("java-gradle-settings-kts")},
		{name: "Gemfile", want: ManifestType("ruby-gemfile")},
		{name: "Gemfile.lock", want: ManifestType("ruby-gemfile-lock")},
		{name: "Package.swift", want: ManifestType("swift-package")},
		{name: "Podfile", want: ManifestType("ios-podfile")},
		{name: "Cartfile", want: ManifestType("ios-cartfile")},
		{name: "composer.json", want: ManifestType("php-composer")},
		{name: "composer.lock", want: ManifestType("php-composer-lock")},
		{name: "pubspec.yaml", want: ManifestType("dart-pubspec")},
		{name: "pubspec.lock", want: ManifestType("dart-pubspec-lock")},
		{name: "rebar.config", want: ManifestType("erlang-rebar-config")},
		{name: "rebar.lock", want: ManifestType("erlang-rebar-lock")},
		{name: "deps.edn", want: ManifestType("clojure-deps-edn")},
		{name: "project.clj", want: ManifestType("clojure-project-clj")},
		{name: "stack.yaml", want: ManifestType("haskell-stack")},
		{name: "stack.yaml.lock", want: ManifestType("haskell-stack-lock")},
		{name: "cabal.project", want: ManifestType("haskell-cabal-project")},
		{name: "packages.config", want: ManifestType("dotnet-packages-config")},
		{name: "packages.lock.json", want: ManifestType("dotnet-packages-lock")},
		{name: "Directory.Packages.props", want: ManifestType("dotnet-directory-packages-props")},
		{name: "paket.dependencies", want: ManifestType("dotnet-paket-dependencies")},
		{name: "paket.lock", want: ManifestType("dotnet-paket-lock")},
		{name: "go.mod", want: ManifestType("go-mod")},
		{name: "go.sum", want: ManifestType("go-sum")},
		{name: "go.work", want: ManifestType("go-work")},
		{name: "Gopkg.toml", want: ManifestType("go-gopkg-toml")},
		{name: "glide.yaml", want: ManifestType("go-glide-yaml")},
		{name: "Cargo.toml", want: ManifestType("rust-cargo")},
		{name: "Cargo.lock", want: ManifestType("rust-cargo-lock")},
		{name: "Gopkg.lock", want: ManifestType("go-gopkg-lock")},
		{name: "glide.lock", want: ManifestType("go-glide-lock")},
		{name: "app.csproj", want: ManifestType("dotnet-csproj")},
		{name: "conan.lock", want: ManifestType("cpp-conan-lock")},
		{name: "Package.resolved", want: ManifestType("swift-package-resolved")},
		{name: "Podfile.lock", want: ManifestType("ios-podfile-lock")},
		{name: "mix.lock", want: ManifestType("elixir-mix-lock")},
		{name: "pom.xml", want: ManifestType("java")},
	}

	for _, tc := range testCases {
		got, ok := ruleset.DetectManifest(tc.name)
		if !ok {
			t.Fatalf("expected %s to be detected", tc.name)
		}
		if got != tc.want {
			t.Fatalf("expected type %q, got %q", tc.want, got)
		}
	}
}

func TestDetectManifestIgnoresSimilarNames(t *testing.T) {
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
		"Pipfile",
		"Pipfile.lock.bak",
		"conda-lock.yaml",
		"conda-lock.yml.bak",
		"index.html.bak",
		"component.jsx",
	}

	for _, tc := range testCases {
		if _, ok := ruleset.DetectManifest(tc); ok {
			t.Fatalf("expected %s to be ignored", tc)
		}
	}
}

func TestDetectManifestIgnoresParserBackedManifests(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []string{
		"package.json",
		"pyproject.toml",
		"Pipfile",
		"index.html",
		"job.tf",
		"app.js",
	}

	for _, tc := range testCases {
		if _, ok := ruleset.DetectManifest(tc); ok {
			t.Fatalf("expected %s to be ignored by DetectManifest", tc)
		}
	}
}

func TestDetectManifestIgnoresPathGlobBackedManifests(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: js\n    path-glob: 'apps/**/package.json'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	if _, ok := ruleset.DetectManifest("package.json"); ok {
		t.Fatalf("expected DetectManifest to ignore path-glob-backed rules")
	}
}

func TestDetectManifestFileAtRelativePathMatchesPathGlob(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-requirements\n    path-glob: '**/requirements/*.txt'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	got, deps, hasDependencies, ok, err := ruleset.DetectManifestFileAtRelativePath("apps/api/requirements/base.txt", "base.txt", "apps/api/requirements/base.txt")
	if err != nil {
		t.Fatalf("DetectManifestFileAtRelativePath failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected path-glob rule to match relative path input")
	}
	if got != ManifestType("python-requirements") {
		t.Fatalf("unexpected manifest type: got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if hasDependencies != nil {
		t.Fatalf("expected unknown has_dependencies, got %+v", hasDependencies)
	}
}

func TestDetectManifestFileAtRelativePathMatchesPathGlobWithAbsolutePath(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-requirements\n    path-glob: '**/requirements/*.txt'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	absPath := filepath.Join(root, "apps", "api", "requirements", "base.txt")

	got, deps, hasDependencies, ok, err := ruleset.DetectManifestFileAtRelativePath(absPath, "base.txt", "apps/api/requirements/base.txt")
	if err != nil {
		t.Fatalf("DetectManifestFileAtRelativePath failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected path-glob rule to match absolute path with explicit relative path")
	}
	if got != ManifestType("python-requirements") {
		t.Fatalf("unexpected manifest type: got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if hasDependencies != nil {
		t.Fatalf("expected unknown has_dependencies, got %+v", hasDependencies)
	}
}

func TestDetectManifestFileDoesNotMatchPathGlobWithoutRelativePath(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-requirements\n    path-glob: 'apps/**/requirements/*.txt'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	got, deps, hasDependencies, ok, err := ruleset.DetectManifestFile("apps/api/requirements/base.txt", "base.txt")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if ok {
		t.Fatalf("expected no path-glob match without explicit relative path, got type=%q deps=%+v hasDependencies=%+v", got, deps, hasDependencies)
	}
}

func TestDetectManifestFileMatchesSelectorOnlyFilenameRuleWithEmptyPath(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	got, deps, hasDependencies, ok, err := ruleset.DetectManifestFile("", "package-lock.json")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected filename-only rule to match with empty path")
	}
	if got != ManifestType("js-npm-lock") {
		t.Fatalf("unexpected manifest type: got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if hasDependencies != nil {
		t.Fatalf("expected unknown has_dependencies, got %+v", hasDependencies)
	}
}

func TestScanFindsNestedManifestsSortedByRelativePath(t *testing.T) {
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

	if len(result.Manifests) != 4 {
		t.Fatalf("expected 4 manifests, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Path != "a/index.html" || result.Manifests[1].Path != "a/package.json" || result.Manifests[2].Path != "b/requirements.dev.in" || result.Manifests[3].Path != "c/job.tf" {
		t.Fatalf("unexpected manifest order: %+v", result.Manifests)
	}
}

func TestScanFindsRequirementsInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, manifest := range result.Manifests {
		if manifest.Type == ManifestType("python-requirements") && manifest.Path == "requirements.qt6_3.in" {
			return
		}
	}

	t.Fatalf("expected requirements.qt6_3.in fixture to be detected, got %+v", result.Manifests)
}

func TestScanFindsPoetryLockInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, manifest := range result.Manifests {
		if manifest.Type == ManifestType("python-poetry-lock") && manifest.Path == "backend/poetry.lock" {
			return
		}
	}

	t.Fatalf("expected backend/poetry.lock fixture to be detected, got %+v", result.Manifests)
}

func TestScanFindsCondaEnvironmentInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "conda-environment"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, manifest := range result.Manifests {
		if manifest.Type == ManifestType("python-conda-environment") && manifest.Path == "environment.yml" {
			if manifest.Dependencies != nil {
				t.Fatalf("expected no extracted dependencies, got %+v", manifest.Dependencies)
			}
			if manifest.HasDependencies != nil {
				t.Fatalf("expected has_dependencies to remain unknown, got %+v", manifest.HasDependencies)
			}
			return
		}
	}

	t.Fatalf("expected environment.yml fixture to be detected, got %+v", result.Manifests)
}

func TestScanMarksPackageJSONHasDependenciesTrueForDependencySections(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), `{
  "name": "demo",
  "dependencies": {
    "react": "^19.0.0"
  }
}`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].HasDependencies == nil || !*result.Manifests[0].HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.Manifests[0].HasDependencies)
	}
	if len(result.Manifests[0].Dependencies) != 0 {
		t.Fatalf("expected no extracted dependencies, got %+v", result.Manifests[0].Dependencies)
	}
}

func TestScanMarksPackageJSONHasDependenciesFalseWithoutDependencySections(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), `{
  "name": "demo",
  "private": true
}`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].HasDependencies == nil || *result.Manifests[0].HasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", result.Manifests[0].HasDependencies)
	}
}

func TestScanMarksPackageJSONHasDependenciesFalseForEmptyDependencySections(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), `{
  "name": "demo",
  "devDependencies": {}
}`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].HasDependencies == nil || *result.Manifests[0].HasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", result.Manifests[0].HasDependencies)
	}
}

func TestScanMarksHasDependenciesTrueWhenDependenciesAreExtracted(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "toml", "pipfile"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].HasDependencies == nil || !*result.Manifests[0].HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.Manifests[0].HasDependencies)
	}
}

func TestScanFindsPipfileLockInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, manifest := range result.Manifests {
		if manifest.Type == ManifestType("python-pipfile-lock") && manifest.Path == "backend/Pipfile.lock" {
			return
		}
	}

	t.Fatalf("expected backend/Pipfile.lock fixture to be detected, got %+v", result.Manifests)
}

func TestScanFindsPdmLockInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, manifest := range result.Manifests {
		if manifest.Type == ManifestType("python-pdm-lock") && manifest.Path == "backend/pdm.lock" {
			return
		}
	}

	t.Fatalf("expected backend/pdm.lock fixture to be detected, got %+v", result.Manifests)
}

func TestScanFindsCondaLockInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, manifest := range result.Manifests {
		if manifest.Type == ManifestType("python-conda-lock") && manifest.Path == "backend/conda-lock.yml" {
			return
		}
	}

	t.Fatalf("expected backend/conda-lock.yml fixture to be detected, got %+v", result.Manifests)
}

func TestScanFindsAdditionalLockfilesInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	want := map[string]ManifestType{
		"frontend/package-lock.json":   ManifestType("js-npm-lock"),
		"frontend/pnpm-lock.yaml":      ManifestType("js-pnpm-lock"),
		"frontend/bun.lock":            ManifestType("js-bun-lock"),
		"frontend/bun.lockb":           ManifestType("js-bun-lockb"),
		"frontend/deno.lock":           ManifestType("deno-lock"),
		"java-service/gradle.lockfile": ManifestType("java-gradle-lockfile"),
		"ruby-app/Gemfile":             ManifestType("ruby-gemfile"),
		"ruby-app/Gemfile.lock":        ManifestType("ruby-gemfile-lock"),
		"php-app/composer.json":        ManifestType("php-composer"),
		"php-app/composer.lock":        ManifestType("php-composer-lock"),
		"go-service/go.mod":            ManifestType("go-mod"),
		"go-service/go.sum":            ManifestType("go-sum"),
		"rust-app/Cargo.toml":          ManifestType("rust-cargo"),
		"rust-app/Cargo.lock":          ManifestType("rust-cargo-lock"),
		"go-service/Gopkg.lock":        ManifestType("go-gopkg-lock"),
		"go-service/glide.lock":        ManifestType("go-glide-lock"),
		"dotnet-app/app.csproj":        ManifestType("dotnet-csproj"),
		"cpp-app/conan.lock":           ManifestType("cpp-conan-lock"),
		"ios-app/Package.resolved":     ManifestType("swift-package-resolved"),
		"ios-app/Podfile.lock":         ManifestType("ios-podfile-lock"),
		"elixir-app/mix.lock":          ManifestType("elixir-mix-lock"),
	}

	for _, manifest := range result.Manifests {
		wantType, ok := want[manifest.Path]
		if !ok {
			continue
		}
		if manifest.Type != wantType {
			t.Fatalf("expected %s to be detected as %q, got %q", manifest.Path, wantType, manifest.Type)
		}
		delete(want, manifest.Path)
	}

	if len(want) != 0 {
		t.Fatalf("expected all additional lockfile fixtures to be detected, missing %+v", want)
	}
}

func TestScanDefaultRulesMatchRequirementsDirectoriesAnywhere(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "requirements", "base.txt"), "")
	mustWriteFile(t, filepath.Join(root, "apps", "api", "requirements", "base.txt"), "")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %+v", result.Manifests)
	}
	if result.Manifests[0].Type != ManifestType("python-requirements-dir") || result.Manifests[1].Type != ManifestType("python-requirements-dir") {
		t.Fatalf("unexpected manifest types: %+v", result.Manifests)
	}
	if result.Manifests[0].Path != "apps/api/requirements/base.txt" || result.Manifests[1].Path != "requirements/base.txt" {
		t.Fatalf("unexpected manifests: %+v", result.Manifests)
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

	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanMatchesPyprojectDependenciesFromFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "toml", "pyproject"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("python-pyproject") || manifest.Path != "pyproject.toml" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	want := []Dependency{
		{Name: "scikit-build-core>=0.10", Section: "build-system.requires"},
		{Name: "pybind11>=2.12.0", Section: "build-system.requires"},
		{Name: "requests>=2.31", Section: "project.dependencies"},
		{Name: "fastapi[all]>=0.110; python_version >= '3.10'", Section: "project.dependencies"},
		{Name: "pytest>=8", Section: "project.optional-dependencies.dev"},
		{Name: "ruff==0.4.8", Section: "project.optional-dependencies.dev"},
		{Name: "mypy>=1.10", Section: "dependency-groups.lint"},
		{Name: "django = \"^5.0\"", Section: "tool.poetry.dependencies"},
		{Name: "httpx = { extras = [\"http2\"], version = \"^0.27\" }", Section: "tool.poetry.dependencies"},
		{Name: "private-lib = { branch = \"main\", git = \"https://github.com/acme/private-lib.git\" }", Section: "tool.poetry.dependencies"},
		{Name: "factory-boy = { markers = \"python_version >= '3.11'\", version = \"^3.3\" }", Section: "tool.poetry.group.test.dependencies"},
		{Name: "pytest-cov = \"^5.0\"", Section: "tool.poetry.group.test.dependencies"},
	}
	if !slices.Equal(manifest.Dependencies, want) {
		t.Fatalf("unexpected dependencies: %+v", manifest.Dependencies)
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

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("python-pipfile") || manifest.Path != "Pipfile" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	want := []string{
		"requests = \"*\"",
		"sphinx = \">=7\"",
	}
	if !slices.Equal(dependencyNames(manifest.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
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

	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanMatchesPipfileDependenciesFromFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "toml", "pipfile"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("python-pipfile") || manifest.Path != "Pipfile" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	want := []string{
		"requests = \"*\"",
		"pytest = \">=8\"",
		"sphinx = { extras = [\"docs\"], version = \">=7\" }",
	}
	if !slices.Equal(dependencyNames(manifest.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
	}
}

func TestScanIgnoresPipfileMetadataOnlyFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "toml", "pipfile-metadata-only"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("python-pipfile") || manifest.Path != "Pipfile" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if !slices.Equal(dependencyNames(manifest.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
	}
}

func TestScanMatchesSetupPyWithInstallRequiresFromFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "setup-py-install-requires"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("python-setup-py") || manifest.Path != "setup.py" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if got := dependencyNames(manifest.Dependencies); !slices.Equal(got, []string{"requests>=2.31", "pytest>=8", "ruff>=0.4"}) {
		t.Fatalf("unexpected dependencies: got %+v", got)
	}
}

func TestScanMatchesSetupPyWithExtrasRequireOnly(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "setup-py-extras-require"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if got := dependencyNames(result.Manifests[0].Dependencies); !slices.Equal(got, []string{"pytest>=8", "ruff>=0.4", "mkdocs>=1.6"}) {
		t.Fatalf("unexpected dependencies: got %+v", got)
	}
}

func TestScanMatchesSetupPyWithInstallRequiresAndExtrasRequire(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "setup-py-both"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if got := dependencyNames(result.Manifests[0].Dependencies); !slices.Equal(got, []string{"requests>=2.31", "pytest>=8"}) {
		t.Fatalf("unexpected dependencies: got %+v", got)
	}
}

func TestScanMatchesSetupPyWithoutExtractingNonLiteralDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "python", "setup-py-nonliteral"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if got := result.Manifests[0].Dependencies; len(got) != 0 {
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("python-setup-cfg") || manifest.Path != "setup.cfg" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if got := dependencyNames(manifest.Dependencies); !slices.Equal(got, want) {
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

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("html-external-scripts") || manifest.Path != "templates/index.html" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	want := []string{
		"https://cdn.jsdelivr.net/npm/dompurify@3.0.8/dist/purify.min.js",
		"https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js",
	}
	if !slices.Equal(dependencyNames(manifest.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
	}
}

func TestScanMatchesHTMLModuleImportFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "html", "module-import"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("html-external-scripts") || manifest.Path != "index.html" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	want := []string{"https://cdn.jsdelivr.net/npm/swiper@12.1.2/+esm"}
	if !slices.Equal(dependencyNames(manifest.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
	}
}

func TestScanMatchesHTMLNamespaceModuleImportFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "html", "module-namespace-import"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("html-external-scripts") || manifest.Path != "index.html" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	want := []string{"https://cdn.jsdelivr.net/npm/d3@7/+esm"}
	if !slices.Equal(dependencyNames(manifest.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
	}
}

func TestScanMatchesHTMLImportMapFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "html", "importmap"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("html-external-scripts") || manifest.Path != "index.html" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	want := []string{
		"https://cdn.jsdelivr.net/npm/super-media-element@1.3/+esm",
		"https://cdn.jsdelivr.net/npm/media-tracks@0.2/+esm",
		"https://cdn.jsdelivr.net/npm/hls.js@1.6.0-beta.1/dist/hls.mjs",
	}
	if !slices.Equal(dependencyNames(manifest.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
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

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	want := []string{
		"https://cdn.jsdelivr.net/npm/super-media-element@1.3/+esm",
		"https://cdn.jsdelivr.net/npm/media-tracks@0.2/+esm",
		"https://cdn.jsdelivr.net/npm/hls.js@1.6.0-beta.1/dist/hls.mjs",
		"https://registry.npmmirror.com/stylelint-config-recess-order/5.0.0/files/groups.js",
	}
	if !slices.Equal(dependencyNames(manifest.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
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

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	want := []string{
		"https://esm.sh/react@^19.1.0/",
		"https://esm.sh/react@^19.1.0",
		"https://esm.sh/@google/genai@^1.0.0",
		"https://esm.sh/recharts@^2.15.3",
		"https://esm.sh/react-dom@^19.1.0/",
	}
	if !slices.Equal(dependencyNames(result.Manifests[0].Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Manifests[0].Dependencies, want)
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanMatchesJavaScriptBannerDetectors(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []struct {
		path     string
		wantType ManifestType
		wantDeps []string
	}{
		{
			path:     "jquery.min.js",
			wantType: ManifestType("js-banner-block-start"),
			wantDeps: []string{"jQuery@3.7.1"},
		},
		{
			path:     "purify.min.js",
			wantType: ManifestType("js-banner-plain-block-start"),
			wantDeps: []string{"DOMPurify@3.0.8"},
		},
		{
			path:     "bootstrap.min.js",
			wantType: ManifestType("js-banner-multiline-preserved"),
			wantDeps: []string{"Bootstrap@5.3.3"},
		},
		{
			path:     "mustache.min.js",
			wantType: ManifestType("js-banner-line-comment"),
			wantDeps: []string{"Mustache.js@4.2.0"},
		},
		{
			path:     "htmx.min.js",
			wantType: ManifestType("js-banner-version-tagged"),
			wantDeps: []string{"htmx.org@2.0.4"},
		},
	}

	result, err := Scan(filepath.Join("..", "..", "testdata", "js", "banners"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != len(testCases) {
		t.Fatalf("expected %d manifests, got %d", len(testCases), len(result.Manifests))
	}

	gotByPath := make(map[string]ManifestMatch, len(result.Manifests))
	for _, manifest := range result.Manifests {
		gotByPath[manifest.Path] = manifest
	}

	for _, tc := range testCases {
		manifest, ok := gotByPath[tc.path]
		if !ok {
			t.Fatalf("expected manifest for %q, got %+v", tc.path, result.Manifests)
		}
		if manifest.Type != tc.wantType {
			t.Fatalf("unexpected manifest type for %q: got %q want %q", tc.path, manifest.Type, tc.wantType)
		}
		if !slices.Equal(dependencyNames(manifest.Dependencies), tc.wantDeps) {
			t.Fatalf("unexpected dependencies for %q: got %+v want %+v", tc.path, manifest.Dependencies, tc.wantDeps)
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
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

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("terraform.aws_glue_job.python") || result.Manifests[0].Path != "glue/job.tf" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	if len(result.Manifests[0].Dependencies) != 0 {
		t.Fatalf("expected terraform detector to keep dependencies empty, got %+v", result.Manifests[0].Dependencies)
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
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

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("typescript.cdk.aws_glue_job.python") || result.Manifests[0].Path != "glue/job.ts" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	wantDependencies := []string{"pandas==2.2.1", "scikit-learn==1.4.1.post1"}
	if !slices.Equal(dependencyNames(result.Manifests[0].Dependencies), wantDependencies) {
		t.Fatalf("unexpected dependencies: got %v want %v", result.Manifests[0].Dependencies, wantDependencies)
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if got := dependencyNames(result.Manifests[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if got := dependencyNames(result.Manifests[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if got := dependencyNames(result.Manifests[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("typescript.cdk.aws_glue_job.python") || result.Manifests[0].Path != "job.ts" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	wantDependencies := []string{"pandas==2.2.1", "scikit-learn==1.4.1.post1"}
	if !slices.Equal(dependencyNames(result.Manifests[0].Dependencies), wantDependencies) {
		t.Fatalf("unexpected dependencies: got %v want %v", result.Manifests[0].Dependencies, wantDependencies)
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
		if len(result.Manifests) != 0 {
			t.Fatalf("expected no manifests for %s, got %+v", root, result.Manifests)
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("typescript.cdk.aws_glue_job.python") || result.Manifests[0].Path != "job.ts" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	if got := dependencyNames(result.Manifests[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("typescript.cdk.aws_glue_job.python") || result.Manifests[0].Path != "job.ts" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	if got := dependencyNames(result.Manifests[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("typescript.cdk.aws_glue_job.python") || result.Manifests[0].Path != "job.ts" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	if got := result.Manifests[0].Dependencies; len(got) != 0 {
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("python.cdk.aws_glue_job.python") || result.Manifests[0].Path != "glue/job.py" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	wantDependencies := []string{"pandas==2.2.1", "scikit-learn==1.4.1.post1"}
	if !slices.Equal(dependencyNames(result.Manifests[0].Dependencies), wantDependencies) {
		t.Fatalf("unexpected dependencies: got %v want %v", result.Manifests[0].Dependencies, wantDependencies)
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if got := dependencyNames(result.Manifests[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanMatchesPythonFixtureFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "python", "glue-cfnjob-inline")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("python.cdk.aws_glue_job.python") || result.Manifests[0].Path != "job.py" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	if got := dependencyNames(result.Manifests[0].Dependencies); !slices.Equal(got, []string{"pandas==2.2.1"}) {
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

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Path != "src/package.json" {
		t.Fatalf("unexpected manifest path: %+v", result.Manifests[0])
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected vendor manifest to be found without ignore list, got %d", len(result.Manifests))
	}

	result, err = Scan(root, []string{"vendor"}, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected vendor manifest to be ignored, got %+v", result.Manifests)
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
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: \"\"\n    filename-regex: '^package\\.json$'\n"))
	if err == nil {
		t.Fatalf("expected error for missing rule name")
	}
}

func TestLoadRulesRejectsInvalidRegex(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js\n    filename-regex: '('\n"))
	if err == nil {
		t.Fatalf("expected invalid regex error")
	}
}

func TestLoadRulesRejectsMissingSelectors(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js\n"))
	if err == nil {
		t.Fatalf("expected error for missing selectors")
	}
}

func TestLoadRulesAcceptsPathGlobSelector(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js\n    path-glob: 'apps/**/package.json'\n"))
	if err != nil {
		t.Fatalf("expected path glob rule to load: %v", err)
	}
}

func TestLoadRulesRejectsInvalidPathGlob(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js\n    path-glob: 'apps/[.json'\n"))
	if err == nil {
		t.Fatalf("expected invalid path glob error")
	}
}

func TestLoadRulesRejectsPathGlobWithEmptySegment(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js\n    path-glob: 'apps//package.json'\n"))
	if err == nil {
		t.Fatalf("expected invalid path glob with empty segment")
	}
}

func TestLoadRulesRejectsPathGlobWithInvalidRecursiveWildcardPlacement(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js\n    path-glob: 'apps/**b/package.json'\n"))
	if err == nil {
		t.Fatalf("expected invalid recursive wildcard placement")
	}
}

func TestLoadRulesAcceptsCombinedSelectors(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js\n    filename-regex: '^package\\.json$'\n    path-glob: 'apps/**/package.json'\n"))
	if err != nil {
		t.Fatalf("expected combined selector rule to load: %v", err)
	}
}

func TestScanMatchesPathGlobRequirementsAnywhere(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-requirements\n    path-glob: '**/requirements/*.txt'\n"))
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
	if len(result.Manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %+v", result.Manifests)
	}
	if result.Manifests[0].Path != "apps/api/requirements/base.txt" || result.Manifests[1].Path != "requirements/base.txt" {
		t.Fatalf("unexpected manifests: %+v", result.Manifests)
	}
}

func TestScanMatchesQuestionMarkGlobSemantics(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-requirements\n    path-glob: 'apps/**/req?.txt'\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %+v", result.Manifests)
	}
	if result.Manifests[0].Path != "apps/api/req1.txt" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
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
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-requirements\n    path-glob: 'apps/**/requirements/*.txt'\n"))
	if err != nil {
		t.Fatalf("expected path glob rule to load: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "api", "requirements", "base.txt"), "")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %+v", result.Manifests)
	}
	if result.Manifests[0].Path != "apps/api/requirements/base.txt" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
}

func TestScanPathGlobDoesNotMatchNestedGrandchildren(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-requirements\n    path-glob: '**/requirements/*.txt'\n"))
	if err != nil {
		t.Fatalf("expected path glob rule to load: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "apps", "api", "requirements", "nested", "base.txt"), "")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests for nested grandchild path, got %+v", result.Manifests)
	}
}

func TestScanCombinedSelectorsRequireBothMatches(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-requirements\n    filename-regex: '^requirements\\.txt$'\n    path-glob: '**/requirements/*.txt'\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %+v", result.Manifests)
	}
	if result.Manifests[0].Path != "apps/api/requirements/requirements.txt" {
		t.Fatalf("unexpected manifest path: %+v", result.Manifests[0])
	}
}

func TestLoadRulesAcceptsBannerRegexParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js-banner\n    filename-regex: '.*\\.js$'\n    banner-regex: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)\\s+v?(\\d+\\.\\d+\\.\\d+)'\n"))
	if err != nil {
		t.Fatalf("expected banner regex rule to load: %v", err)
	}
}

func TestLoadRulesRejectsInvalidBannerRegex(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js-banner\n    filename-regex: '.*\\.js$'\n    banner-regex: '('\n"))
	if err == nil {
		t.Fatalf("expected invalid banner regex error")
	}
}

func TestLoadRulesRejectsBannerRegexWithoutRequiredCaptureGroups(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js-banner\n    filename-regex: '.*\\.js$'\n    banner-regex: '^/\\*!\\s*[A-Za-z0-9._/-]+\\s+v?\\d+\\.\\d+\\.\\d+'\n"))
	if err == nil {
		t.Fatalf("expected missing capture group error")
	}
}

func TestLoadRulesRejectsTerraformParserWithoutResourceType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: terraform.aws_glue_job.python\n    filename-regex: '.*\\.tf$'\n    terraform:\n      conditions:\n        - path: default_arguments.--job-language\n          equals: python\n"))
	if err == nil {
		t.Fatalf("expected missing resource type error")
	}
}

func TestLoadRulesAcceptsTypeScriptParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        module: aws-cdk-lib/aws-glue\n        construct: CfnJob\n        props_argument_index: 2\n        within:\n          - defaultArguments\n        conditions:\n          - key: --additional-python-modules\n            present: true\n        extract:\n          key: --additional-python-modules\n          split: comma\n"))
	if err != nil {
		t.Fatalf("expected typescript parser to load: %v", err)
	}
}

func TestLoadRulesAcceptsPythonParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python.cdk.aws_glue_job.python\n    filename-regex: '.*\\.py$'\n    python:\n      cdk_construct:\n        module: aws_cdk.aws_glue\n        construct: CfnJob\n        keyword_argument: default_arguments\n        conditions:\n          - key: --additional-python-modules\n            present: true\n        extract:\n          key: --additional-python-modules\n          split: comma\n"))
	if err != nil {
		t.Fatalf("expected python parser to load: %v", err)
	}
}

func TestLoadRulesAcceptsGenericPythonCallParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-setup-py\n    filename-regex: '^setup\\.py$'\n    python:\n      call:\n        module: setuptools\n        function: setup\n        conditions:\n          any_of:\n            - keyword: install_requires\n              present: true\n            - keyword: extras_require\n              present: true\n        extract:\n          - keyword: install_requires\n            literal: string_list\n          - keyword: extras_require\n            literal: dict_string_lists\n"))
	if err != nil {
		t.Fatalf("expected generic python call parser to load: %v", err)
	}
}

func TestLoadRulesRejectsPythonCallParserWithUnsupportedLiteralExtractor(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-setup-py\n    filename-regex: '^setup\\.py$'\n    python:\n      call:\n        module: setuptools\n        function: setup\n        conditions:\n          any_of:\n            - keyword: install_requires\n              present: true\n        extract:\n          - keyword: install_requires\n            literal: string_tuple\n"))
	if err == nil {
		t.Fatalf("expected unsupported literal extractor error")
	}
}

func TestLoadRulesAcceptsHTMLParser(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: html-external-scripts\n    filename-regex: '.*\\.html$'\n    html:\n      external_scripts: true\n"))
	if err != nil {
		t.Fatalf("expected html rule to load: %v", err)
	}
	if got := ruleset.SupportedManifestTypes(); !slices.Equal(got, []ManifestType{ManifestType("html-external-scripts")}) {
		t.Fatalf("unexpected supported types: %+v", got)
	}
}

func TestLoadRulesRejectsHTMLParserWithoutExternalScripts(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: html-external-scripts\n    filename-regex: '.*\\.html$'\n    html: {}\n"))
	if err == nil {
		t.Fatalf("expected missing html parser configuration error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithoutModule(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        construct: CfnJob\n        props_argument_index: 2\n        conditions:\n          - key: --additional-python-modules\n            present: true\n"))
	if err == nil {
		t.Fatalf("expected missing module error")
	}
}

func TestLoadRulesRejectsPythonParserWithoutKeywordArgument(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python.cdk.aws_glue_job.python\n    filename-regex: '.*\\.py$'\n    python:\n      cdk_construct:\n        module: aws_cdk.aws_glue\n        construct: CfnJob\n        conditions:\n          - key: --additional-python-modules\n            present: true\n"))
	if err == nil {
		t.Fatalf("expected missing keyword argument error")
	}
}

func TestScanDoesNotMatchSetupPyWithoutTargetKeywords(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-setup-py\n    filename-regex: '^setup\\.py$'\n    python:\n      call:\n        module: setuptools\n        function: setup\n        conditions:\n          any_of:\n            - keyword: install_requires\n              present: true\n            - keyword: extras_require\n              present: true\n"))
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestLoadRulesRejectsTypeScriptParserWithoutConstruct(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        module: aws-cdk-lib/aws-glue\n        props_argument_index: 2\n        conditions:\n          - key: --additional-python-modules\n            present: true\n"))
	if err == nil {
		t.Fatalf("expected missing construct error")
	}
}

func TestLoadRulesRejectsTerraformParserWithoutConditions(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: terraform.aws_glue_job.python\n    filename-regex: '.*\\.tf$'\n    terraform:\n      resource_type: aws_glue_job\n"))
	if err == nil {
		t.Fatalf("expected missing conditions error")
	}
}

func TestLoadRulesRejectsTerraformConditionWithoutMatcher(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: terraform.aws_glue_job.python\n    filename-regex: '.*\\.tf$'\n    terraform:\n      resource_type: aws_glue_job\n      conditions:\n        - path: default_arguments.--job-language\n"))
	if err == nil {
		t.Fatalf("expected invalid terraform condition error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithoutPropsArgumentIndex(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        module: aws-cdk-lib/aws-glue\n        construct: CfnJob\n        conditions:\n          - key: --additional-python-modules\n            present: true\n"))
	if err == nil {
		t.Fatalf("expected missing props argument index error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithoutConditions(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        module: aws-cdk-lib/aws-glue\n        construct: CfnJob\n        props_argument_index: 2\n"))
	if err == nil {
		t.Fatalf("expected missing typescript conditions error")
	}
}

func TestLoadRulesRejectsPythonParserWithoutConditions(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python.cdk.aws_glue_job.python\n    filename-regex: '.*\\.py$'\n    python:\n      cdk_construct:\n        module: aws_cdk.aws_glue\n        construct: CfnJob\n        keyword_argument: default_arguments\n"))
	if err == nil {
		t.Fatalf("expected missing python conditions error")
	}
}

func TestLoadRulesRejectsTypeScriptConditionWithoutMatcher(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        module: aws-cdk-lib/aws-glue\n        construct: CfnJob\n        props_argument_index: 2\n        conditions:\n          - key: --additional-python-modules\n"))
	if err == nil {
		t.Fatalf("expected invalid typescript condition error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithUnsupportedExtractSplit(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        module: aws-cdk-lib/aws-glue\n        construct: CfnJob\n        props_argument_index: 2\n        conditions:\n          - key: --additional-python-modules\n            present: true\n        extract:\n          key: --additional-python-modules\n          split: space\n"))
	if err == nil {
		t.Fatalf("expected invalid extract split error")
	}
}

func TestLoadRulesRejectsPythonParserWithUnsupportedExtractSplit(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python.cdk.aws_glue_job.python\n    filename-regex: '.*\\.py$'\n    python:\n      cdk_construct:\n        module: aws_cdk.aws_glue\n        construct: CfnJob\n        keyword_argument: default_arguments\n        conditions:\n          - key: --additional-python-modules\n            present: true\n        extract:\n          key: --additional-python-modules\n          split: space\n"))
	if err == nil {
		t.Fatalf("expected invalid python extract split error")
	}
}

func TestLoadRulesAcceptsYAMLParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '.*\\.ya?ml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("expected yaml parser to load: %v", err)
	}
}

func TestLoadRulesAcceptsYAMLExistsParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: conda-environment\n    filename-regex: '^environment\\.ya?ml$'\n    yaml:\n      exists: dependencies\n"))
	if err != nil {
		t.Fatalf("expected yaml exists parser to load: %v", err)
	}
}

func TestLoadRulesSupportsTOMLTableQueriesAndExcludeKeys(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pipfile\n    filename-regex: '^Pipfile$'\n    toml:\n      table-queries:\n        - '*'\n      exclude-keys:\n        - source\n        - requires\n"))
	if err != nil {
		t.Fatalf("expected generic toml table config to load: %v", err)
	}
}

func TestLoadRulesRejectsYAMLParserWithoutQuery(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '.*\\.ya?ml$'\n    yaml: {}\n"))
	if err == nil {
		t.Fatalf("expected missing yaml query error")
	}
}

func TestLoadRulesRejectsYAMLParserWithQueryAndExists(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: conda-environment\n    filename-regex: '^environment\\.ya?ml$'\n    yaml:\n      query: dependencies[]\n      exists: dependencies\n"))
	if err == nil {
		t.Fatalf("expected mutually exclusive yaml query and exists error")
	}
}

func TestLoadRulesRejectsMalformedYAMLQuery(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '.*\\.ya?ml$'\n    yaml:\n      query: workflow..steps[].config.packages.pip[]\n"))
	if err == nil {
		t.Fatalf("expected malformed yaml query error")
	}
}

func TestLoadRulesRejectsMalformedYAMLExistsPath(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: conda-environment\n    filename-regex: '^environment\\.ya?ml$'\n    yaml:\n      exists: dependencies..\n"))
	if err == nil {
		t.Fatalf("expected malformed yaml exists path error")
	}
}

func TestLoadRulesAcceptsTOMLParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - project.dependencies[]\n        - project.optional-dependencies.*[]\n"))
	if err != nil {
		t.Fatalf("expected toml parser to load: %v", err)
	}
}

func TestLoadRulesRejectsTOMLParserWithoutQueries(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml: {}\n"))
	if err == nil {
		t.Fatalf("expected missing toml queries error")
	}
}

func TestLoadRulesRejectsMalformedTOMLQuery(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - project..dependencies[]\n"))
	if err == nil {
		t.Fatalf("expected malformed toml query error")
	}
}

func TestLoadRulesRejectsTOMLParserWithOtherParserType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: mixed\n    filename-regex: '^pyproject\\.toml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n    toml:\n      queries:\n        - project.dependencies[]\n"))
	if err == nil {
		t.Fatalf("expected multiple parser type error")
	}
}

func TestLoadRulesAcceptsINIParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-setup-cfg\n    filename-regex: '^setup\\.cfg$'\n    ini:\n      queries:\n        - section: options\n          key: install_requires\n        - section: options.extras_require\n          key: '*'\n"))
	if err != nil {
		t.Fatalf("expected ini parser to load: %v", err)
	}
}

func TestLoadRulesRejectsINIParserWithoutQueries(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-setup-cfg\n    filename-regex: '^setup\\.cfg$'\n    ini: {}\n"))
	if err == nil {
		t.Fatalf("expected missing ini queries error")
	}
}

func TestLoadRulesRejectsINIQueryWithoutSection(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-setup-cfg\n    filename-regex: '^setup\\.cfg$'\n    ini:\n      queries:\n        - key: install_requires\n"))
	if err == nil {
		t.Fatalf("expected missing ini section error")
	}
}

func TestLoadRulesRejectsINIQueryWithoutKey(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-setup-cfg\n    filename-regex: '^setup\\.cfg$'\n    ini:\n      queries:\n        - section: options\n"))
	if err == nil {
		t.Fatalf("expected missing ini key error")
	}
}

func TestLoadRulesRejectsINIParserWithOtherParserType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: mixed\n    filename-regex: '^setup\\.cfg$'\n    ini:\n      queries:\n        - section: options\n          key: install_requires\n    toml:\n      queries:\n        - project.dependencies[]\n"))
	if err == nil {
		t.Fatalf("expected multiple parser type error")
	}
}

func TestLoadRulesRejectsMultipleParserTypes(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: mixed\n    filename-regex: '.*'\n    terraform:\n      resource_type: aws_glue_job\n      conditions:\n        - path: default_arguments.--job-language\n          equals: python\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
	if err == nil {
		t.Fatalf("expected multiple parser type error")
	}
}

func TestLoadRulesRejectsBannerRegexWithOtherParserType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: mixed\n    filename-regex: '.*\\.js$'\n    banner-regex: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)\\s+v?(\\d+\\.\\d+\\.\\d+)'\n    html:\n      external_scripts: true\n"))
	if err == nil {
		t.Fatalf("expected multiple parser type error")
	}
}

func TestLoadRulesSupportsCustomFirstMatchOrdering(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: broad\n    filename-regex: '.*\\.json$'\n  - name: specific\n    filename-regex: '^package\\.json$'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	got, ok := ruleset.DetectManifest("package.json")
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("broad") {
		t.Fatalf("expected first pattern to win, got %q", got)
	}
}

func TestLoadDefaultRulesProvidesSupportedTypeOrder(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	want := []ManifestType{
		ManifestType("python-requirements"),
		ManifestType("python-requirements-dir"),
		ManifestType("python-uv"),
		ManifestType("python-poetry-lock"),
		ManifestType("python-pipfile-lock"),
		ManifestType("python-pdm-lock"),
		ManifestType("python-conda-lock"),
		ManifestType("python-pyproject"),
		ManifestType("python-conda-environment"),
		ManifestType("python-pipfile"),
		ManifestType("python-setup-py"),
		ManifestType("python-setup-cfg"),
		ManifestType("js"),
		ManifestType("js-bower"),
		ManifestType("js-npm-shrinkwrap"),
		ManifestType("js-npm-lock"),
		ManifestType("js-yarn"),
		ManifestType("js-pnpm-lock"),
		ManifestType("js-bun-lock"),
		ManifestType("js-bun-lockb"),
		ManifestType("deno-lock"),
		ManifestType("deno-json"),
		ManifestType("deno-jsonc"),
		ManifestType("java"),
		ManifestType("java-gradle-lockfile"),
		ManifestType("java-gradle"),
		ManifestType("java-gradle-kts"),
		ManifestType("java-gradle-settings"),
		ManifestType("java-gradle-settings-kts"),
		ManifestType("ruby-gemfile"),
		ManifestType("ruby-gemfile-lock"),
		ManifestType("swift-package"),
		ManifestType("ios-podfile"),
		ManifestType("ios-cartfile"),
		ManifestType("php-composer"),
		ManifestType("php-composer-lock"),
		ManifestType("dart-pubspec"),
		ManifestType("dart-pubspec-lock"),
		ManifestType("erlang-rebar-config"),
		ManifestType("erlang-rebar-lock"),
		ManifestType("clojure-deps-edn"),
		ManifestType("clojure-project-clj"),
		ManifestType("haskell-stack"),
		ManifestType("haskell-stack-lock"),
		ManifestType("haskell-cabal-project"),
		ManifestType("dotnet-packages-config"),
		ManifestType("dotnet-packages-lock"),
		ManifestType("dotnet-directory-packages-props"),
		ManifestType("dotnet-paket-dependencies"),
		ManifestType("dotnet-paket-lock"),
		ManifestType("go-mod"),
		ManifestType("go-sum"),
		ManifestType("go-work"),
		ManifestType("go-gopkg-toml"),
		ManifestType("go-glide-yaml"),
		ManifestType("rust-cargo"),
		ManifestType("rust-cargo-lock"),
		ManifestType("go-gopkg-lock"),
		ManifestType("go-glide-lock"),
		ManifestType("dotnet-csproj"),
		ManifestType("cpp-conan-lock"),
		ManifestType("swift-package-resolved"),
		ManifestType("ios-podfile-lock"),
		ManifestType("elixir-mix-lock"),
		ManifestType("js-banner-block-start"),
		ManifestType("js-banner-plain-block-start"),
		ManifestType("js-banner-multiline-preserved"),
		ManifestType("js-banner-line-comment"),
		ManifestType("js-banner-version-tagged"),
		ManifestType("html-external-scripts"),
		ManifestType("terraform.aws_glue_job.python"),
		ManifestType("typescript.cdk.aws_glue_job.python"),
		ManifestType("python.cdk.aws_glue_job.python"),
	}
	got := ruleset.SupportedManifestTypes()
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected supported type order: got %v want %v", got, want)
	}
}

func TestScanMatchesYAMLDependenciesFromCustomRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '^workflow\\.yaml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if !slices.Equal(dependencyNames(result.Manifests[0].Dependencies), []string{"requests", "pendulum"}) {
		t.Fatalf("unexpected dependencies: %+v", result.Manifests[0].Dependencies)
	}
}

func TestScanMatchesYAMLExistsRuleWithoutExtractingDependencies(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: conda-environment\n    filename-regex: '^environment\\.ya?ml$'\n    yaml:\n      exists: dependencies\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("conda-environment") || result.Manifests[0].Path != "environment.yml" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	if result.Manifests[0].Dependencies != nil {
		t.Fatalf("expected no extracted dependencies, got %+v", result.Manifests[0].Dependencies)
	}
}

func TestScanMatchesYAMLDependenciesFromPathGlobRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    path-glob: '**/pipelines/workflow.yaml'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("yaml-pip") || manifest.Path != "apps/api/pipelines/workflow.yaml" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if !slices.Equal(dependencyNames(manifest.Dependencies), []string{"requests", "pendulum"}) {
		t.Fatalf("unexpected dependencies: %+v", manifest.Dependencies)
	}
}

func TestScanMatchesTOMLDependenciesFromPathGlobRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    path-glob: '**/pyproject.toml'\n    toml:\n      queries:\n        - project.dependencies[]\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("python-pyproject") || manifest.Path != "services/api/pyproject.toml" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if !slices.Equal(dependencyNames(manifest.Dependencies), []string{"requests>=2.31"}) {
		t.Fatalf("unexpected dependencies: %+v", manifest.Dependencies)
	}
}

func TestScanMatchesTOMLDependenciesFromCombinedSelectors(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    path-glob: '**/pipelines/pyproject.toml'\n    toml:\n      queries:\n        - project.dependencies[]\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %+v", result.Manifests)
	}
	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("python-pyproject") || manifest.Path != "apps/api/pipelines/pyproject.toml" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if !slices.Equal(dependencyNames(manifest.Dependencies), []string{"requests>=2.31"}) {
		t.Fatalf("unexpected dependencies: %+v", manifest.Dependencies)
	}
}

func TestScanMatchesYAMLDependenciesAcrossNestedLists(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '^workflow\\.yaml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if !slices.Equal(dependencyNames(result.Manifests[0].Dependencies), []string{"requests", "pendulum"}) {
		t.Fatalf("unexpected dependencies: %+v", result.Manifests[0].Dependencies)
	}
}

func TestScanMatchesYAMLDependenciesFromCombinedSelectors(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '^workflow\\.yaml$'\n    path-glob: '**/pipelines/workflow.yaml'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %+v", result.Manifests)
	}
	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("yaml-pip") || manifest.Path != "apps/api/pipelines/workflow.yaml" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if !slices.Equal(dependencyNames(manifest.Dependencies), []string{"requests", "pendulum"}) {
		t.Fatalf("unexpected dependencies: %+v", manifest.Dependencies)
	}
}

func TestScanDoesNotMatchYAMLWhenQueryResolvesToNonStrings(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '^workflow\\.yaml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanDoesNotMatchYAMLWhenQueryMissing(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '^workflow\\.yaml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanDoesNotMatchYAMLExistsRuleWhenPathMissing(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: conda-environment\n    filename-regex: '^environment\\.ya?ml$'\n    yaml:\n      exists: dependencies\n"))
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanMatchesTOMLDependenciesFromCustomRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - build-system.requires[]\n        - project.dependencies[]\n        - project.optional-dependencies.*[]\n        - dependency-groups.*[]\n        - tool.poetry.dependencies\n        - tool.poetry.group.*.dependencies\n"))
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

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	want := []Dependency{
		{Name: "scikit-build-core>=0.10", Section: "build-system.requires"},
		{Name: "pybind11>=2.12.0", Section: "build-system.requires"},
		{Name: "requests>=2.31", Section: "project.dependencies"},
		{Name: "pytest>=8", Section: "project.optional-dependencies.dev"},
		{Name: "mypy>=1.10", Section: "dependency-groups.lint"},
		{Name: "django = \"^5.0\"", Section: "tool.poetry.dependencies"},
		{Name: "httpx = { extras = [\"http2\"], version = \"^0.27\" }", Section: "tool.poetry.dependencies"},
		{Name: "pytest-cov = \"^5.0\"", Section: "tool.poetry.group.test.dependencies"},
	}
	if !slices.Equal(result.Manifests[0].Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Manifests[0].Dependencies, want)
	}
}

func TestScanMatchesTOMLDependencyTablesFromCustomRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pipfile\n    filename-regex: '^Pipfile$'\n    toml:\n      table-queries:\n        - '*'\n      exclude-keys:\n        - source\n        - requires\n        - scripts\n        - pipenv\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	want := []string{
		"requests = \"*\"",
		"pytest-cov = \">=5\"",
	}
	if !slices.Equal(dependencyNames(result.Manifests[0].Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Manifests[0].Dependencies, want)
	}
}

func TestScanDoesNotMatchTOMLWhenQueryResolvesToNoUsableValues(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - project.dependencies[]\n"))
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanIgnoresInlineTablesInExpandedTOMLArrays(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - dependency-groups.*[]\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	want := []string{"pytest>=8"}
	if !slices.Equal(dependencyNames(result.Manifests[0].Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Manifests[0].Dependencies, want)
	}
}

func TestScanDoesNotMatchTOMLWhenExpandedArrayContainsOnlyInlineTables(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - dependency-groups.*[]\n"))
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
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanPreservesPythonKeyOutsidePoetryDependencies(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: custom-toml\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - tool.custom.dependencies\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	want := []string{
		"django = \"^5.0\"",
		"python = \"^3.12\"",
	}
	if !slices.Equal(dependencyNames(result.Manifests[0].Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Manifests[0].Dependencies, want)
	}
}

func TestScanSkipsPythonInConcretePoetryDependencyGroupTable(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - tool.poetry.group.test.dependencies\n"))
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
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	want := []string{"django = \"^5.0\""}
	if !slices.Equal(dependencyNames(result.Manifests[0].Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Manifests[0].Dependencies, want)
	}
}

func TestScanReturnsTOMLParseErrors(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - project.dependencies[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[project
dependencies = ["requests>=2.31"]
`)

	_, err = Scan(root, nil, ruleset)
	if err == nil {
		t.Fatalf("expected scan to fail")
	}
	if !strings.Contains(err.Error(), "parse toml file") {
		t.Fatalf("expected toml parse error, got %v", err)
	}
}

func TestScanBannerRegexRequiresNonEmptyCaptureGroups(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: js-banner\n    filename-regex: '^app\\.js$'\n    banner-regex: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)?\\s+v?(\\d+\\.\\d+\\.\\d+)?'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "app.js"), "/*! Demo */\nconsole.log('x')\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanBannerRegexUsesFirstMatchingRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: first\n    filename-regex: '^app\\.js$'\n    banner-regex: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)\\s+v?(\\d+\\.\\d+\\.\\d+)'\n  - name: second\n    filename-regex: '^app\\.js$'\n    banner-regex: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)\\s+v?(\\d+\\.\\d+\\.\\d+)'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "app.js"), "/*! jQuery v3.7.1 */\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("first") {
		t.Fatalf("expected first matching rule to win, got %+v", result.Manifests[0])
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

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}
