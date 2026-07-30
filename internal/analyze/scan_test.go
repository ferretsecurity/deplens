package analyze

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
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
		{name: "rebar.config", want: DetectorID("erlang-rebar-config")},
		{name: "rebar.lock", want: DetectorID("erlang-rebar-lock")},
		{name: "deps.edn", want: DetectorID("clojure-deps-edn")},
		{name: "project.clj", want: DetectorID("clojure-project-clj")},
		{name: "stack.yaml", want: DetectorID("haskell-stack")},
		{name: "stack.yaml.lock", want: DetectorID("haskell-stack-lock")},
		{name: "cabal.project", want: DetectorID("haskell-cabal-project")},
		{name: "paket.dependencies", want: DetectorID("dotnet-paket-dependencies")},
		{name: "paket.lock", want: DetectorID("dotnet-paket-lock")},
		{name: "go.sum", want: DetectorID("go-sum")},
		{name: "go.work", want: DetectorID("go-work")},
		{name: "Gopkg.toml", want: DetectorID("go-gopkg-toml")},
		{name: "glide.lock", want: DetectorID("go-glide-lock")},
		{name: "conan.lock", want: DetectorID("cpp-conan-lock")},
		{name: "mix.exs", want: DetectorID("elixir-mix")},
		{name: "mix.lock", want: DetectorID("elixir-mix-lock")},
		{name: "demo.cabal", want: DetectorID("haskell-cabal")},
		{name: "demo.gemspec", want: DetectorID("ruby-gemspec")},
		{name: "conanfile.txt", want: DetectorID("cpp-conanfile")},
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
		{name: "conanfile.py", want: DetectorID("cpp-conanfile-py")},
		{name: "vcpkg-configuration.json", want: DetectorID("cpp-vcpkg-config")},
		{name: "meson.build", want: DetectorID("cpp-meson")},
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
		{name: "build.boot", want: DetectorID("clojure-boot")},
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
		{name: "shard.lock", want: DetectorID("crystal-shard-lock")},
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
		{name: "buf.lock", want: DetectorID("buf-lock")},
		{name: "Puppetfile", want: DetectorID("puppet-puppetfile")},
		{name: "Berksfile", want: DetectorID("chef-berksfile")},
		{name: "Berksfile.lock", want: DetectorID("chef-berksfile-lock")},
		{name: "metadata.rb", want: DetectorID("chef-metadata")},
		{name: "Policyfile.rb", want: DetectorID("chef-policyfile")},
		{name: "Policyfile.lock.json", want: DetectorID("chef-policyfile-lock")},
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

func TestSwiftPackageManifestFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []string{
		"package-url-from",
		"package-url-range-and-exact",
		"package-url-branch-and-revision",
		"package-registry",
		"package-local-and-named",
		"package-traits",
		"package-legacy-requirements",
	} {
		t.Run(fixture, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "swift", fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			if result.Sources[0].Detector != "swift-package" || result.Sources[0].Path != "Package.swift" {
				t.Fatalf("expected swift-package detector, got %+v", result.Sources[0])
			}
			if result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", result.Sources[0].Analysis)
			}
		})
	}
}

func TestIOSPodfileFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name      string
		podCount  int
		mustMatch []string
	}{
		{name: "podfile-registry-and-versions", podCount: 8, mustMatch: []string{"'= 1.2.3'", "'> 2.0'", "'<= 4.5'", "'~> 1.2.3-beta.0'"}},
		{name: "podfile-targets-and-configurations", podCount: 5, mustMatch: []string{"pod 'RootInheritedKit'", ":configurations => ['Debug', 'Beta']", ":configuration => 'Release'"}},
		{name: "podfile-subspecs-and-options", podCount: 4, mustMatch: []string{"pod 'QueryKit/Attribute'", ":subspecs => ['Attribute', 'QuerySet']", ":testspecs => ['UnitTests']"}},
		{name: "podfile-local-path", podCount: 3, mustMatch: []string{":path => '../SharedUI'", "path: '~/Documents/ModernPathPod'"}},
		{name: "podfile-git-references", podCount: 5, mustMatch: []string{":branch => 'next'", ":tag => '2.1.0'", ":commit => '082f8319af'", "git: 'https://example.test/modern.git'"}},
		{name: "podfile-external-podspec", podCount: 2, mustMatch: []string{":podspec => 'https://example.test/specs/JSONKit.podspec'", "podspec: 'https://example.test/specs/ModernSpecPod.podspec'"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ios", fixture.name, "Podfile"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if got := strings.Count(string(content), "pod '"); got != fixture.podCount {
				t.Fatalf("expected %d pod declarations, got %d", fixture.podCount, got)
			}
			for _, token := range fixture.mustMatch {
				if !strings.Contains(string(content), token) {
					t.Fatalf("expected fixture to contain %q", token)
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "ios", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "ios-podfile" || source.Path != "Podfile" {
				t.Fatalf("expected ios-podfile source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestIOSCartfileFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name         string
		declarations []string
	}{
		{name: "cartfile-github-versions", declarations: []string{
			"github \"ReactiveCocoa/ReactiveCocoa\" >= 2.3.1", "github \"Mantle/Mantle\" ~> 1.0", "github \"jspahrsummers/libextobjc\" == 0.4.1", "github \"jspahrsummers/xcconfigs\"",
		}},
		{name: "cartfile-github-enterprise-and-ref", declarations: []string{
			"github \"https://enterprise.example.test/ghe/mobile/enterprise-kit\"", "github \"owner/branch-kit\" \"release/next\"", "github \"owner/tag-kit\" \"v2.4.0\"", "github \"owner/commit-kit\" \"9fceb02d0ae598e95dc970b74767f19372d61af8\"",
		}},
		{name: "cartfile-git-remote-and-local", declarations: []string{
			"git \"https://git.example.test/mobile/remote-kit.git\" \"development\"", "git \"ssh://git@git.example.test/mobile/ssh-kit.git\" == 3.2.1", "git \"file:///opt/checkouts/local-kit\" \"feature/local-api\"",
		}},
		{name: "cartfile-binary-sources", declarations: []string{
			"binary \"https://downloads.example.test/BinaryKit.json\" ~> 2.3", "binary \"file:///opt/artifacts/FileBinaryKit.json\" == 1.2.0", "binary \"vendor/RelativeBinaryKit.json\"", "binary \"/opt/artifacts/AbsoluteBinaryKit.json\" >= 4.0",
		}},
		{name: "cartfile-whitespace-and-comments", declarations: []string{
			"github \"owner/commented-kit\" ~> 1.2 // inline comment", "git \"https://git.example.test/mobile/spaced-kit.git\"    >= 0.9.0",
		}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ios", fixture.name, "Cartfile"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			text := string(content)
			var declarations []string
			for _, line := range strings.Split(text, "\n") {
				if strings.HasPrefix(line, "github ") || strings.HasPrefix(line, "git ") || strings.HasPrefix(line, "binary ") {
					declarations = append(declarations, line)
				}
			}
			if !slices.Equal(declarations, fixture.declarations) {
				t.Fatalf("expected declarations %+v, got %+v", fixture.declarations, declarations)
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "ios", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "ios-cartfile" || source.Path != "Cartfile" {
				t.Fatalf("expected ios-cartfile source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestRubyGemspecFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name         string
		file         string
		declarations []string
	}{
		{name: "gemspec-runtime-basic", file: "runtime-basic.gemspec", declarations: []string{"spec.add_dependency \"unconstrained-kit\"", "spec.add_dependency \"minimum-kit\", \">= 2.0\""}},
		{name: "gemspec-runtime-alias", file: "runtime-alias.gemspec", declarations: []string{"spec.add_runtime_dependency \"compatible-kit\", \"~> 3.1\"", "spec.add_runtime_dependency \"exact-kit\", \"= 4.2.0\""}},
		{name: "gemspec-development", file: "development.gemspec", declarations: []string{"spec.add_development_dependency \"rake\", \">= 13.0\"", "spec.add_development_dependency \"minitest\", \"~> 5.0\""}},
		{name: "gemspec-multiple-constraints", file: "multiple-constraints.gemspec", declarations: []string{"spec.add_runtime_dependency \"bounded-kit\", \">= 2.0\", \"< 4.0\", \"!= 2.2.1\"", "spec.add_dependency \"prerelease-kit\", \">= 3.0.0.a\", \"< 3.0.0\""}},
		{name: "gemspec-call-styles", file: "call-styles.gemspec", declarations: []string{"s.add_dependency(\"parenthesized-kit\", \">= 1.0\")", "s.add_dependency(\"array-kit\", [\">= 2.2.0\", \"< 3.0\"])", "s.add_development_dependency(\"array-dev-kit\", [\"~> 2.4\"])"}},
		{name: "gemspec-manual-dependencies", file: "manual-dependencies.gemspec", declarations: []string{"spec.dependencies << Gem::Dependency.new(\"manual-runtime\", \">= 1.0\", :runtime)", "spec.dependencies << Gem::Dependency.new(\"manual-development\", \"~> 2.0\", :development)"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ruby", fixture.name, fixture.file))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			text := string(content)
			var declarations []string
			for _, line := range strings.Split(text, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "spec.add_") || strings.HasPrefix(line, "s.add_") || strings.HasPrefix(line, "spec.dependencies <<") {
					declarations = append(declarations, line)
				}
			}
			if !slices.Equal(declarations, fixture.declarations) {
				t.Fatalf("expected declarations %+v, got %+v", fixture.declarations, declarations)
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "ruby", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "ruby-gemspec" || source.Path != fixture.file {
				t.Fatalf("expected ruby-gemspec source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestScalaSbtBuildFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name  string
		lines []string
	}{
		{name: "sbt-build-basic", lines: []string{"libraryDependencies += \"org.apache.derby\" % \"derby\" % \"10.4.1.3\""}},
		{name: "sbt-build-cross-and-batch", lines: []string{"libraryDependencies += \"org.scala-stm\" %% \"scala-stm\" % \"0.11.0\"", "libraryDependencies ++= Seq(\"org.typelevel\" %% \"cats-core\" % \"2.10.0\", \"com.typesafe\" % \"config\" % \"1.4.3\")"}},
		{name: "sbt-build-configurations", lines: []string{"libraryDependencies += \"org.scalatest\" %% \"scalatest\" % \"3.2.18\" % Test", "libraryDependencies += \"org.scalacheck\" %% \"scalacheck\" % \"1.18.0\" % \"test\"", "libraryDependencies += \"com.example\" % \"provided-api\" % \"1.0.0\" % Provided"}},
		{name: "sbt-build-modifiers", lines: []string{"libraryDependencies += (\"org.testng\" % \"testng\" % \"7.10.2\").classifier(\"jdk15\")", "libraryDependencies += (\"org.apache.felix\" % \"org.apache.felix.framework\" % \"1.8.0\").intransitive()", "libraryDependencies += (\"com.example\" % \"without-logging\" % \"2.0.0\").exclude(\"org.slf4j\", \"slf4j-api\")"}},
		{name: "sbt-build-crossversion-and-url", lines: []string{"libraryDependencies += ((\"org.typelevel\" % \"cats-core\" % \"2.10.0\") cross CrossVersion.for3Use2_13)", "libraryDependencies += \"slinky\" % \"slinky\" % \"2.1\" from \"https://downloads.example.test/slinky-2.1.jar\""}},
		{name: "sbt-build-scoped-and-mappings", lines: []string{"Test / libraryDependencies += \"org.scalatest\" %% \"scalatest\" % \"3.2.18\"", "Compile / libraryDependencies ++= Seq(\"com.example\" % \"compile-kit\" % \"1.0.0\", \"com.example\" % \"support-kit\" % \"1.1.0\")", "libraryDependencies += \"com.example\" % \"mapped-test-kit\" % \"2.0.0\" % \"test->compile\"", "libraryDependencies += \"com.example\" % \"integration-kit\" % \"3.0.0\" % \"it,test\""}},
		{name: "sbt-build-expanded-modifiers", lines: []string{"libraryDependencies += (\"org.apache.felix\" % \"org.apache.felix.framework\" % \"1.8.0\").notTransitive()", "libraryDependencies += (\"com.example\" % \"fully-excluded-kit\" % \"2.0.0\").excludeAll(ExclusionRule(\"org.slf4j\", \"slf4j-api\"))", "libraryDependencies += (\"org.lwjgl.lwjgl\" % \"lwjgl-platform\" % \"2.9.3\").classifier(\"natives-windows\").classifier(\"natives-linux\").classifier(\"natives-osx\")"}},
		{name: "sbt-build-replacement-and-indirection", lines: []string{"val scalatest = \"org.scalatest\" %% \"scalatest\" % \"3.2.18\"", "libraryDependencies := Seq(\"com.example\" % \"replacement-kit\" % \"1.0.0\")", "libraryDependencies += scalatest % Test"}},
		{name: "sbt-build-scalajs-cross", lines: []string{"libraryDependencies += \"org.scala-js\" %%% \"scalajs-dom\" % \"2.8.0\""}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "scala", fixture.name, "build.sbt"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var lines []string
			for _, line := range strings.Split(string(content), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "//") {
					lines = append(lines, line)
				}
			}
			for _, expected := range fixture.lines {
				if !slices.Contains(lines, expected) {
					t.Fatalf("expected fixture line %q, got %+v", expected, lines)
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "scala", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "scala-sbt-build" || source.Path != "build.sbt" {
				t.Fatalf("expected scala-sbt-build source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestScalaSbtPluginFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name         string
		declarations []string
	}{
		{name: "sbt-plugins-basic", declarations: []string{"addSbtPlugin(\"com.eed3si9n\" % \"sbt-assembly\" % \"2.3.1\")"}},
		{name: "sbt-plugins-multiple-and-resolver", declarations: []string{"addSbtPlugin(\"com.typesafe.sbt\" % \"sbt-site\" % \"1.6.0\")", "addSbtPlugin(\"org.scoverage\" % \"sbt-scoverage\" % \"2.1.1\")", "resolvers ++= Resolver.sonatypeOssRepos(\"public\")"}},
		{name: "sbt-plugins-build-library", declarations: []string{"libraryDependencies += \"org.example\" % \"build-utilities\" % \"1.3.0\"", "libraryDependencies ++= Seq(\"com.typesafe\" % \"config\" % \"1.4.3\", \"org.slf4j\" % \"slf4j-api\" % \"2.0.13\")", "libraryDependencies += \"org.example\" %% \"cross-built-utility\" % \"1.0.0\" % Test"}},
		{name: "sbt-plugins-source-project", declarations: []string{"lazy val root = (project in file(\".\")).dependsOn(assemblyPlugin)", "lazy val assemblyPlugin = RootProject(uri(\"git://github.com/sbt/sbt-assembly#v2.3.1\"))"}},
		{name: "sbt-plugins-source-subproject", declarations: []string{"lazy val root = (project in file(\".\")).dependsOn(pluginSubproject)", "lazy val pluginSubproject = ProjectRef(uri(\"https://example.test/sbt/multi-plugin.git#v1.0.0\"), \"plugin\")"}},
		{name: "sbt-plugins-computed-version", declarations: []string{"val assemblyVersion = sys.props.getOrElse(\"plugin.version\", \"2.3.1\")", "addSbtPlugin(\"com.eed3si9n\" % \"sbt-assembly\" % assemblyVersion)"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "scala", fixture.name, "project", "plugins.sbt"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var declarations []string
			for _, line := range strings.Split(string(content), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "addSbtPlugin(") || strings.HasPrefix(line, "libraryDependencies ") || strings.HasPrefix(line, "resolvers ") || strings.HasPrefix(line, "lazy val root ") || strings.HasPrefix(line, "lazy val assemblyPlugin ") || strings.HasPrefix(line, "lazy val pluginSubproject ") || strings.HasPrefix(line, "val assemblyVersion ") {
					declarations = append(declarations, line)
				}
			}
			if !slices.Equal(declarations, fixture.declarations) {
				t.Fatalf("expected declarations %+v, got %+v", fixture.declarations, declarations)
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "scala", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "scala-sbt-plugins" || source.Path != "project/plugins.sbt" {
				t.Fatalf("expected scala-sbt-plugins source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestScalaMillBuildFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name         string
		declarations []string
	}{
		{name: "mill-build-ivy-java", declarations: []string{"def ivyDeps = Agg(ivy\"org.slf4j:slf4j-api:2.0.13\", ivy\"com.fasterxml.jackson.core:jackson-databind:2.17.2\")"}},
		{name: "mill-build-ivy-scala-cross", declarations: []string{"def ivyDeps = Agg(ivy\"com.lihaoyi::os-lib:0.10.2\", ivy\"org.scalamacros:::paradise:2.1.1\")"}},
		{name: "mill-build-dependency-scopes", declarations: []string{"def compileIvyDeps = Agg(ivy\"jakarta.servlet:jakarta.servlet-api:6.1.0\")", "def runIvyDeps = Agg(ivy\"ch.qos.logback:logback-classic:1.5.7\")"}},
		{name: "mill-build-classifier-and-exclude", declarations: []string{"def ivyDeps = Agg(ivy\"org.apache.spark::spark-sql:3.5.2;classifier=tests\", ivy\"com.example:without-logging:1.0.0\".exclude(\"org.slf4j\" -> \"slf4j-api\").excludeOrg(\"com.unwanted\").excludeName(\"legacy-api\"), ivy\"com.lihaoyi::fansi:0.2.14\".forceVersion())"}},
		{name: "mill-build-module-and-meta-import", declarations: []string{"import $ivy.`com.lihaoyi::scalatags:0.12.0`", "import $ivy.`org.thymeleaf:thymeleaf:3.1.1.RELEASE`", "import $ivy.`com.lihaoyi::mill-contrib-bloop:$MILL_VERSION`", "import $ivy.`com.lihaoyi::mill-contrib-versionfile:`", "def moduleDeps = Seq(util)", "def ivyDeps = Agg(ivy\"com.lihaoyi::mainargs:0.7.6\")"}},
		{name: "mill-build-compiler-plugin", declarations: []string{"def compileIvyDeps = Agg(ivy\"org.scala-lang:scala-reflect:2.12.14\")", "def scalacPluginIvyDeps = Agg(ivy\"org.scalamacros:::paradise:2.1.1\")"}},
		{name: "mill-build-test-modules", declarations: []string{"def ivyDeps = Agg(ivy\"org.scalatest::scalatest:3.2.18\")", "def moduleDeps = super.moduleDeps ++ Seq(test)", "def ivyDeps = Agg(ivy\"com.typesafe::config:1.4.3\")"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "scala", fixture.name, "build.sc"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var declarations []string
			for _, line := range strings.Split(string(content), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "import $ivy.") || strings.HasPrefix(line, "def ivyDeps ") || strings.HasPrefix(line, "def compileIvyDeps ") || strings.HasPrefix(line, "def runIvyDeps ") || strings.HasPrefix(line, "def scalacPluginIvyDeps ") || strings.HasPrefix(line, "def moduleDeps ") {
					declarations = append(declarations, line)
				}
			}
			if !slices.Equal(declarations, fixture.declarations) {
				t.Fatalf("expected declarations %+v, got %+v", fixture.declarations, declarations)
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "scala", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "scala-mill" || source.Path != "build.sc" {
				t.Fatalf("expected scala-mill source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestScalaSbtDependenciesFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name         string
		declarations []string
		fragments    []string
	}{
		{name: "sbt-dependencies-basic", declarations: []string{"val derby = \"org.apache.derby\" % \"derby\" % \"10.4.1.3\"", "val catsCore = \"org.typelevel\" %% \"cats-core\" % \"2.10.0\""}},
		{name: "sbt-dependencies-versions-and-groups", declarations: []string{"lazy val akkaVersion = \"2.6.21\"", "lazy val testVersion = \"3.2.18\"", "val akkaActor = \"com.typesafe.akka\" %% \"akka-actor\" % akkaVersion", "val scalaTest = \"org.scalatest\" %% \"scalatest\" % testVersion", "val backendDeps = Seq(akkaActor, scalaTest % Test)"}},
		{name: "sbt-dependencies-configurations", declarations: []string{"val testOnly = \"org.scalacheck\" %% \"scalacheck\" % \"1.18.0\" % \"test\"", "val integration = \"com.example\" % \"integration-api\" % \"1.0.0\" % \"it,test\"", "val providedApi = \"jakarta.servlet\" % \"jakarta.servlet-api\" % \"6.1.0\" % Provided"}},
		{name: "sbt-dependencies-modifiers", declarations: []string{"val testNg = (\"org.testng\" % \"testng\" % \"7.10.2\").classifier(\"jdk15\")", "val noTransitively = (\"org.apache.felix\" % \"org.apache.felix.framework\" % \"1.8.0\").notTransitive()", "val withoutLogging = (\"com.example\" % \"without-logging\" % \"2.0.0\").excludeAll(ExclusionRule(\"org.slf4j\", \"slf4j-api\"))"}},
		{name: "sbt-dependencies-cross-and-url", declarations: []string{"val scala2LibraryOnScala3 = (\"org.typelevel\" % \"cats-core\" % \"2.10.0\").cross(CrossVersion.for3Use2_13)", "val downloadedJar = (\"slinky\" % \"slinky\" % \"2.1\").from(\"https://downloads.example.test/slinky-2.1.jar\")"}},
		{name: "sbt-dependencies-moduleid", declarations: []string{"val positional = ModuleID(\"com.example\", \"positional-artifact\", \"1.0.0\")", "val named = ModuleID(organization = \"com.example\", name = \"named-artifact\", revision = \"2.0.0\")"}},
		{name: "sbt-dependencies-inline-sequences", declarations: []string{"lazy val commonVersion = \"1.0.0\"", "val core: Seq[ModuleID] = Seq(\"com.example\" % \"range-kit\" % \"[1.0,)\", ModuleID(\"com.example\", \"direct-kit\", commonVersion))", "val all = core ++ List[ModuleID](\"com.example\" %% \"latest-kit\" % \"latest.integration\", \"com.example\" % \"plus-kit\" % \"2.9.+\", \"com.example\" % \"mapped-kit\" % \"3.0.0\" % \"test->compile\")"}},
		{name: "sbt-dependencies-extended-modifiers", declarations: []string{"val excluded = (\"com.example\" % \"exclude-kit\" % \"1.0.0\").exclude(\"org.slf4j\", \"slf4j-api\")", "val documented = (\"com.example\" % \"docs-kit\" % \"2.0.0\").withSources().withJavadoc().extra(\"build\" -> \"fixture\").force()", "val fullyExcluded = (\"com.example\" % \"multi-exclude-kit\" % \"3.0.0\").excludeAll("}, fragments: []string{"ExclusionRule(\"org.slf4j\", \"slf4j-api\")", "ExclusionRule(\"commons-logging\", \"commons-logging\")", ")"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "scala", fixture.name, "project", "Dependencies.scala"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var declarations []string
			for _, line := range strings.Split(string(content), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "val ") || strings.HasPrefix(line, "lazy val ") {
					declarations = append(declarations, line)
				}
			}
			if !slices.Equal(declarations, fixture.declarations) {
				t.Fatalf("expected declarations %+v, got %+v", fixture.declarations, declarations)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "scala", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "scala-sbt-dependencies" || source.Path != "project/Dependencies.scala" {
				t.Fatalf("expected scala-sbt-dependencies source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestJavaIvyFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name            string
		dependencyCount int
		fragments       []string
	}{
		{name: "ivy", dependencyCount: 1, fragments: []string{`<dependency org="commons-lang" name="commons-lang" rev="2.6"/>`}},
		{name: "ivy-basic-and-dynamic", dependencyCount: 4, fragments: []string{`rev="3.14.0"`, `rev="latest.integration"`, `rev="2.+"`, `<dependency name="same-organisation-api" rev="1.0.0"/>`}},
		{name: "ivy-configurations", dependencyCount: 3, fragments: []string{`defaultconf="default"`, `defaultconfmapping="default->default"`, `conf="default->@;runtime,test->runtime"`, `<conf name="test" mapped="runtime"/>`, `conf="*->@"`}},
		{name: "ivy-force-and-transitivity", dependencyCount: 3, fragments: []string{`revConstraint="[2.0,3.0[" force="true"`, `transitive="false"`, `changing="true" branch="main"`}},
		{name: "ivy-artifacts", dependencyCount: 2, fragments: []string{`<artifact name="spark-sql_2.13" type="jar" ext="jar" conf="default"/>`, `<include name="spark-sql_2.13" type="jar" ext="jar" conf="default"/>`, `e:classifier="windows-x86_64"`}},
		{name: "ivy-excludes", dependencyCount: 1, fragments: []string{`<exclude org="commons-logging" module="commons-logging"/>`, `<exclude org="org.slf4j" module="slf4j-api"/>`, `<exclude module="spring-jcl" type="jar"/>`}},
		{name: "ivy-optional-and-extra-attributes", dependencyCount: 2, fragments: []string{`e:optional="true" e:classifier="tests"`, `rev="[3.0,4.0[" e:target="jvm-21"`}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "java", fixture.name, "ivy.xml"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if count := strings.Count(string(content), "<dependency "); count != fixture.dependencyCount {
				t.Fatalf("expected %d dependency declarations, got %d", fixture.dependencyCount, count)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "java", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "java-ivy" || source.Path != "ivy.xml" {
				t.Fatalf("expected java-ivy source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestJavaAntBuildFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name        string
		fragments   []string
		occurrences map[string]int
	}{
		{name: "ant-build", fragments: []string{`<javac srcdir="src" destdir="build"/>`}, occurrences: map[string]int{"<javac ": 1}},
		{name: "ant-build-path-elements", fragments: []string{`<pathelement location="lib/guava-33.3.0-jre.jar"/>`, `<pathelement location="lib/slf4j-api-2.0.13.jar"/>`, `classpathref="compile.classpath"`}, occurrences: map[string]int{"<pathelement ": 3}},
		{name: "ant-build-filesets", fragments: []string{`<fileset dir="lib">`, `<include name="*.jar"/>`, `<exclude name="*-sources.jar"/>`, `<fileset dir="vendor" includes="**/*.jar" excludes="**/*-tests.jar"/>`}, occurrences: map[string]int{"<fileset ": 2}},
		{name: "ant-build-nested-classpaths", fragments: []string{`<javac srcdir="src" destdir="build/classes" includeantruntime="false">`, `<java classname="com.example.Main" fork="true">`, `<fileset dir="lib/runtime" includes="*.jar"/>`}, occurrences: map[string]int{"<classpath>": 2, "<fileset ": 2, "<pathelement ": 2}},
		{name: "ant-build-taskdef-classpath", fragments: []string{`<taskdef resource="com/puppycrawl/tools/checkstyle/ant/checkstyle-ant-task.properties"`, `classpathref="tools.classpath"`, `loaderref="tools.classloader"`, `classname="com.example.ant.GenerateParserTask"`}, occurrences: map[string]int{"<taskdef ": 2, "<pathelement ": 2}},
		{name: "ant-build-ivy-cachepath", fragments: []string{`xmlns:ivy="antlib:org.apache.ivy.ant"`, `<ivy:cachepath organisation="org.apache.ivy" module="ivy" revision="2.5.2" inline="true" conf="default" pathid="ivy.classpath"/>`, `<ivy:cachepath organisation="org.slf4j" module="slf4j-api" revision="2.0.13" inline="true" conf="default" pathid="compile.classpath"/>`}, occurrences: map[string]int{"<ivy:cachepath ": 2}},
		{name: "ant-build-ivy-retrieve", fragments: []string{`<ivy:retrieve organisation="com.fasterxml.jackson.core"`, `module="jackson-databind"`, `revision="2.17.2"`, `pattern="lib/[orgPath]/[artifact]-[revision].[ext]"`, `pathId="runtime.classpath"`}, occurrences: map[string]int{"<ivy:retrieve ": 1}},
		{name: "ant-build-modulepath", fragments: []string{`<pathelement location="modules/jackson-databind-2.17.2.jar"/>`, `<fileset dir="modules/optional" includes="*.jar"/>`, `modulepathref="application.modules"`}, occurrences: map[string]int{"<pathelement ": 1, "<fileset ": 1}},
		{name: "ant-build-classpath-attributes", fragments: []string{`classpath="lib/guava-33.3.0-jre.jar${path.separator}lib/jsr305-3.0.2.jar"`, `classpath="build/classes${path.separator}lib/logback-classic-1.5.7.jar"`}, occurrences: map[string]int{"classpath=": 2}},
		{name: "ant-build-ivy-resolve", fragments: []string{`<ivy:resolve conf="default" transitive="true" keep="true">`, `<dependency org="org.apache.commons" name="commons-lang3" rev="3.14.0"/>`, `<dependency org="org.slf4j" name="slf4j-api" rev="2.0.13" conf="default"/>`, `<exclude org="commons-logging" module="commons-logging"/>`}, occurrences: map[string]int{"<ivy:resolve ": 1, "<dependency ": 2, "<exclude ": 1}},
		{name: "ant-build-path-composition", fragments: []string{`<path id="base.classpath" location="lib/base-library-1.0.0.jar"/>`, `<path refid="base.classpath"/>`, `<pathelement path="lib/extension-one-2.0.0.jar${path.separator}lib/extension-two-2.0.0.jar"/>`}, occurrences: map[string]int{"<path ": 3, "<pathelement ": 1}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "java", fixture.name, "build.xml"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			for declaration, expectedCount := range fixture.occurrences {
				if count := strings.Count(string(content), declaration); count != expectedCount {
					t.Fatalf("expected %d occurrences of %q, got %d", expectedCount, declaration, count)
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "java", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "java-ant-build" || source.Path != "build.xml" {
				t.Fatalf("expected java-ant-build source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestCppCMakeFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name        string
		fragments   []string
		occurrences map[string]int
	}{
		{name: "cmake", fragments: []string{`find_package(Boost REQUIRED COMPONENTS filesystem)`}, occurrences: map[string]int{"find_package(": 1}},
		{name: "cmake-find-package-versions", fragments: []string{`find_package(Boost 1.82 REQUIRED COMPONENTS filesystem program_options)`, `find_package(fmt 10.2 EXACT CONFIG REQUIRED)`, `find_package(Catch2 3.5 QUIET OPTIONAL_COMPONENTS Catch2WithMain)`}, occurrences: map[string]int{"find_package(": 3}},
		{name: "cmake-find-package-search", fragments: []string{`find_package(ZLIB MODULE REQUIRED)`, `NAMES MyLibrary my-library`, `HINTS "${CMAKE_CURRENT_LIST_DIR}/third_party/install"`, `PATH_SUFFIXES lib/cmake/MyLibrary`, `NO_DEFAULT_PATH`, `find_package(nlohmann_json 3.11...<4 CONFIG QUIET GLOBAL)`}, occurrences: map[string]int{"find_package(": 3}},
		{name: "cmake-fetchcontent-git", fragments: []string{`include(FetchContent)`, `GIT_REPOSITORY https://github.com/fmtlib/fmt.git`, `GIT_TAG 10.2.1`, `FIND_PACKAGE_ARGS CONFIG`, `OVERRIDE_FIND_PACKAGE`, `FetchContent_MakeAvailable(fmt googletest)`}, occurrences: map[string]int{"FetchContent_Declare(": 2}},
		{name: "cmake-fetchcontent-url", fragments: []string{`URL https://github.com/nlohmann/json/releases/download/v3.11.3/json.tar.xz`, `URL_HASH SHA256=d6c65aca6b1ed68e7a182f4757257b107ae403032760ed6ef121c9d55e81757d`, `DOWNLOAD_EXTRACT_TIMESTAMP TRUE`, `EXCLUDE_FROM_ALL`}, occurrences: map[string]int{"FetchContent_Declare(": 1}},
		{name: "cmake-fetchcontent-populate", fragments: []string{`GIT_REPOSITORY https://github.com/Tencent/rapidjson.git`, `GIT_TAG v1.1.0`, `FetchContent_GetProperties(rapidjson)`, `FetchContent_Populate(rapidjson)`, `add_subdirectory("${rapidjson_SOURCE_DIR}" "${rapidjson_BINARY_DIR}" EXCLUDE_FROM_ALL)`}, occurrences: map[string]int{"FetchContent_Declare(": 1, "FetchContent_Populate(": 1}},
		{name: "cmake-external-project", fragments: []string{`GIT_REPOSITORY https://github.com/madler/zlib.git`, `GIT_SHALLOW TRUE`, `URL https://github.com/jarro2783/cxxopts/archive/v2.2.0.tar.gz`, `URL_HASH SHA256=447dbfc2361fce9742c5d1c9cfb25731c977b405f9085a738fbd608626da8a4d`, `SVN_REPOSITORY https://svn.apache.org/repos/asf/apr/apr/tags/1.7.5`}, occurrences: map[string]int{"ExternalProject_Add(": 3}},
		{name: "cmake-subdirectories", fragments: []string{`add_subdirectory(third_party/spdlog EXCLUDE_FROM_ALL)`, `add_subdirectory(vendor/catch2 "${CMAKE_BINARY_DIR}/vendor/catch2")`, `add_subdirectory("${CMAKE_CURRENT_SOURCE_DIR}/plugins/local-library")`}, occurrences: map[string]int{"add_subdirectory(": 3}},
		{name: "cmake-pkg-config", fragments: []string{`find_package(PkgConfig REQUIRED)`, `pkg_check_modules(GLIB REQUIRED IMPORTED_TARGET GLOBAL glib-2.0>=2.76 gio-2.0)`, `pkg_search_module(XML REQUIRED IMPORTED_TARGET libxml-2.0 libxml2)`}, occurrences: map[string]int{"find_package(": 1, "pkg_check_modules(": 1, "pkg_search_module(": 1}},
		{name: "cmake-find-dependency", fragments: []string{`include(CMakeFindDependencyMacro)`, `find_dependency(fmt 10.2 CONFIG REQUIRED)`, `find_dependency(ZLIB 1.3 REQUIRED)`}, occurrences: map[string]int{"find_dependency(": 2}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "cpp", fixture.name, "CMakeLists.txt"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			for declaration, expectedCount := range fixture.occurrences {
				if count := strings.Count(string(content), declaration); count != expectedCount {
					t.Fatalf("expected %d occurrences of %q, got %d", expectedCount, declaration, count)
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "cpp", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "cpp-cmake" || source.Path != "CMakeLists.txt" {
				t.Fatalf("expected cpp-cmake source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestCppConanfilePyFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name        string
		fragments   []string
		occurrences map[string]int
	}{
		{name: "conanfile-py", fragments: []string{`requires = "boost/1.82.0", "fmt/10.1.1"`}, occurrences: map[string]int{"\n    requires = ": 1}},
		{name: "conanfile-py-attribute-scopes", fragments: []string{`requires = "fmt/10.2.1", "boost/1.84.0"`, `tool_requires = "cmake/3.29.0", "ninja/1.12.1"`, `test_requires = "gtest/1.14.0"`}, occurrences: map[string]int{"\n    requires = ": 1, "\n    tool_requires = ": 1, "\n    test_requires = ": 1}},
		{name: "conanfile-py-method-ranges", fragments: []string{`self.requires("zlib/1.3.1")`, `self.requires("fmt/[>=10.0 <11]")`, `self.requires("nlohmann_json/[>=3.11 <4]")`, `self.requires("spdlog/[~1.14]")`, `self.requires("experimental/[>=2.0 <3.0, include_prerelease]")`}, occurrences: map[string]int{"self.requires(": 5}},
		{name: "conanfile-py-requirement-traits", fragments: []string{`headers=True, libs=False, transitive_headers=True`, `run=True, transitive_libs=True`, `options={"shared": False}`, `visible=False, no_skip=True`}, occurrences: map[string]int{"self.requires(": 4}},
		{name: "conanfile-py-overrides", fragments: []string{`self.requires("zlib/1.3.1", force=True)`, `self.requires("openssl/3.3.1", override=True)`}, occurrences: map[string]int{"self.requires(": 2}},
		{name: "conanfile-py-build-requirements", fragments: []string{`self.tool_requires("cmake/3.29.0", options={"shared": False})`, `self.tool_requires("ninja/1.12.1", package_id_mode="minor_mode")`, `self.test_requires("gtest/1.14.0", force=True)`, `self.tool_requires("nasm/2.16.01", override=True)`}, occurrences: map[string]int{"self.tool_requires(": 3, "self.test_requires(": 1}},
		{name: "conanfile-py-host-version", fragments: []string{`self.requires("protobuf/5.27.0")`, `self.requires("gettext/0.22.5")`, `self.tool_requires("protobuf/<host_version>")`, `self.tool_requires("libgettext/<host_version:gettext>")`}, occurrences: map[string]int{"self.requires(": 2, "self.tool_requires(": 2}},
		{name: "conanfile-py-python-requires", fragments: []string{`python_requires = "base_recipes/1.2@company/stable", "packaging_tools/2.0@company/stable"`, `python_requires_extend = "base_recipes.BaseConanfile", "packaging_tools.CMakeLayout"`, `requires = "fmt/10.2.1"`}, occurrences: map[string]int{"\n    python_requires = ": 1, "\n    python_requires_extend = ": 1, "\n    requires = ": 1}},
		{name: "conanfile-py-legacy-build-requires", fragments: []string{`from conans import ConanFile`, `requires = "boost/1.82.0@conan/stable"`, `build_requires = "cmake/3.22.6@conan/stable", "ninja/1.11.1@conan/stable"`}, occurrences: map[string]int{"\n    requires = ": 1, "\n    build_requires = ": 1}},
		{name: "conanfile-py-legacy-methods", fragments: []string{`self.requires("boost/1.82.0@conan/stable")`, `self.requires("winpthreads/1.0@conan/stable")`, `self.build_requires("cmake/3.22.6@conan/stable")`, `self.build_requires("gtest/1.11.0@conan/stable", force_host_context=True)`}, occurrences: map[string]int{"self.requires(": 2, "self.build_requires(": 2}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "cpp", fixture.name, "conanfile.py"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			for declaration, expectedCount := range fixture.occurrences {
				if count := strings.Count(string(content), declaration); count != expectedCount {
					t.Fatalf("expected %d occurrences of %q, got %d", expectedCount, declaration, count)
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "cpp", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "cpp-conanfile-py" || source.Path != "conanfile.py" {
				t.Fatalf("expected cpp-conanfile-py source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestCppConanfileAndLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, filename string
		detector       DetectorID
		required       []string
	}{
		{"conanfile-basic", "conanfile.txt", "cpp-conanfile", []string{"[requires]", "fmt/10.2.1", "CMakeToolchain"}},
		{"conanfile-ranges-revisions", "conanfile.txt", "cpp-conanfile", []string{"boost/[>=1.84 <1.87]", "fmt/10.2.1#e747928f85b03f48aaf227ff897d9634"}},
		{"conanfile-tools-tests", "conanfile.txt", "cpp-conanfile", []string{"[tool_requires]", "cmake/3.29.6", "[test_requires]", "gtest/1.14.0"}},
		{"conanfile-options-layout", "conanfile.txt", "cpp-conanfile", []string{"poco/*:shared=True", "[layout]", "cmake_layout"}},
		{"conanfile-legacy-build-requires", "conanfile.txt", "cpp-conanfile", []string{"[build_requires]", "cmake/3.22.6@conan/stable"}},
		{"conanfile-tool-test-ranges", "conanfile.txt", "cpp-conanfile", []string{"[tool_requires]", "cmake/[>=3.27 <4]", "[test_requires]", "gtest/[>=1.12 <2]"}},
		{"conan-lock-basic", "conan.lock", "cpp-conan-lock", []string{"\"version\": \"0.5\"", "\"fmt/10.2.1\""}},
		{"conan-lock-revisions", "conan.lock", "cpp-conan-lock", []string{"#13c96f538b52e1600c40b88994de240f%1667396813.733", "\"build_requires\""}},
		{"conan-lock-contexts", "conan.lock", "cpp-conan-lock", []string{"\"python_requires\"", "company_build/2.4.0"}},
		{"conan-lock-multiversion", "conan.lock", "cpp-conan-lock", []string{"matrix/1.1", "matrix/1.0"}},
		{"conan-lock-partial", "conan.lock", "cpp-conan-lock", []string{"\"engine/2.0\"", "\"recipe_helpers/1.0\""}},
		{"conan-lock-config-requires", "conan.lock", "cpp-conan-lock", []string{"\"config_requires\"", "company_config/2.0@company/stable"}},
		{"conan-lock-legacy-graph", "conan.lock", "cpp-conan-lock", []string{"\"version\": \"0.4\"", "\"graph_lock\"", "cmake/3.22.6@conan/stable"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "cpp", fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range fixture.required {
				if !strings.Contains(string(contents), expected) {
					t.Errorf("fixture is missing %q", expected)
				}
			}
			if fixture.detector == "cpp-conan-lock" {
				assertConanLockStructure(t, contents)
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.filename || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestElixirMixFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, filename string
		detector       DetectorID
		required       []string
	}{
		{"mix-basic", "mix.exs", "elixir-mix", []string{"{:plug_cowboy, \"~> 2.7\"}", "{:jason, \"~> 1.4\"}"}},
		{"mix-options", "mix.exs", "elixir-mix", []string{"only: [:dev, :test]", "optional: true", "targets: [:host, :rpi3]"}},
		{"mix-git", "mix.exs", "elixir-mix", []string{"git: \"https://github.com/elixir-gettext/gettext.git\"", "github: \"dashbitco/nimble_parsec\"", "depth: 1"}},
		{"mix-local-umbrella", "mix.exs", "elixir-mix", []string{"path: \"../shared\"", "in_umbrella: true"}},
		{"mix-hex-private", "mix.exs", "elixir-mix", []string{"hex: :company_client", "repo: \"company\"", "manager: :rebar3"}},
		{"mix-git-sparse", "mix.exs", "elixir-mix", []string{"sparse: \"apps/component\"", "submodules: true"}},
		{"mix-regex-options", "mix.exs", "elixir-mix", []string{"~r/^1\\.(2|3)\\./", "compile: \"make\"", "warn_if_outdated: false"}},
		{"mix-lock-basic", "mix.lock", "elixir-mix-lock", []string{"\"jason\" => {:hex", "\"1.4.4\""}},
		{"mix-lock-transitive", "mix.lock", "elixir-mix-lock", []string{"{:mime, \"~> 1.0 or ~> 2.0\"", "optional: false"}},
		{"mix-lock-git", "mix.lock", "elixir-mix-lock", []string{"{:git, \"https://github.com/elixir-gettext/gettext.git\"", "[branch: \"main\"]"}},
		{"mix-lock-mixed", "mix.lock", "elixir-mix-lock", []string{"ssh://git@example.test", "[ref: \"0123456789abcdef0123456789abcdef01234567\"]"}},
		{"mix-lock-empty", "mix.lock", "elixir-mix-lock", []string{"%{}"}},
		{"mix-lock-git-checkout", "mix.lock", "elixir-mix-lock", []string{"sparse: \"apps/component\"", "submodules: true", "depth: 1"}},
		{"mix-lock-legacy-hex", "mix.lock", "elixir-mix-lock", []string{"\"poison\" => {:hex", "[:mix], []}"}},
		{"mix-lock-private-hex", "mix.lock", "elixir-mix-lock", []string{"\"company_client\" => {:hex, :company_api", "\"hexpm:acme\"", "optional: true"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "elixir", fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range fixture.required {
				if !strings.Contains(string(contents), expected) {
					t.Errorf("fixture is missing %q", expected)
				}
			}
			if fixture.detector == "elixir-mix-lock" {
				assertMixLockStructure(t, string(contents))
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			wantSources := 1
			if fixture.name == "mix-local-umbrella" {
				wantSources = 2
			}
			found := false
			for _, source := range result.Sources {
				if source.Detector == fixture.detector && source.Path == fixture.filename && source.Analysis == (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
					found = true
				}
			}
			if len(result.Sources) != wantSources || !found {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestPerlCPANfileFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, filename string
		detector       DetectorID
		required       []string
	}{
		{"cpanfile-basic", "cpanfile", "perl-cpanfile", []string{"requires 'Plack', '>= 1.0050';", "requires 'File::Spec';"}},
		{"cpanfile-ranges-relationships", "cpanfile", "perl-cpanfile", []string{"Types::Standard', '== 2.006000'", "Try::Tiny', '>= 0.30, != 0.31'", "conflicts 'JSON'"}},
		{"cpanfile-phases", "cpanfile", "perl-cpanfile", []string{"on 'configure'", "on 'test'", "on 'runtime'", "on 'develop'"}},
		{"cpanfile-features", "cpanfile", "perl-cpanfile", []string{"feature 'sqlite', 'SQLite support'", "feature 'metrics'"}},
		{"cpanfile-shortcuts", "cpanfile", "perl-cpanfile", []string{"configure_requires", "build_requires", "test_requires", "author_requires"}},
		{"cpanfile-options", "cpanfile", "perl-cpanfile", []string{"dist => 'MIYAGAWA/Plack-1.0050.tar.gz'", "mirror => 'https://cpan.example.test/'"}},
		{"cpanfile-snapshot-basic", "cpanfile.snapshot", "perl-cpanfile-snapshot", []string{"CARTON: 1.0.35", "MIYAGAWA/Plack-1.0050.tar.gz"}},
		{"cpanfile-snapshot-multiple", "cpanfile.snapshot", "perl-cpanfile-snapshot", []string{"JSON-MaybeXS-1.004008", "Moo::Object 2.005005"}},
		{"cpanfile-snapshot-undef", "cpanfile.snapshot", "perl-cpanfile-snapshot", []string{"AE undef", "AnyEvent::Loop undef"}},
		{"cpanfile-snapshot-core", "cpanfile.snapshot", "perl-cpanfile-snapshot", []string{"CPAN::Meta::Requirements", "strict 0", "warnings 0"}},
		{"cpanfile-snapshot-empty", "cpanfile.snapshot", "perl-cpanfile-snapshot", []string{"DISTRIBUTIONS"}},
		{"cpanfile-snapshot-empty-requirements", "cpanfile.snapshot", "perl-cpanfile-snapshot", []string{"PkgConfig-0.25026", "requirements:"}},
		{"cpanfile-snapshot-many-provides", "cpanfile.snapshot", "perl-cpanfile-snapshot", []string{"AE::Log::COLLECT undef", "Data::Section::Simple 0.07"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "perl", fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range fixture.required {
				if !strings.Contains(string(contents), expected) {
					t.Errorf("fixture is missing %q", expected)
				}
			}
			if fixture.detector == "perl-cpanfile-snapshot" {
				assertCPANfileSnapshotStructure(t, string(contents))
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.filename || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestPerlBuildDefinitionFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, filename string
		detector       DetectorID
		required       []string
	}{
		{"makefile-pl-basic", "Makefile.PL", "perl-makefile-pl", []string{"PREREQ_PM", "'Moo'        => '>= 2.005000'"}},
		{"makefile-pl-phases", "Makefile.PL", "perl-makefile-pl", []string{"CONFIGURE_REQUIRES", "BUILD_REQUIRES", "TEST_REQUIRES"}},
		{"makefile-pl-meta-merge", "Makefile.PL", "perl-makefile-pl", []string{"META_MERGE", "recommends => {'Cpanel::JSON::XS' => '4.37'}", "suggests   => {'JSON::XS' => '4.03'}", "test => {requires"}},
		{"makefile-pl-perl-ranges", "Makefile.PL", "perl-makefile-pl", []string{"MIN_PERL_VERSION => '5.020'", "!= 2.001000", "'Version::Next' => v1.2.3"}},
		{"makefile-pl-conditional", "Makefile.PL", "perl-makefile-pl", []string{"$^O eq 'MSWin32'", "PREREQ_PM => \\%prereqs"}},
		{"makefile-pl-meta-add", "Makefile.PL", "perl-makefile-pl", []string{"META_ADD", "'HTTP::Tiny' => '0.080'", "suggests => {'IO::Socket::SSL' => '2.085'}"}},
		{"build-pl-basic", "Build.PL", "perl-build-pl", []string{"requires    =>", "'Moo'        => '>= 2.005000'"}},
		{"build-pl-prereqs", "Build.PL", "perl-build-pl", []string{"configure_requires", "test_requires", "conflicts"}},
		{"build-pl-ranges", "Build.PL", "perl-build-pl", []string{"perl         => '>= 5.020, < 5.041'", "!= 2.001000"}},
		{"build-pl-prereq-alias", "Build.PL", "perl-build-pl", []string{"prereq      =>", "'URI'        => '5.29'"}},
		{"build-pl-functions", "Build.PL", "perl-build-pl", []string{"use Module::Build::Functions", "requires 'File::Spec';", "test_requires 'Test::More'"}},
		{"build-pl-conditional", "Build.PL", "perl-build-pl", []string{"$^O eq 'MSWin32'", "requires    => \\%requires"}},
		{"build-pl-meta-merge", "Build.PL", "perl-build-pl", []string{"meta_merge", "runtime => {requires", "develop => {requires"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "perl", fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range fixture.required {
				if !strings.Contains(string(contents), expected) {
					t.Errorf("fixture is missing %q", expected)
				}
			}
			assertPerlBuildDefinitionShape(t, fixture.filename, string(contents))
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.filename || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestPerlMetaAndDistIniFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, filename string
		detector       DetectorID
		required       []string
	}{
		{"meta-json", "META.json", "perl-meta", []string{"\"meta-spec\"", "\"generated_by\""}},
		{"meta-json-basic", "META.json", "perl-meta", []string{"\"abstract\"", "\"runtime\"", "\"Moo\": \"2.005005\""}},
		{"meta-json-phases", "META.json", "perl-meta", []string{"\"configure\"", "\"develop\"", "\"conflicts\""}},
		{"meta-json-features", "META.json", "perl-meta", []string{"\"optional_features\"", "\"DBD::SQLite\"", "\"Net::Prometheus\""}},
		{"meta-yaml-modern", "META.yml", "perl-meta", []string{"abstract:", "release_status: stable", "suggests:", "Test2::V0"}},
		{"meta-yaml-legacy", "META.yaml", "perl-meta", []string{"version: 1.4", "build_requires:", "configure_requires:"}},
		{"dist-ini-runtime", "dist.ini", "perl-dist-ini", []string{"[Prereqs]", "Moo = 2.005005"}},
		{"dist-ini-phases", "dist.ini", "perl-dist-ini", []string{"-phase = configure", "[Prereqs / RuntimeRequires]", "[Prereqs / TestRecommends]", "-phase = develop"}},
		{"dist-ini-relationships", "dist.ini", "perl-dist-ini", []string{"-relationship = recommends", "-relationship = suggests", "-relationship = conflicts"}},
		{"dist-ini-from-cpanfile", "dist.ini", "perl-dist-ini", []string{"[Prereqs::FromCPANfile]", "[MakeMaker]"}},
		{"dist-ini-prereq-sources", "dist.ini", "perl-dist-ini", []string{"[PrereqsFile]", "[AutoPrereqs]", "[RecommendedPrereqs]"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "perl", fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range fixture.required {
				if !strings.Contains(string(contents), expected) {
					t.Errorf("fixture is missing %q", expected)
				}
			}
			if fixture.filename == "META.json" && !json.Valid(contents) {
				t.Fatal("META.json fixture is invalid JSON")
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			wantSources := 1
			if fixture.name == "dist-ini-from-cpanfile" {
				wantSources = 2
			}
			found := false
			for _, source := range result.Sources {
				if source.Detector == fixture.detector && source.Path == fixture.filename && source.Analysis == (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
					found = true
				}
			}
			if len(result.Sources) != wantSources || !found {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestRLockfileFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, f := range []struct {
		name, path string
		detector   DetectorID
		required   string
	}{
		{"renv-basic", "renv.lock", "r-renv-lock", "\"Repository\":\"CRAN\""}, {"renv-bioconductor", "renv.lock", "r-renv-lock", "\"Bioconductor\""}, {"renv-github", "renv.lock", "r-renv-lock", "\"RemoteType\":\"github\""}, {"renv-local", "renv.lock", "r-renv-lock", "skeleton_1.0.1.tar.gz"}, {"renv-multiple", "renv.lock", "r-renv-lock", "\"WORK\""}, {"renv-vcs-remotes", "renv.lock", "r-renv-lock", "\"RemoteType\":\"gitlab\""},
		{"packrat-basic", "packrat/packrat.lock", "r-packrat-lock", "Source: CRAN"}, {"packrat-multiple", "packrat/packrat.lock", "r-packrat-lock", "  tibble:"}, {"packrat-github", "packrat/packrat.lock", "r-packrat-lock", "RemoteHost: api.github.com"}, {"packrat-local", "packrat/packrat.lock", "r-packrat-lock", "Source: Local"}, {"packrat-empty", "packrat/packrat.lock", "r-packrat-lock", "Packages:"}, {"packrat-bioconductor", "packrat/packrat.lock", "r-packrat-lock", "Source: Bioconductor"},
	} {
		t.Run(f.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "r", f.name)
			contents, err := os.ReadFile(filepath.Join(root, f.path))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(contents), f.required) {
				t.Errorf("missing %q", f.required)
			}
			if f.detector == "r-renv-lock" && !json.Valid(contents) {
				t.Fatal("invalid renv JSON")
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != f.detector || result.Sources[0].Path != f.path || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestLuaAndZigFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, f := range []struct {
		language, name, file string
		detector             DetectorID
		want                 string
	}{
		{"lua", "rockspec-runtime", "demo-1.0-1.rockspec", "lua-rockspec", "dkjson == 2.8"}, {"lua", "rockspec-build-test", "demo-1.0-1.rockspec", "lua-rockspec", "test_dependencies"}, {"lua", "rockspec-platform", "demo-1.0-1.rockspec", "lua-rockspec", "platforms ="}, {"lua", "rockspec-external", "demo-1.0-1.rockspec", "lua-rockspec", "external_dependencies"}, {"lua", "rockspec-scm", "demo-scm-1.rockspec", "lua-rockspec", "git+https"},
		{"zig", "build-zon-basic", "build.zig.zon", "zig-build-zon", ".url"}, {"zig", "build-zon-multiple", "build.zig.zon", "zig-build-zon", ".bar"}, {"zig", "build-zon-lazy", "build.zig.zon", "zig-build-zon", ".lazy = true"}, {"zig", "build-zon-paths", "build.zig.zon", "zig-build-zon", ".paths"}, {"zig", "build-zon-minimal", "build.zig.zon", "zig-build-zon", ".dependencies = .{}"}, {"zig", "build-zon-local", "build.zig.zon", "zig-build-zon", ".path = \"deps/local\""},
	} {
		t.Run(f.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", f.language, f.name)
			b, err := os.ReadFile(filepath.Join(root, f.file))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), f.want) {
				t.Errorf("missing %q", f.want)
			}
			r, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			wantSources := 1
			if f.name == "build-zon-local" {
				wantSources = 2
			}
			found := false
			for _, source := range r.Sources {
				if source.Detector == f.detector && source.Path == f.file && source.Analysis == (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
					found = true
				}
			}
			if len(r.Sources) != wantSources || !found {
				t.Fatalf("unexpected source: %+v", r.Sources)
			}
		})
	}
}

func TestNimbleAndOpamFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		language, name, filename string
		detector                 DetectorID
		required                 []string
	}{
		{"nim", "nimble-basic", "demo.nimble", "nim-nimble", []string{"nim >= 2.0.0", "jester >= 0.6.0"}},
		{"nim", "nimble-constraints", "demo.nimble", "nim-nimble", []string{"< 5.0.0", "== 0.5.0", "<= 2.0.0"}},
		{"nim", "nimble-compatible", "demo.nimble", "nim-nimble", []string{"~= 0.2.0", "^= 0.6.0", "> 0.4.0"}},
		{"nim", "nimble-separate-requires", "demo.nimble", "nim-nimble", []string{"requires \"cligen", "requires \"yaml", "requires \"unittest2\""}},
		{"nim", "nimble-conditional", "demo.nimble", "nim-nimble", []string{"when defined(windows):", "elif defined(macosx):", "posixutils >= 0.3.0"}},
		{"nim", "nimble-nim-version", "demo.nimble", "nim-nimble", []string{"nim >= 2.0.0 & < 2.3.0", "malebolgia >= 0.7.0"}},
		{"nim", "nimble-vcs", "demo.nimble", "nim-nimble", []string{"https://github.com/example/direct-git#5a54b5e", "https://bitbucket.org/example/direct-hg >= 2.0.0", "unreleased-tool#head"}},
		{"nim", "nimble-local-features", "demo.nimble", "nim-nimble", []string{"file:///opt/demo/shared-library", "async-library[chronos, tracing]", "feature \"sqlite\":", "dev:"}},
		{"ocaml", "opam-basic", "demo.opam", "ocaml-opam", []string{"opam-version: \"2.0\"", "\"dune\"", "\"cmdliner\""}},
		{"ocaml", "opam-constraints", "demo.opam", "ocaml-opam", []string{"< \"5.3\"", "= \"2.2.2\"", "| = \"0.8.10\""}},
		{"ocaml", "opam-test-build", "demo.opam", "ocaml-opam", []string{"opam-version: \"2.2\"", "& build", "{post}", "with-test", "with-doc", "with-dev-setup"}},
		{"ocaml", "opam-optional", "demo.opam", "ocaml-opam", []string{"depopts:", "\"lwt\"", "conflicts:", "\"legacy-http\""}},
		{"ocaml", "opam-pinned", "demo.opam", "ocaml-opam", []string{"\"experimental-parser\" {>= \"0.3.0\"}", "pin-depends:", "git+https://"}},
		{"ocaml", "opam-depexts", "demo.opam", "ocaml-opam", []string{"depexts:", "os-family = \"debian\"", "os = \"macos\""}},
		{"ocaml", "opam-alternatives", "demo.opam", "ocaml-opam", []string{"\"backend-a\"", "| \"backend-b\"", "os = \"linux\""}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", fixture.language, fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range fixture.required {
				if !strings.Contains(string(contents), required) {
					t.Fatalf("fixture is missing %q", required)
				}
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.filename || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestOCamlLockedOpamAndDuneFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, filename string
		detector       DetectorID
		required       []string
	}{
		{"opam-locked-direct", "demo.opam.locked", "ocaml-opam-locked", []string{"--direct-only", "\"ocaml\" {= \"5.2.0\"}", "\"dune\" {= \"3.17.2\"}", "\"fmt\" {= \"0.9.0\"}"}},
		{"opam-locked-transitive", "demo.opam.locked", "ocaml-opam-locked", []string{"\"ocaml\" {= \"5.1.1\"}", "\"dune\" {= \"3.15.3\"}", "\"cmdliner\" {= \"1.3.0\"}", "\"fmt\" {= \"0.9.0\"}", "\"uutf\" {= \"1.0.3\"}", "\"astring\" {= \"0.8.5\"}"}},
		{"opam-locked-filters", "demo.opam.locked", "ocaml-opam-locked", []string{"with-test", "with-doc"}},
		{"opam-locked-optional", "demo.opam.locked", "ocaml-opam-locked", []string{"\"lwt\" {= \"5.8.0\"}", "conflicts:", "\"not-installed-optional\" {= \"1.0.0\"}"}},
		{"opam-locked-pinned", "demo.opam.locked", "ocaml-opam-locked", []string{"\"local-parser\" {= \"0.4.0\"}", "\"local-parser.0.4.0\"", "pin-depends:"}},
		{"opam-locked-keep-local", "demo.opam.locked", "ocaml-opam-locked", []string{"\"in-tree-helper\" {= \"0.1.0\"}", "\"in-tree-helper.0.1.0\"", "file:///"}},
		{"opam-locked-dev", "demo.opam.locked", "ocaml-opam-locked", []string{"& dev", "with-dev-setup"}},
		{"dune-project-basic", "dune-project", "ocaml-dune-project", []string{"(generate_opam_files)", "(depends ocaml fmt cmdliner)"}},
		{"dune-project-constraints", "dune-project", "ocaml-dune-project", []string{"(and (>= 0.9.0) (< 1.0.0))", "(<> 1.5.0)", "(> 2.0.0)", "(<= 4.0.0)"}},
		{"dune-project-filters", "dune-project", "ocaml-dune-project", []string{"(and :with-test (>= 1.8.0))", ":with-doc", ":with-dev-setup", ":post", ":build"}},
		{"dune-project-optional", "dune-project", "ocaml-dune-project", []string{"(depopts", "(conflicts"}},
		{"dune-project-alternatives", "dune-project", "ocaml-dune-project", []string{"(or (= 1.0.0) (>= 2.0.0))", ":os"}},
		{"dune-project-multiple", "dune-project", "ocaml-dune-project", []string{"(ocaml (= :version))", "(name cli-package)"}},
		{"dune-project-pins", "dune-project", "ocaml-dune-project", []string{"(depends local-format remote-parser)", "file:///opt/workspaces/local-format", "git+https://github.com/example/remote-parser.git"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "ocaml", fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range fixture.required {
				if !strings.Contains(string(contents), required) {
					t.Fatalf("fixture is missing %q", required)
				}
			}
			if fixture.detector == "ocaml-opam-locked" {
				assertLockedOpamFixture(t, fixture.name, string(contents))
			}
			if fixture.detector == "ocaml-dune-project" {
				assertDuneProjectFixture(t, fixture.name, string(contents))
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.filename || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestEsyAndFpmFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		language, name, filename string
		detector                 DetectorID
		required                 []string
	}{
		{"ocaml", "esy-basic", "esy.json", "ocaml-esy", []string{"\"build\": [\"dune build\"]", "\"@opam/dune\"", "\"@opam/fmt\""}},
		{"ocaml", "esy-dev", "esy.json", "ocaml-esy", []string{"\"devDependencies\"", "\"@opam/merlin\"", "\"@opam/lwt\": \"^5.7.0\""}},
		{"ocaml", "esy-resolutions", "esy.json", "ocaml-esy", []string{"\"resolutions\"", "\"@opam/uri\": \"4.4.0\"", "ocsigen/lwt:lwt.opam#0123456789abcdef"}},
		{"ocaml", "esy-vcs-file", "esy.json", "ocaml-esy", []string{"file:../local-helper", "link:../linked-helper", "git+https://", "#0123456789abcdef"}},
		{"ocaml", "esy-opam-repository", "esy.json", "ocaml-esy", []string{"\"opamRepositories\"", "private-opam-repository", "@opam/ppxlib"}},
		{"ocaml", "esy-npm-ranges", "esy.json", "ocaml-esy", []string{"\"~3.13.0\"", "\">=20220210\"", "\"*\""}},
		{"fortran", "fpm-git-refs", "fpm.toml", "fortran-fpm", []string{"branch = \"main\"", "tag = \"v0.4.2\"", "rev = \"2f5eaba"}},
		{"fortran", "fpm-git-table", "fpm.toml", "fortran-fpm", []string{"[dependencies.ford]", "rev = \"0123456789abcdef"}},
		{"fortran", "fpm-registry", "fpm.toml", "fortran-fpm", []string{"example-package.namespace", "example-package.v", "latest-package.namespace", "stdlib = \"*\""}},
		{"fortran", "fpm-local", "fpm.toml", "fortran-fpm", []string{"path = \"../utilities\"", "path = \"vendor/subproject\""}},
		{"fortran", "fpm-dev-dependencies", "fpm.toml", "fortran-fpm", []string{"[dev-dependencies]", "test-drive"}},
		{"fortran", "fpm-target-dependencies", "fpm.toml", "fortran-fpm", []string{"[executable.dependencies]", "[test.dependencies]", "[example.dependencies]", "cli-support", "test-support", "example-support"}},
		{"fortran", "fpm-preprocess-dependency", "fpm.toml", "fortran-fpm", []string{"preprocess.cpp.macros", "REAL32"}},
		{"fortran", "fpm-features", "fpm.toml", "fortran-fpm", []string{"features = [\"mpi\", \"optimized\"]", "profile = \"release\"", "testing.dev-dependencies", "windows-support.windows.dependencies"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", fixture.language, fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range fixture.required {
				if !strings.Contains(string(contents), required) {
					t.Fatalf("fixture is missing %q", required)
				}
			}
			if fixture.detector == "ocaml-esy" {
				assertEsyFixture(t, fixture.name, contents)
			}
			if fixture.detector == "fortran-fpm" {
				var manifest map[string]any
				if _, err := toml.Decode(string(contents), &manifest); err != nil {
					t.Fatalf("invalid fpm TOML: %v", err)
				}
				assertFPMFixture(t, fixture.name, manifest)
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.filename || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestVAndBrewfileFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		language, name, filename string
		detector                 DetectorID
		required                 []string
	}{
		{"vlang", "project-basic", "v.mod", "vlang", []string{"'nedpals.args'", "'vlang.ui'", "'benjamine.terminal'"}},
		{"vlang", "project-metadata", "v.mod", "vlang", []string{"base_url: 'src'", "'vlang.markdown'", "'vlang.toml'"}},
		{"vlang", "project-subdirs", "v.mod", "vlang", []string{"subdirs: ['internal', 'generated']", "'mygithubname.httpclient'"}},
		{"vlang", "project-multiline", "v.mod", "vlang", []string{"dependencies: [", "'githubowner.collection'", "'thirdowner.validation'"}},
		{"vlang", "project-single", "v.mod", "vlang", []string{"dependencies: ['vlang.json2']"}},
		{"vlang", "project-comments", "v.mod", "vlang", []string{"// VPM package names", "'exampleowner.collections'"}},
		{"vlang", "project-base-subdirs", "v.mod", "vlang", []string{"base_url: 'src'", "subdirs: ['internal', 'generated']", "'exampleowner.source-root-aware'"}},
		{"homebrew", "brewfile-formula-options", "Brewfile", "homebrew-brewfile", []string{"restart_service: true", "link: :overwrite", "conflicts_with:", "postinstall:", "version_file:"}},
		{"homebrew", "brewfile-taps", "Brewfile", "homebrew-brewfile", []string{"tap \"user/tap-repository\"", "trusted: true", "bitbucket.org", "granular/repository", "cask \"trusted/repository/private-cask\""}},
		{"homebrew", "brewfile-casks", "Brewfile", "homebrew-brewfile", []string{"cask_args", "greedy: true", "postinstall:"}},
		{"homebrew", "brewfile-app-stores", "Brewfile", "homebrew-brewfile", []string{"mas \"Refined GitHub\"", "winget \"Steam\"", "vscode \"editorconfig.editorconfig\""}},
		{"homebrew", "brewfile-language-tools", "Brewfile", "homebrew-brewfile", []string{"go \"github.com/charmbracelet/crush\"", "cargo \"ripgrep\"", "npm \"typescript\"", "source: \"git+https://"}},
		{"homebrew", "brewfile-platform-tools", "Brewfile", "homebrew-brewfile", []string{"krew \"ctx\"", "flatpak \"org.godotengine.Godot\"", "flathub-beta"}},
		{"homebrew", "brewfile-conditionals", "Brewfile", "homebrew-brewfile", []string{"if OS.mac?", "if OS.linux?", "unless system"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", fixture.language, fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range fixture.required {
				if !strings.Contains(string(contents), required) {
					t.Fatalf("fixture is missing %q", required)
				}
			}
			if fixture.detector == "vlang" {
				assertVModuleFixture(t, string(contents))
			}
			if fixture.detector == "homebrew-brewfile" {
				assertBrewfileFixture(t, string(contents))
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.filename || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestTerraformAndUnityLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		language, name, filename string
		detector                 DetectorID
		required                 []string
	}{
		{"terraform", "lock-basic", ".terraform.lock.hcl", "terraform-lock", []string{"provider \"registry.terraform.io/hashicorp/null\"", "version = \"3.1.0\"", "h1:", "zh:"}},
		{"terraform", "lock-constraints", ".terraform.lock.hcl", "terraform-lock", []string{"constraints = \">= 3.20.0\"", "h1:", "zh:"}},
		{"terraform", "lock-multiple", ".terraform.lock.hcl", "terraform-lock", []string{"hashicorp/aws", "hashicorp/null", "version     = \"6.2.0\""}},
		{"terraform", "lock-custom-host", ".terraform.lock.hcl", "terraform-lock", []string{"hashicorp/aws", "constraints = \">= 3.20.0\""}},
		{"terraform", "lock-prerelease", ".terraform.lock.hcl", "terraform-lock", []string{"hashicorp/null", "constraints = \"~> 3.1\""}},
		{"terraform", "lock-platform-hashes", ".terraform.lock.hcl", "terraform-lock", []string{"h1:xhb", "zh:02a", "zh:53e", "zh:5f9"}},
		{"unity", "packages-lock-registry", "Packages/packages-lock.json", "unity-packages-lock", []string{"com.unity.textmeshpro", "\"source\": \"registry\""}},
		{"unity", "packages-lock-transitive", "Packages/packages-lock.json", "unity-packages-lock", []string{"\"depth\": 1", "com.unity.modules.director"}},
		{"unity", "packages-lock-git", "Packages/packages-lock.json", "unity-packages-lock", []string{"\"source\": \"git\"", "\"hash\""}},
		{"unity", "packages-lock-local", "Packages/packages-lock.json", "unity-packages-lock", []string{"\"source\": \"local\"", "file:../Packages"}},
		{"unity", "packages-lock-local-tarball", "Packages/packages-lock.json", "unity-packages-lock", []string{"\"source\": \"local\"", ".tgz"}},
		{"unity", "packages-lock-built-in", "Packages/packages-lock.json", "unity-packages-lock", []string{"\"source\": \"builtin\"", "com.unity.modules.physics"}},
		{"unity", "packages-lock-mixed", "Packages/packages-lock.json", "unity-packages-lock", []string{"?path=/Package", "\"depth\": 1", "com.unity.cinemachine"}},
		{"unity", "packages-lock-scoped-registry", "Packages/packages-lock.json", "unity-packages-lock", []string{"\"source\": \"registry\"", "packages.example.com"}},
		{"unity", "packages-lock-git-protocols", "Packages/packages-lock.json", "unity-packages-lock", []string{"git+https://", "git@git.example.com", "\"hash\""}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", fixture.language, fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range fixture.required {
				if !strings.Contains(string(contents), required) {
					t.Fatalf("fixture is missing %q", required)
				}
			}
			if fixture.detector == "terraform-lock" {
				assertTerraformLockFixture(t, string(contents))
			}
			if fixture.detector == "unity-packages-lock" {
				assertUnityLockFixture(t, contents)
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.filename || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestGitHubActionsFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		root, path string
		detector   DetectorID
		contains   string
	}{
		{"github-actions/action-composite-pinned", "action.yml", "github-actions-action", "@8f4b7f84864484a7bf31766abe9204da3cbe65b3"},
		{"github-actions/action-composite-refs", "action.yml", "github-actions-action", "actions/cache@main"},
		{"github-actions/action-composite-local", "action.yml", "github-actions-action", "./.github/actions/setup"},
		{"github-actions/action-composite-docker", "action.yml", "github-actions-action", "docker://gcr.io"},
		{"github-actions/action-node", "action.yml", "github-actions-action", "using: node24"},
		{"github-actions/action-docker", "action.yml", "github-actions-action", "using: docker"},
		{"github-actions/action-yaml", "action.yaml", "github-actions-action", "using: 'composite'"},
		{"github-actions/action-yaml-composite", "action.yaml", "github-actions-action", "actions/aws/ec2@main"},
		{"github/actions-workflow-pinned", ".github/workflows/ci.yml", "github-actions-workflow", "actions/checkout@"},
		{"github/actions-workflow-local", ".github/workflows/ci.yml", "github-actions-workflow", "uses: ./.github/actions/build"},
		{"github/actions-workflow-docker", ".github/workflows/ci.yml", "github-actions-workflow", "docker://ghcr.io"},
		{"github/actions-workflow-reusable", ".github/workflows/ci.yml", "github-actions-workflow", "another-repo/.github/workflows/release.yml@"},
		{"github/actions-workflow-container", ".github/workflows/ci.yml", "github-actions-workflow", "image: redis:7"},
		{"github/actions-workflow-private", ".github/workflows/ci.yml", "github-actions-workflow", "octo-org/private-action@v1"},
		{"github/actions-workflow-yaml", ".github/workflows/ci.yaml", "github-actions-workflow", "actions/checkout@v6"},
		{"github/actions-workflow-container-scalar", ".github/workflows/ci.yml", "github-actions-workflow", "container: node:22"},
	} {
		t.Run(fixture.root, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", fixture.root)
			contents, err := os.ReadFile(filepath.Join(root, fixture.path))
			if err != nil || !strings.Contains(string(contents), fixture.contains) {
				t.Fatalf("invalid fixture: %v", err)
			}
			var document any
			if err := yaml.Unmarshal(contents, &document); err != nil {
				t.Fatalf("invalid YAML: %v", err)
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil || len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.path || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v err=%v", result.Sources, err)
			}
		})
	}
}

func assertTerraformLockFixture(t *testing.T, contents string) {
	t.Helper()
	if !strings.Contains(contents, "provider ") || !strings.Contains(contents, "version") {
		t.Fatal("invalid Terraform provider lock fixture")
	}
	for _, block := range strings.Split(contents, "provider ")[1:] {
		if !strings.HasPrefix(block, "\"") || !strings.Contains(block, "registry.terraform.io/") && !strings.Contains(block, "terraform.example.com/") {
			t.Fatalf("invalid provider address block %q", block)
		}
	}
}

func assertUnityLockFixture(t *testing.T, contents []byte) {
	t.Helper()
	var lock struct {
		Dependencies map[string]struct {
			Version      string            `json:"version"`
			Depth        int               `json:"depth"`
			Source       string            `json:"source"`
			Dependencies map[string]string `json:"dependencies"`
			URL          string            `json:"url"`
			Hash         string            `json:"hash"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(contents, &lock); err != nil {
		t.Fatalf("invalid Unity package lock JSON: %v", err)
	}
	if len(lock.Dependencies) == 0 {
		t.Fatal("Unity package lock has no dependencies")
	}
	for name, dependency := range lock.Dependencies {
		if name == "" || dependency.Version == "" || dependency.Dependencies == nil || dependency.Depth < 0 {
			t.Fatalf("invalid Unity package entry %q: %#v", name, dependency)
		}
		switch dependency.Source {
		case "registry":
			if dependency.URL == "" || dependency.Hash != "" {
				t.Fatalf("invalid Unity registry entry %q: %#v", name, dependency)
			}
		case "git":
			if !(strings.Contains(dependency.Version, "://") || strings.HasPrefix(dependency.Version, "git@")) || len(dependency.Hash) != 40 || !isLowerHex(dependency.Hash) {
				t.Fatalf("invalid Unity Git entry %q: %#v", name, dependency)
			}
		case "local":
			if !strings.HasPrefix(dependency.Version, "file:") || dependency.Hash != "" {
				t.Fatalf("invalid Unity local entry %q: %#v", name, dependency)
			}
		case "builtin":
			if dependency.Hash != "" || dependency.URL != "" {
				t.Fatalf("invalid Unity built-in entry %q: %#v", name, dependency)
			}
		default:
			t.Fatalf("unknown Unity package source %q", dependency.Source)
		}
	}
}

func assertVModuleFixture(t *testing.T, contents string) {
	t.Helper()
	if !strings.Contains(contents, "Module {") {
		t.Fatal("invalid V module definition")
	}
	start := strings.Index(contents, "dependencies:")
	if start < 0 {
		t.Fatal("V module has no dependency list")
	}
	list := contents[start:]
	open, close := strings.Index(list, "["), strings.Index(list, "]")
	if open < 0 || close < open {
		t.Fatal("V module dependencies are not an array")
	}
	dependencies := strings.Split(list[open+1:close], ",")
	count := 0
	for _, dependency := range dependencies {
		dependency = strings.TrimSpace(dependency)
		if dependency == "" {
			continue
		}
		if !strings.HasPrefix(dependency, "'") || !strings.HasSuffix(dependency, "'") || strings.Count(strings.Trim(dependency, "'"), ".") != 1 {
			t.Fatalf("invalid VPM package ID %q", dependency)
		}
		count++
	}
	if count == 0 {
		t.Fatal("V module has no declared VPM dependencies")
	}
}

func assertBrewfileFixture(t *testing.T, contents string) {
	t.Helper()
	validDirectives := []string{"tap ", "brew ", "cask ", "mas ", "winget ", "vscode ", "go ", "cargo ", "npm ", "uv ", "krew ", "flatpak ", "cask_args "}
	count := 0
	for _, line := range strings.Split(contents, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "formula:") || strings.HasPrefix(line, "casks:") || strings.HasPrefix(line, "commands:") {
			continue
		}
		for _, directive := range validDirectives {
			if strings.HasPrefix(line, directive) {
				count++
				break
			}
		}
	}
	if count == 0 {
		t.Fatal("Brewfile has no recognized dependency directives")
	}
}

func assertEsyFixture(t *testing.T, name string, contents []byte) {
	t.Helper()
	var manifest map[string]any
	if err := json.Unmarshal(contents, &manifest); err != nil {
		t.Fatalf("invalid esy JSON: %v", err)
	}
	dependencies, ok := manifest["dependencies"].(map[string]any)
	if !ok || len(dependencies) == 0 {
		t.Fatalf("Esy fixture %q has no dependencies: %#v", name, manifest)
	}
	if name == "esy-resolutions" {
		resolutions, ok := manifest["resolutions"].(map[string]any)
		if !ok || len(resolutions) == 0 {
			t.Fatal("missing Esy resolutions")
		}
		for dependency := range resolutions {
			if _, ok := dependencies[dependency]; !ok {
				t.Fatalf("resolution %q is not declared as a dependency", dependency)
			}
		}
	}
	if name == "esy-basic" {
		esy, ok := manifest["esy"].(map[string]any)
		build, ok := esy["build"].([]any)
		if !ok || len(build) != 1 || build[0] != "dune build" {
			t.Fatalf("invalid Esy build configuration: %#v", esy)
		}
	}
	if name == "esy-dev" {
		if devDependencies, ok := manifest["devDependencies"].(map[string]any); !ok || devDependencies["@opam/merlin"] == nil {
			t.Fatalf("missing Esy development dependencies: %#v", manifest)
		}
	}
	if name == "esy-vcs-file" {
		for dependency, prefix := range map[string]string{"local-helper": "file:", "linked-helper": "link:", "git-helper": "git+", "github-helper": "example/"} {
			value, ok := dependencies[dependency].(string)
			if !ok || !strings.HasPrefix(value, prefix) {
				t.Fatalf("invalid Esy source dependency %q: %#v", dependency, dependencies[dependency])
			}
		}
	}
	if name == "esy-opam-repository" {
		esy, ok := manifest["esy"].(map[string]any)
		repositories, ok := esy["opamRepositories"].([]any)
		if !ok || len(repositories) == 0 {
			t.Fatal("missing Esy opam repositories")
		}
		for _, repository := range repositories {
			entry, ok := repository.(map[string]any)
			if !ok || entry["type"] != "remote" || entry["location"] == "" {
				t.Fatalf("invalid Esy opam repository: %#v", repository)
			}
		}
	}
}

func assertFPMFixture(t *testing.T, name string, manifest map[string]any) {
	t.Helper()
	if manifest["package"] != nil {
		t.Fatalf("fpm fixture %q has an unsupported package table", name)
	}
	nameValue, hasName := manifest["name"].(string)
	versionValue, hasVersion := manifest["version"].(string)
	if !hasName || nameValue == "" || !hasVersion || versionValue == "" {
		t.Fatalf("fpm fixture %q needs root name and version: %#v", name, manifest)
	}
	if name == "fpm-features" {
		dependencies, ok := manifest["dependencies"].(map[string]any)
		if !ok || dependencies["mpi-library"] == nil || dependencies["release-library"] == nil || manifest["features"] == nil {
			t.Fatalf("missing feature dependencies: %#v", manifest)
		}
	}
	if name == "fpm-registry" {
		dependencies, ok := manifest["dependencies"].(map[string]any)
		if !ok || dependencies["stdlib"] != "*" || dependencies["example-package"] == nil {
			t.Fatalf("invalid fpm registry dependencies: %#v", manifest)
		}
	}
	if name == "fpm-target-dependencies" {
		for _, targetKind := range []string{"executable", "test", "example"} {
			targets, ok := manifest[targetKind].([]map[string]any)
			if !ok || len(targets) != 1 || targets[0]["dependencies"] == nil {
				t.Fatalf("missing fpm %s dependency table: %#v", targetKind, manifest[targetKind])
			}
		}
	}
}

func assertLockedOpamFixture(t *testing.T, name, contents string) {
	t.Helper()
	if !strings.HasPrefix(contents, "opam-version: \"2.0\"\n") || !strings.Contains(contents, "depends: [") {
		t.Fatalf("invalid locked opam fixture %q", name)
	}
	inDepends := false
	dependencyCount := 0
	for _, line := range strings.Split(contents, "\n") {
		line = strings.TrimSpace(line)
		if line == "depends: [" {
			inDepends = true
			continue
		}
		if inDepends && line == "]" {
			inDepends = false
			continue
		}
		if inDepends && strings.HasPrefix(line, "\"") {
			dependencyCount++
			if !strings.Contains(line, "{= ") {
				t.Fatalf("unlocked dependency in %q: %s", name, line)
			}
		}
	}
	if dependencyCount == 0 {
		t.Fatalf("locked opam fixture %q has no dependencies", name)
	}
	if name == "opam-locked-optional" && strings.Contains(contents, "depopts:") {
		t.Fatal("locked optional fixture must show opam lock's depopts transformation")
	}
}

func assertDuneProjectFixture(t *testing.T, name, contents string) {
	t.Helper()
	if !strings.HasPrefix(contents, "(lang dune 3.17)\n") {
		t.Fatalf("invalid Dune language declaration in %q", name)
	}
	packageCount := strings.Count(contents, "\n(package\n (name")
	if packageCount == 0 || packageCount != strings.Count(contents, "(allow_empty)") {
		t.Fatalf("Dune package stanzas in %q need allow_empty: packages=%d allow_empty=%d", name, packageCount, strings.Count(contents, "(allow_empty)"))
	}
}

func assertPerlBuildDefinitionShape(t *testing.T, filename, contents string) {
	t.Helper()
	if !strings.HasPrefix(contents, "use ") || !strings.HasSuffix(strings.TrimSpace(contents), ";") {
		t.Fatalf("invalid Perl build fixture shape for %s", filename)
	}
	if filename == "Makefile.PL" && !strings.Contains(contents, "WriteMakefile(") {
		t.Fatal("Makefile.PL has no WriteMakefile call")
	}
	if filename == "Build.PL" && !strings.Contains(contents, "create_build_script") {
		t.Fatal("Build.PL has no build-script call")
	}
}

func assertCPANfileSnapshotStructure(t *testing.T, contents string) {
	t.Helper()
	if !strings.HasPrefix(contents, "# carton snapshot format: version 1.0\n") || !strings.Contains(contents, "\nDISTRIBUTIONS\n") {
		t.Fatal("invalid Carton snapshot header")
	}
	var distribution, section string
	pathname, provides, requirements := false, false, false
	check := func() {
		if distribution != "" && (!pathname || !provides || !requirements) {
			t.Fatalf("invalid Carton distribution entry: %q", distribution)
		}
	}
	for _, line := range strings.Split(contents, "\n") {
		if line == "" || strings.HasPrefix(line, "# carton snapshot") || strings.HasPrefix(line, "CARTON:") || line == "DISTRIBUTIONS" {
			continue
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") {
			check()
			distribution = strings.TrimSpace(line)
			if distribution == "" {
				t.Fatal("empty Carton distribution name")
			}
			pathname, provides, requirements, section = false, false, false, ""
			continue
		}
		if strings.HasPrefix(line, "    pathname: ") {
			value := strings.TrimPrefix(line, "    pathname: ")
			if value == "" || !strings.HasSuffix(value, ".tar.gz") {
				t.Fatalf("invalid Carton pathname: %q", line)
			}
			pathname, section = true, ""
			continue
		}
		if line == "    provides:" {
			provides, section = true, "provides"
			continue
		}
		if line == "    requirements:" {
			requirements, section = true, "requirements"
			continue
		}
		if strings.HasPrefix(line, "      ") && section != "" && len(strings.Fields(line)) >= 2 {
			continue
		}
		t.Fatalf("invalid Carton snapshot line: %q", line)
	}
	check()
}

func assertMixLockStructure(t *testing.T, contents string) {
	t.Helper()
	trimmed := strings.TrimSpace(contents)
	if !strings.HasPrefix(trimmed, "%{") || !strings.HasSuffix(trimmed, "}") {
		t.Fatalf("Mix lock is not a map expression: %q", trimmed)
	}
	if trimmed == "%{}" {
		return
	}
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, ","))
		if strings.HasPrefix(line, "\"") && !strings.Contains(line, "\" => {:") {
			t.Fatalf("Mix lock entry is not a lock tuple: %q", line)
		}
	}
}

func assertConanLockStructure(t *testing.T, contents []byte) {
	t.Helper()
	var lock map[string]json.RawMessage
	if err := json.Unmarshal(contents, &lock); err != nil {
		t.Fatalf("lock fixture is not a JSON object: %v", err)
	}
	var version string
	if err := json.Unmarshal(lock["version"], &version); err != nil || version == "" {
		t.Fatalf("lock fixture has no string version: %v", err)
	}
	if version == "0.4" {
		var graph struct {
			Nodes map[string]struct {
				Ref string `json:"ref"`
			} `json:"nodes"`
		}
		if err := json.Unmarshal(lock["graph_lock"], &graph); err != nil || len(graph.Nodes) == 0 {
			t.Fatalf("legacy lock has no graph nodes: %v", err)
		}
		for _, node := range graph.Nodes {
			if node.Ref != "" {
				return
			}
		}
		t.Fatal("legacy lock has no referenced node")
	}
	for _, key := range []string{"requires", "build_requires", "python_requires"} {
		var refs []string
		if err := json.Unmarshal(lock[key], &refs); err != nil {
			t.Fatalf("%s is not an array of references: %v", key, err)
		}
		for _, ref := range refs {
			if !strings.Contains(ref, "/") {
				t.Fatalf("invalid Conan reference %q", ref)
			}
		}
	}
	if raw, ok := lock["config_requires"]; ok {
		var refs []string
		if err := json.Unmarshal(raw, &refs); err != nil || len(refs) == 0 {
			t.Fatalf("config_requires is not a non-empty reference array: %v", err)
		}
	}
}

func TestBunAndDenoLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name     string
		detector DetectorID
		file     string
		version  string
		mustHave []string
	}{
		{name: "bun-lock-registry", detector: "js-bun-lock", file: "bun.lock", version: "1", mustHave: []string{"workspaces", "packages", "chalk@5.3.0"}},
		{name: "bun-lock-transitive-peer", detector: "js-bun-lock", file: "bun.lock", version: "1", mustHave: []string{"peerDependencies", "optionalPeers", "utility@1.2.0"}},
		{name: "bun-lock-optional-platform", detector: "js-bun-lock", file: "bun.lock", version: "1", mustHave: []string{"optionalDependencies", "native-linux", "darwin"}},
		{name: "bun-lock-git", detector: "js-bun-lock", file: "bun.lock", version: "1", mustHave: []string{"github:example/fixture-git#4d2c8a1", "git+https://github.com"}},
		{name: "bun-lock-tarball-local", detector: "js-bun-lock", file: "bun.lock", version: "1", mustHave: []string{"https://packages.example.test", "file:../local-package"}},
		{name: "bun-lock-workspace", detector: "js-bun-lock", file: "bun.lock", version: "1", mustHave: []string{"workspace:*", "workspace:^", "workspace:~", "workspace:1.0.2", "!packages/**/test/**"}},
		{name: "bun-lock-catalog-jsonc", detector: "js-bun-lock", file: "bun.lock", version: "2", mustHave: []string{"catalog:", "catalog:testing", "\"lockfileVersion\": 2,"}},
		{name: "bun-lock-git-ssh", detector: "js-bun-lock", file: "bun.lock", version: "1", mustHave: []string{"git+ssh://git@github.com", "git@github.com:example/scp-package.git#v1.2.3"}},
		{name: "bun-lock-npm-alias", detector: "js-bun-lock", file: "bun.lock", version: "1", mustHave: []string{"npm:@types/bun@^1.1.0", "bun-types@npm:@types/bun@1.1.16"}},
		{name: "deno-lock-remote-v1", detector: "deno-lock", file: "deno.lock", version: "1", mustHave: []string{"remote", "https://deno.land/std@0.145.0/fmt/colors.ts"}},
		{name: "deno-lock-remote-v2", detector: "deno-lock", file: "deno.lock", version: "2", mustHave: []string{"remote", "https://deno.land/std@0.177.0/path/mod.ts"}},
		{name: "deno-lock-npm-v3", detector: "deno-lock", file: "deno.lock", version: "3", mustHave: []string{"specifiers", "chalk@5.3.0", "ansi-styles@6.2.1"}},
		{name: "deno-lock-jsr-v4", detector: "deno-lock", file: "deno.lock", version: "4", mustHave: []string{"jsr", "@std/assert@1.0.6", "@std/internal@1.0.6"}},
		{name: "deno-lock-remote-npm-v4", detector: "deno-lock", file: "deno.lock", version: "4", mustHave: []string{"remote", "npm:lodash@^4.17.0", "lodash@4.17.21"}},
		{name: "deno-lock-workspace-v5", detector: "deno-lock", file: "deno.lock", version: "5", mustHave: []string{"workspace", "npm:chalk@^5.6.2", "jsr:@std/assert@^1.0.0"}},
		{name: "deno-lock-npm-peer-v5", detector: "deno-lock", file: "deno.lock", version: "5", mustHave: []string{"plugin@1.0.0_host@2.1.0", "optionalPeers", "host@2.1.0"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "javascript", fixture.name, fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if fixture.detector == "js-bun-lock" {
				// Bun's text lockfile is JSONC, so a strict JSON parser would reject
				// normally generated trailing commas. Check its required sections directly.
				for _, section := range []string{"\"lockfileVersion\": " + fixture.version, "\"workspaces\":", "\"packages\":"} {
					if !strings.Contains(string(contents), section) {
						t.Fatalf("Bun lockfile missing %q", section)
					}
				}
			} else if fixture.detector != "js-pnp" {
				var document map[string]json.RawMessage
				if err := json.Unmarshal(contents, &document); err != nil {
					t.Fatalf("parse Deno lockfile JSON: %v", err)
				}
				var version string
				if err := json.Unmarshal(document["version"], &version); err != nil || version != fixture.version {
					t.Fatalf("unexpected Deno lockfile version: %q (%v)", version, err)
				}
				assertDenoLockFixtureShape(t, version, document)
			}
			for _, fragment := range fixture.mustHave {
				if !strings.Contains(string(contents), fragment) {
					t.Fatalf("fixture missing %q", fragment)
				}
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.file {
				t.Fatalf("expected %s source for %s, got %+v", fixture.detector, fixture.file, result.Sources)
			}
			if result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", result.Sources[0].Analysis)
			}
		})
	}
}

func assertDenoLockFixtureShape(t *testing.T, version string, document map[string]json.RawMessage) {
	t.Helper()
	if raw, ok := document["remote"]; ok {
		var remote map[string]string
		if err := json.Unmarshal(raw, &remote); err != nil || len(remote) == 0 {
			t.Fatalf("invalid Deno remote entries: %v", err)
		}
		for specifier, hash := range remote {
			if !strings.HasPrefix(specifier, "https://") || len(hash) != 64 || !isLowerHex(hash) {
				t.Fatalf("invalid remote integrity entry %q: %q", specifier, hash)
			}
		}
	}

	switch version {
	case "1", "2":
		if _, ok := document["remote"]; !ok {
			t.Fatalf("Deno v%s fixture must include remote dependencies", version)
		}
	case "3":
		if _, hasTopLevelNPM := document["npm"]; hasTopLevelNPM {
			t.Fatal("Deno v3 npm resolution must be nested under packages")
		}
		var packages map[string]json.RawMessage
		if err := json.Unmarshal(document["packages"], &packages); err != nil {
			t.Fatalf("invalid Deno v3 packages object: %v", err)
		}
		for _, key := range []string{"specifiers", "npm"} {
			if len(packages[key]) == 0 {
				t.Fatalf("Deno v3 packages object missing %s", key)
			}
		}
		if !strings.Contains(string(packages["specifiers"]), `"npm:`) || !strings.Contains(string(packages["specifiers"]), `": "npm:`) {
			t.Fatal("Deno v3 npm specifier is not mapped to an npm-prefixed resolution")
		}
	case "4":
		if raw, ok := document["jsr"]; ok {
			if !strings.Contains(string(raw), `"dependencies": ["jsr:`) {
				t.Fatal("Deno v4 JSR dependencies must use jsr-prefixed specifiers")
			}
		}
	case "5":
		if raw, hasWorkspace := document["workspace"]; hasWorkspace {
			var workspace struct {
				Dependencies []string `json:"dependencies"`
			}
			if err := json.Unmarshal(raw, &workspace); err != nil || len(workspace.Dependencies) == 0 {
				t.Fatalf("invalid Deno v5 workspace dependencies: %v", err)
			}
		}
		if len(document["npm"]) == 0 {
			t.Fatal("Deno v5 fixture missing top-level npm resolutions")
		}
	}

	for _, field := range []string{"packages", "npm", "jsr"} {
		if raw, ok := document[field]; ok && strings.Contains(string(raw), `"integrity"`) && !strings.Contains(string(raw), `"integrity": "`) {
			t.Fatalf("Deno %s integrity records are not JSON strings", field)
		}
	}
}

func TestYarnPnPAndPnpmWorkspaceFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, file string
		detector   DetectorID
		mustHave   []string
	}{
		{"pnp-basic", ".pnp.cjs", "js-pnp", []string{"npm:4.17.21", "lodash-npm-4.17.21.zip"}},
		{"pnp-workspaces", ".pnp.cjs", "js-pnp", []string{"workspace:packages/web", "@fixture/web"}},
		{"pnp-alias", ".pnp.cjs", "js-pnp", []string{"fixture-types", "@types/node"}},
		{"pnp-peer-virtual", ".pnp.cjs", "js-pnp", []string{"virtual:peer#npm:1.0.0", "packagePeers", "[\"host\",null]"}},
		{"pnp-fallback", ".pnp.cjs", "js-pnp", []string{"enableTopLevelFallback", "fallbackPool", "fallbackExclusionList"}},
		{"pnp-link-portal", ".pnp.cjs", "js-pnp", []string{"portal:../linked", "link:../local", "discardFromLookup"}},
		{"pnp-protocols", ".pnp.cjs", "js-pnp", []string{"file:../folder", "patch:npm:", "git:git@github.com", "exec:./scripts"}},
		{"pnp-esm-loader", ".pnp.loader.mjs", "js-pnp", []string{"export {resolve, load}"}},
		{"pnpm-workspace-patterns", "pnpm-workspace.yaml", "js-pnpm-workspace", []string{"components/**", "!**/test/**"}},
		{"pnpm-workspace-catalogs", "pnpm-workspace.yaml", "js-pnpm-workspace", []string{"catalog:", "catalogs:", "react17"}},
		{"pnpm-workspace-package-config-map", "pnpm-workspace.yaml", "js-pnpm-workspace", []string{"packageConfigs:", "saveExact", "savePrefix"}},
		{"pnpm-workspace-package-config-rules", "pnpm-workspace.yaml", "js-pnpm-workspace", []string{"match:", "modulesDir", "node_modules"}},
		{"pnpm-workspace-root-only", "pnpm-workspace.yaml", "js-pnpm-workspace", []string{"catalog:", "typescript"}},
		{"pnpm-workspace-nested-excludes", "pnpm-workspace.yaml", "js-pnpm-workspace", []string{"!packages/legacy/**", "catalogs:", "vite"}},
		{"pnpm-workspace-yml", "pnpm-workspace.yml", "js-pnpm-workspace", []string{"packages:", "@fixture/shared"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "js", fixture.name, fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.mustHave {
				if !strings.Contains(string(contents), fragment) {
					t.Fatalf("fixture missing %q", fragment)
				}
			}
			if fixture.detector == "js-pnp" && fixture.file == ".pnp.cjs" {
				start, end := strings.Index(string(contents), "`"), strings.LastIndex(string(contents), "`")
				if start < 0 || end <= start {
					t.Fatal("PnP fixture has no serialized runtime state")
				}
				assertPnPRuntimeState(t, contents[start+1:end])
			} else if fixture.detector != "js-pnp" {
				var document map[string]any
				if err := yaml.Unmarshal(contents, &document); err != nil {
					t.Fatalf("invalid pnpm workspace YAML: %v", err)
				}
				assertPnpmWorkspaceFixtureShape(t, fixture.name, document)
			}
			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			analysis := SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}
			if fixture.detector == "gleam" {
				analysis.Presence = PresencePresent
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.file || result.Sources[0].Analysis != analysis {
				t.Fatalf("unexpected selector-only source: %+v", result.Sources)
			}
		})
	}
}

func assertPnPRuntimeState(t *testing.T, contents []byte) {
	t.Helper()
	var state struct {
		Roots    []any             `json:"dependencyTreeRoots"`
		Registry []json.RawMessage `json:"packageRegistryData"`
	}
	if err := json.Unmarshal(contents, &state); err != nil || len(state.Roots) == 0 || len(state.Registry) == 0 {
		t.Fatalf("invalid PnP runtime state: %v", err)
	}
	locators := make(map[string]bool)
	type dependencyTarget struct {
		name   string
		target any
	}
	var targets []dependencyTarget
	for _, rawEntry := range state.Registry {
		var entry []json.RawMessage
		if err := json.Unmarshal(rawEntry, &entry); err != nil || len(entry) != 2 {
			t.Fatalf("invalid PnP registry entry: %v", err)
		}
		var name any
		if err := json.Unmarshal(entry[0], &name); err != nil {
			t.Fatalf("invalid PnP package name: %v", err)
		}
		var versions []json.RawMessage
		if err := json.Unmarshal(entry[1], &versions); err != nil || len(versions) == 0 {
			t.Fatalf("invalid PnP reference store: %v", err)
		}
		for _, rawVersion := range versions {
			var version []json.RawMessage
			if err := json.Unmarshal(rawVersion, &version); err != nil || len(version) != 2 {
				t.Fatalf("invalid PnP reference entry: %v", err)
			}
			var reference any
			if err := json.Unmarshal(version[0], &reference); err != nil || (name == nil) != (reference == nil) {
				t.Fatalf("invalid PnP top-level locator pairing: %v", err)
			}
			if name != nil {
				locatorName, nameOK := name.(string)
				locatorReference, referenceOK := reference.(string)
				if !nameOK || !referenceOK {
					t.Fatalf("invalid PnP locator %#v / %#v", name, reference)
				}
				locators[locatorName+"\x00"+locatorReference] = true
			}
			var info struct {
				Location     string            `json:"packageLocation"`
				Dependencies []json.RawMessage `json:"packageDependencies"`
				LinkType     string            `json:"linkType"`
			}
			if err := json.Unmarshal(version[1], &info); err != nil || !(strings.HasPrefix(info.Location, "./") || strings.HasPrefix(info.Location, "../")) || !strings.HasSuffix(info.Location, "/") || (info.LinkType != "HARD" && info.LinkType != "SOFT") {
				t.Fatalf("invalid PnP package information: %v", err)
			}
			for _, rawDependency := range info.Dependencies {
				var dependency []json.RawMessage
				if err := json.Unmarshal(rawDependency, &dependency); err != nil || len(dependency) != 2 {
					t.Fatalf("invalid PnP dependency tuple: %v", err)
				}
				var dependencyName string
				var target any
				if err := json.Unmarshal(dependency[0], &dependencyName); err != nil {
					t.Fatalf("invalid PnP dependency name: %v", err)
				}
				if err := json.Unmarshal(dependency[1], &target); err != nil {
					t.Fatalf("invalid PnP dependency target: %v", err)
				}
				switch target.(type) {
				case nil, string, []any:
				default:
					t.Fatalf("invalid PnP dependency target type %T", target)
				}
				targets = append(targets, dependencyTarget{name: dependencyName, target: target})
			}
		}
	}
	for _, target := range targets {
		switch value := target.target.(type) {
		case nil:
		case string:
			if !locators[target.name+"\x00"+value] {
				t.Fatalf("PnP dependency %q does not resolve to %q", target.name, value)
			}
		case []any:
			if len(value) != 2 {
				t.Fatalf("invalid PnP alias target %#v", value)
			}
			actualName, nameOK := value[0].(string)
			actualReference, referenceOK := value[1].(string)
			if !nameOK || !referenceOK || !locators[actualName+"\x00"+actualReference] {
				t.Fatalf("PnP alias target %#v has no package locator", value)
			}
		}
	}
}

func assertPnpmWorkspaceFixtureShape(t *testing.T, name string, document map[string]any) {
	t.Helper()
	if name == "pnpm-workspace-root-only" {
		if _, hasPackages := document["packages"]; hasPackages {
			t.Fatal("root-only workspace must omit packages")
		}
		if _, ok := document["catalog"].(map[string]any); !ok {
			t.Fatal("root-only workspace catalog must be a mapping")
		}
		return
	}
	if _, ok := document["packages"].([]any); !ok {
		t.Fatalf("workspace %s packages must be a list", name)
	}
	if strings.Contains(name, "catalog") {
		if _, ok := document["catalog"].(map[string]any); name == "pnpm-workspace-catalogs" && !ok {
			t.Fatal("default catalog must be a mapping")
		}
		if name == "pnpm-workspace-catalogs" {
			if _, ok := document["catalogs"].(map[string]any); !ok {
				t.Fatal("named catalogs must be a mapping")
			}
		}
	}
	if strings.Contains(name, "package-config-map") {
		if _, ok := document["packageConfigs"].(map[string]any); !ok {
			t.Fatal("map packageConfigs must be a mapping")
		}
	}
	if strings.Contains(name, "package-config-rules") {
		if _, ok := document["packageConfigs"].([]any); !ok {
			t.Fatal("rule packageConfigs must be a list")
		}
	}
}

func TestNpmAndYarnConfigFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, file string
		detector   DetectorID
		mustHave   []string
	}{
		{"npmrc-registry", ".npmrc", "js-npmrc", []string{"registry=https://registry.npmjs.org/", "tag=next"}},
		{"npmrc-scoped-registries", ".npmrc", "js-npmrc", []string{"@fixture:registry=", "@internal:registry="}},
		{"npmrc-auth-path", ".npmrc", "js-npmrc", []string{"@private:registry=", ":_authToken=${FIXTURE_TOKEN?}", ":certfile="}},
		{"npmrc-environment", ".npmrc", "js-npmrc", []string{"${HOME}", "${NODE_OPTIONS?}", "${NPM_REGISTRY?}"}},
		{"npmrc-arrays", ".npmrc", "js-npmrc", []string{"omit[]=dev", "workspace[]=packages/cli"}},
		{"npmrc-proxy-ca", ".npmrc", "js-npmrc", []string{"https-proxy=", "cafile="}},
		{"npmrc-basic-auth", ".npmrc", "js-npmrc", []string{":username=fixture-user", ":_password=", ":_auth="}},
		{"npmrc-comments-quoted", ".npmrc", "js-npmrc", []string{"# Project", "; The ini", "registry = \""}},
		{"yarnrc-plugins", ".yarnrc.yml", "js-yarnrc", []string{"plugins:", "@yarnpkg/plugin-workspace-tools"}},
		{"yarnrc-registries", ".yarnrc.yml", "js-yarnrc", []string{"npmScopes:", "npmRegistries:"}},
		{"yarnrc-package-extensions", ".yarnrc.yml", "js-yarnrc", []string{"packageExtensions:", "peerDependenciesMeta:"}},
		{"yarnrc-catalogs", ".yarnrc.yml", "js-yarnrc", []string{"catalog:", "catalogs:"}},
		{"yarnrc-patches-pnp", ".yarnrc.yml", "js-yarnrc", []string{"patchFolder:", "pnpEnableEsmLoader:"}},
		{"yarnrc-network", ".yarnrc.yml", "js-yarnrc", []string{"nodeLinker: pnp", "networkSettings:"}},
		{"yarnrc-git-policy", ".yarnrc.yml", "js-yarnrc", []string{"approvedGitRepositories:", "ssh://git@github.com"}},
		{"yarnrc-platforms", ".yarnrc.yml", "js-yarnrc", []string{"nodeLinker: pnpm", "supportedArchitectures:"}},
		{"yarnrc-package-gates", ".yarnrc.yml", "js-yarnrc", []string{"npmMinimalAgeGate:", "npmPreapprovedPackages:"}},
		{"yarnrc-environment", ".yarnrc.yml", "js-yarnrc", []string{"${YARN_CACHE}", "${YARN_REGISTRY:-", "${YARN_GLOBAL-"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "js", fixture.name, fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.mustHave {
				if !strings.Contains(string(contents), fragment) {
					t.Fatalf("fixture missing %q", fragment)
				}
			}
			if fixture.detector == "js-npmrc" {
				assertNpmrcFixture(t, contents)
			} else {
				var document map[string]any
				if err := yaml.Unmarshal(contents, &document); err != nil || len(document) == 0 {
					t.Fatalf("invalid Yarn YAML: %v", err)
				}
				assertYarnrcFixtureShape(t, fixture.name, document)
			}
			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			analysis := SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}
			if fixture.detector == "gleam" {
				analysis.Presence = PresencePresent
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.file || result.Sources[0].Analysis != analysis {
				t.Fatalf("unexpected selector-only source: %+v", result.Sources)
			}
		})
	}
}

func TestGoSumAndWorkFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, file string
		detector   DetectorID
	}{
		{"sum-basic", "go.sum", "go-sum"}, {"sum-go-mod", "go.sum", "go-sum"}, {"sum-multiple-versions", "go.sum", "go-sum"}, {"sum-pseudo-version", "go.sum", "go-sum"}, {"sum-uppercase-escape", "go.sum", "go-sum"}, {"sum-major-prerelease", "go.sum", "go-sum"},
		{"work-single", "go.work", "go-work"}, {"work-block", "go.work", "go-work"}, {"work-replace-local", "go.work", "go-work"}, {"work-replace-module", "go.work", "go-work"}, {"work-toolchain-godebug", "go.work", "go-work"}, {"work-replace-version-local", "go.work", "go-work"}, {"work-quoted-comments", "go.work", "go-work"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "go", fixture.name, fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if fixture.detector == "go-sum" {
				for _, line := range strings.FieldsFunc(strings.TrimSpace(string(contents)), func(r rune) bool { return r == '\n' }) {
					if fields := strings.Fields(line); len(fields) != 3 || !strings.HasPrefix(fields[2], "h1:") {
						t.Fatalf("invalid go.sum entry %q", line)
					}
				}
			} else if strings.Contains(string(contents), "godebug ") {
				if !strings.Contains(string(contents), "godebug default=") {
					t.Fatal("invalid godebug directive")
				}
			} else if work, err := modfile.ParseWork(path, contents, nil); err != nil || work.Go == nil {
				t.Fatalf("invalid go.work: %v", err)
			}
			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.file || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestCargoConfigAndCrystalShardLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, directory, file string
		detector              DetectorID
	}{
		{"cargo vendored directory", "rust/cargo-config-vendored", ".cargo/config.toml", "rust-cargo-config"},
		{"cargo sparse registry", "rust/cargo-config-sparse-registry", ".cargo/config.toml", "rust-cargo-config"},
		{"cargo sparse replacement source", "rust/cargo-config-sparse-source", ".cargo/config.toml", "rust-cargo-config"},
		{"cargo Git registry", "rust/cargo-config-git-registry", ".cargo/config.toml", "rust-cargo-config"},
		{"cargo local registry", "rust/cargo-config-local-registry", ".cargo/config.toml", "rust-cargo-config"},
		{"cargo Git source", "rust/cargo-config-git-source", ".cargo/config.toml", "rust-cargo-config"},
		{"cargo Git source branch and tag", "rust/cargo-config-git-refs", ".cargo/config.toml", "rust-cargo-config"},
		{"cargo patches and path overrides", "rust/cargo-config-patch-and-paths", ".cargo/config.toml", "rust-cargo-config"},
		{"cargo included config", "rust/cargo-config-includes", ".cargo/config.toml", "rust-cargo-config"},
		{"cargo legacy config filename", "rust/cargo-config-legacy", ".cargo/config", "rust-cargo-config"},
		{"shards v1 GitHub", "crystal/shard-lock-v1-github", "shard.lock", "crystal-shard-lock"},
		{"shards multiple Git", "crystal/shard-lock-multiple-git", "shard.lock", "crystal-shard-lock"},
		{"shards pseudo and prerelease versions", "crystal/shard-lock-pseudo-version", "shard.lock", "crystal-shard-lock"},
		{"shards local paths", "crystal/shard-lock-local-path", "shard.lock", "crystal-shard-lock"},
		{"shards Git transports", "crystal/shard-lock-git-transports", "shard.lock", "crystal-shard-lock"},
		{"shards Mercurial and Fossil", "crystal/shard-lock-hg-fossil", "shard.lock", "crystal-shard-lock"},
		{"shards quoted fields", "crystal/shard-lock-comments-quoted", "shard.lock", "crystal-shard-lock"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", fixture.directory, fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if fixture.detector == "rust-cargo-config" {
				assertCargoConfigFixture(t, fixture.directory, contents)
			} else {
				assertCrystalShardLockFixture(t, fixture.directory, contents)
			}
			root := filepath.Dir(path)
			if fixture.detector == "rust-cargo-config" {
				root = filepath.Dir(root)
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.file || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected selector-only source: %+v", result.Sources)
			}
		})
	}
}

func assertCargoConfigFixture(t *testing.T, fixture string, contents []byte) {
	t.Helper()
	var config map[string]any
	if _, err := toml.Decode(string(contents), &config); err != nil {
		t.Fatalf("parse Cargo config: %v", err)
	}
	sources, _ := config["source"].(map[string]any)
	source := func(name string) map[string]any {
		t.Helper()
		entry, ok := sources[name].(map[string]any)
		if !ok {
			t.Fatalf("missing Cargo source %q: %#v", name, sources)
		}
		return entry
	}
	for name, rawEntry := range sources {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			t.Fatalf("Cargo source %q is not a table: %#v", name, rawEntry)
		}
		definesSource := 0
		for _, key := range []string{"directory", "registry", "local-registry", "git"} {
			if entry[key] != nil {
				definesSource++
			}
		}
		if entry["replace-with"] != nil {
			if definesSource > 1 {
				t.Fatalf("Cargo replacement source %q defines multiple source locations: %#v", name, entry)
			}
		} else if definesSource != 1 {
			t.Fatalf("Cargo source %q must define exactly one source location: %#v", name, entry)
		}
	}
	switch filepath.Base(fixture) {
	case "cargo-config-vendored":
		if source("vendored-sources")["directory"] != "vendor" || source("crates-io")["replace-with"] != "vendored-sources" {
			t.Fatalf("invalid vendored replacement: %#v", sources)
		}
	case "cargo-config-sparse-registry":
		registries, _ := config["registries"].(map[string]any)
		acme, _ := registries["acme"].(map[string]any)
		registry, _ := config["registry"].(map[string]any)
		if index, _ := acme["index"].(string); !strings.HasPrefix(index, "sparse+https://") || registry["default"] != "acme" || len(sources) != 0 {
			t.Fatalf("invalid sparse registry configuration: %#v", config)
		}
	case "cargo-config-sparse-source":
		if registry, _ := source("sparse-mirror")["registry"].(string); !strings.HasPrefix(registry, "sparse+https://") || source("crates-io")["replace-with"] != "sparse-mirror" {
			t.Fatalf("invalid sparse source replacement: %#v", sources)
		}
	case "cargo-config-git-registry":
		if registry, _ := source("mirror")["registry"].(string); !strings.HasPrefix(registry, "ssh://") || source("crates-io")["replace-with"] != "mirror" {
			t.Fatalf("invalid Git registry replacement: %#v", sources)
		}
	case "cargo-config-local-registry":
		if source("local-cache")["local-registry"] != "cache/crates" || source("crates-io")["replace-with"] != "local-cache" {
			t.Fatalf("invalid local registry replacement: %#v", sources)
		}
	case "cargo-config-git-source":
		fork := source("internal-fork")
		rev, _ := fork["rev"].(string)
		if git, _ := fork["git"].(string); !strings.HasPrefix(git, "ssh://") || len(rev) != 40 || source("upstream")["replace-with"] != "internal-fork" {
			t.Fatalf("invalid Git source replacement: %#v", sources)
		}
	case "cargo-config-git-refs":
		branch, tag := source("internal-branch"), source("internal-tag")
		if branch["branch"] != "stable" || branch["tag"] != nil || branch["rev"] != nil || tag["tag"] != "v2.4.0" || tag["branch"] != nil || tag["rev"] != nil || source("upstream-branch")["replace-with"] != "internal-branch" || source("upstream-tag")["replace-with"] != "internal-tag" {
			t.Fatalf("invalid Git branch/tag source replacements: %#v", sources)
		}
	case "cargo-config-patch-and-paths":
		if paths, ok := config["paths"].([]any); !ok || !slices.Equal(paths, []any{"../shared/crates", "overrides/local-util"}) {
			t.Fatalf("invalid Cargo path overrides: %#v", config["paths"])
		}
		patches, _ := config["patch"].(map[string]any)
		cratesIO, _ := patches["crates-io"].(map[string]any)
		serde, _ := cratesIO["serde"].(map[string]any)
		if serde["tag"] != "v1.0.219" || serde["git"] == "" {
			t.Fatalf("invalid Cargo registry patch: %#v", cratesIO)
		}
	case "cargo-config-includes":
		includes, ok := config["include"].([]any)
		if !ok || len(includes) != 2 || includes[0] != "team-defaults.toml" {
			t.Fatalf("invalid Cargo includes: %#v", config["include"])
		}
		optional, optionalOK := includes[1].(map[string]any)
		if !optionalOK || optional["path"] != "developer-overrides.toml" || optional["optional"] != true {
			t.Fatalf("invalid optional Cargo include: %#v", config["include"])
		}
		defaults, err := os.ReadFile(filepath.Join("..", "..", "testdata", fixture, ".cargo", "team-defaults.toml"))
		if err != nil || !strings.Contains(string(defaults), "directory = \"vendor/team\"") {
			t.Fatalf("invalid included Cargo config: %v %q", err, defaults)
		}
	case "cargo-config-legacy":
		if source("legacy-vendor")["directory"] != "vendor" || source("crates-io")["replace-with"] != "legacy-vendor" {
			t.Fatalf("invalid legacy Cargo config: %#v", sources)
		}
	}
}

func assertCrystalShardLockFixture(t *testing.T, fixture string, contents []byte) {
	t.Helper()
	var lock struct {
		Version string                       `yaml:"version"`
		Shards  map[string]map[string]string `yaml:"shards"`
	}
	if err := yaml.Unmarshal(contents, &lock); err != nil {
		t.Fatalf("parse Shards lockfile: %v", err)
	}
	if (lock.Version != "1.0" && lock.Version != "2.0") || len(lock.Shards) == 0 {
		t.Fatalf("invalid Shards lock metadata: %#v", lock)
	}
	for name, shard := range lock.Shards {
		if name == "" || shard["version"] == "" {
			t.Fatalf("invalid locked shard %q: %#v", name, shard)
		}
		sources := 0
		for _, key := range []string{"git", "github", "hg", "fossil", "path"} {
			if shard[key] != "" {
				sources++
			}
		}
		if sources != 1 {
			t.Fatalf("locked shard must have exactly one source: %q %#v", name, shard)
		}
	}
	switch filepath.Base(fixture) {
	case "shard-lock-v1-github":
		if lock.Shards["amber_router"]["github"] != "amberframework/amber" {
			t.Fatalf("invalid legacy GitHub lock entry: %#v", lock.Shards)
		}
	case "shard-lock-pseudo-version":
		if !strings.Contains(lock.Shards["athena-mercure"]["version"], "+git.commit.") || !strings.HasSuffix(lock.Shards["prerelease"]["version"], "-rc1") || !strings.HasSuffix(lock.Shards["dot_prerelease"]["version"], ".rc1") {
			t.Fatalf("invalid pseudo or prerelease locked versions: %#v", lock.Shards)
		}
	case "shard-lock-multiple-git":
		if len(lock.Shards) != 3 || lock.Shards["kemal"]["version"] != "1.6.0" || lock.Shards["radix"]["git"] == lock.Shards["athena"]["git"] {
			t.Fatalf("invalid multiple resolved Git entries: %#v", lock.Shards)
		}
	case "shard-lock-local-path":
		for _, shard := range lock.Shards {
			if !strings.HasPrefix(shard["path"], "..") {
				t.Fatalf("invalid local-path lock entry: %#v", shard)
			}
		}
	case "shard-lock-git-transports":
		if !strings.HasPrefix(lock.Shards["private_client"]["git"], "git@") || !strings.HasPrefix(lock.Shards["gitlab_lib"]["git"], "https://gitlab.com/") || !strings.HasPrefix(lock.Shards["git_protocol_lib"]["git"], "git://") {
			t.Fatalf("missing supported Git transports: %#v", lock.Shards)
		}
	case "shard-lock-hg-fossil":
		if !strings.HasPrefix(lock.Shards["mercurial_lib"]["hg"], "https://hg.") || !strings.Contains(lock.Shards["mercurial_lib"]["version"], "+hg.commit.") || !strings.HasPrefix(lock.Shards["fossil_lib"]["fossil"], "https://fossil.") || !strings.Contains(lock.Shards["fossil_lib"]["version"], "+fossil.commit.") {
			t.Fatalf("invalid Mercurial or Fossil lock entries: %#v", lock.Shards)
		}
	case "shard-lock-comments-quoted":
		if lock.Shards["quoted_name"]["version"] != "2025.07.1" || !strings.HasPrefix(lock.Shards["underscore_dep"]["git"], "https://codeberg.org/") {
			t.Fatalf("invalid quoted Shards fields: %#v", lock.Shards)
		}
	}
}

func TestGleamAndSbtBuildPropsFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		directory, file string
		detector        DetectorID
		wantVersion     string
	}{
		{"scala/sbt-build-props-legacy", "project/build.properties", "scala-sbt-build-props", "0.13.18"},
		{"scala/sbt-build-props-modern", "project/build.properties", "scala-sbt-build-props", "1.10.10"},
		{"scala/sbt-build-props-v2", "project/build.properties", "scala-sbt-build-props", "2.0.4"},
		{"scala/sbt-build-props-prerelease", "project/build.properties", "scala-sbt-build-props", "1.0.0-RC2"},
		{"scala/sbt-build-props-comments", "project/build.properties", "scala-sbt-build-props", "1.9.9"},
		{"scala/sbt-build-props-colon", "project/build.properties", "scala-sbt-build-props", "1.10.10"},
		{"scala/sbt-build-props-nested", "modules/api/project/build.properties", "scala-sbt-build-props", "1.8.3"},
		{"scala/sbt-build-props-meta", "project/project/build.properties", "scala-sbt-build-props", "1.9.8"},
		{"gleam/config-runtime", "gleam.toml", "gleam", ""},
		{"gleam/config-dev-path", "gleam.toml", "gleam", ""},
		{"gleam/config-legacy-dev", "gleam.toml", "gleam", ""},
		{"gleam/config-runtime-path", "gleam.toml", "gleam", ""},
		{"gleam/config-git-refs", "gleam.toml", "gleam", ""},
		{"gleam/config-javascript", "gleam.toml", "gleam", ""},
		{"gleam/config-erlang", "gleam.toml", "gleam", ""},
		{"gleam/config-tools", "gleam.toml", "gleam", ""},
		{"gleam/config-version-ranges", "gleam.toml", "gleam", ""},
		{"gleam/manifest-single", "manifest.toml", "gleam-manifest", ""},
		{"gleam/manifest-transitive", "manifest.toml", "gleam-manifest", ""},
		{"gleam/manifest-build-tools", "manifest.toml", "gleam-manifest", ""},
		{"gleam/manifest-dev-deps", "manifest.toml", "gleam-manifest", ""},
		{"gleam/manifest-legacy-requirements", "manifest.toml", "gleam-manifest", ""},
		{"gleam/manifest-git-local", "manifest.toml", "gleam-manifest", ""},
		{"gleam/manifest-resolved-single", "manifest.toml", "gleam-manifest", ""},
	} {
		t.Run(fixture.directory, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", fixture.directory, fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			switch fixture.detector {
			case "scala-sbt-build-props":
				assertSbtBuildPropsFixture(t, contents, fixture.wantVersion)
			case "gleam":
				assertGleamConfigFixture(t, fixture.directory, contents)
			case "gleam-manifest":
				assertGleamManifestFixture(t, fixture.directory, contents)
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", fixture.directory), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			analysis := SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}
			if fixture.detector == "gleam" {
				analysis.Presence = PresencePresent
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.file || result.Sources[0].Analysis != analysis {
				t.Fatalf("unexpected selector-only source: %+v", result.Sources)
			}
		})
	}
}

func assertSbtBuildPropsFixture(t *testing.T, contents []byte, wantVersion string) {
	t.Helper()
	properties := map[string]string{}
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			key, value, found = strings.Cut(line, ":")
		}
		if !found || strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			t.Fatalf("invalid SBT properties assignment %q", line)
		}
		properties[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if properties["sbt.version"] != wantVersion {
		t.Fatalf("unexpected sbt.version: got %q want %q", properties["sbt.version"], wantVersion)
	}
}

func assertGleamConfigFixture(t *testing.T, fixture string, contents []byte) {
	t.Helper()
	var config map[string]any
	if _, err := toml.Decode(string(contents), &config); err != nil {
		t.Fatalf("parse Gleam config: %v", err)
	}
	if config["name"] == "" || config["version"] == "" {
		t.Fatalf("invalid Gleam package metadata: %#v", config)
	}
	dependencies, ok := config["dependencies"].(map[string]any)
	if !ok || len(dependencies) == 0 {
		t.Fatalf("Gleam config has no dependency table: %#v", config)
	}
	for name, requirement := range dependencies {
		if name == "" || requirement == nil {
			t.Fatalf("invalid runtime dependency %q: %#v", name, requirement)
		}
		if table, ok := requirement.(map[string]any); ok && table["path"] == nil && (table["git"] == nil || table["ref"] == nil) {
			t.Fatalf("invalid inline runtime dependency %q: %#v", name, requirement)
		}
	}
	if strings.HasSuffix(fixture, "config-dev-path") {
		devDependencies, _ := config["dev_dependencies"].(map[string]any)
		pathDependency, _ := devDependencies["build_tools"].(map[string]any)
		if devDependencies["gleeunit"] == "" || pathDependency["path"] != "../build-tools" {
			t.Fatalf("invalid Gleam dev dependencies: %#v", devDependencies)
		}
	}
	if strings.HasSuffix(fixture, "config-legacy-dev") {
		devDependencies, _ := config["dev-dependencies"].(map[string]any)
		if devDependencies["gleeunit"] == "" {
			t.Fatalf("invalid legacy Gleam dev dependencies: %#v", devDependencies)
		}
	}
	if strings.HasSuffix(fixture, "config-runtime-path") {
		pathDependency, _ := dependencies["shared_domain"].(map[string]any)
		if pathDependency["path"] != "../shared-domain" {
			t.Fatalf("invalid runtime path dependency: %#v", dependencies)
		}
	}
	if strings.HasSuffix(fixture, "config-git-refs") {
		for _, name := range []string{"tagged_lib", "branch_lib", "commit_lib"} {
			dependency, _ := dependencies[name].(map[string]any)
			git, _ := dependency["git"].(string)
			if !strings.HasPrefix(git, "https://") || dependency["ref"] == "" {
				t.Fatalf("invalid Git dependency %q: %#v", name, dependency)
			}
		}
	}
	if strings.HasSuffix(fixture, "config-javascript") {
		javascript, _ := config["javascript"].(map[string]any)
		if config["target"] != "javascript" || javascript["runtime"] != "bun" || javascript["typescript_declarations"] != true {
			t.Fatalf("invalid Gleam JavaScript config: %#v", config)
		}
	}
	if strings.HasSuffix(fixture, "config-erlang") && config["target"] != "erlang" {
		t.Fatalf("invalid Gleam Erlang config: %#v", config)
	}
	if strings.HasSuffix(fixture, "config-tools") {
		tools, _ := config["tools"].(map[string]any)
		lustre, _ := tools["lustre"].(map[string]any)
		if lustre["dev"] == nil || lustre["build"] == nil {
			t.Fatalf("invalid Gleam tool config: %#v", tools)
		}
	}
}

func assertGleamManifestFixture(t *testing.T, fixture string, contents []byte) {
	t.Helper()
	var manifest map[string]any
	if _, err := toml.Decode(string(contents), &manifest); err != nil {
		t.Fatalf("parse Gleam manifest: %v", err)
	}
	requirements, ok := manifest["requirements"].(map[string]any)
	if !ok || len(requirements) == 0 {
		t.Fatalf("Gleam manifest has no requirements: %#v", manifest)
	}
	for name, rawRequirement := range requirements {
		if name == "" || rawRequirement == nil {
			t.Fatalf("invalid Gleam requirement %q: %#v", name, rawRequirement)
		}
		switch requirement := rawRequirement.(type) {
		case string:
			if requirement == "" {
				t.Fatalf("empty legacy requirement %q", name)
			}
		case map[string]any:
			if requirement["version"] == "" && requirement["path"] == "" && (requirement["git"] == "" || requirement["ref"] == "") {
				t.Fatalf("missing requirement source %q: %#v", name, requirement)
			}
		default:
			t.Fatalf("invalid requirement %q: %#v", name, rawRequirement)
		}
	}
	if strings.HasSuffix(fixture, "manifest-legacy-requirements") {
		if manifest["packages"] != nil {
			t.Fatalf("legacy manifest must not have packages: %#v", manifest)
		}
		return
	}
	rawPackages, ok := manifest["packages"].([]any)
	if !ok || len(rawPackages) == 0 {
		t.Fatalf("Gleam manifest has no resolved packages: %#v", manifest)
	}
	packages := make([]map[string]any, 0, len(rawPackages))
	packageNames := map[string]struct{}{}
	for _, rawPackage := range rawPackages {
		pkg, ok := rawPackage.(map[string]any)
		if !ok {
			t.Fatalf("invalid resolved Gleam package: %#v", rawPackage)
		}
		packages = append(packages, pkg)
		name, _ := pkg["name"].(string)
		if name == "" || pkg["version"] == "" {
			t.Fatalf("invalid resolved Gleam package: %#v", pkg)
		}
		packageNames[name] = struct{}{}
		switch pkg["source"] {
		case "hex":
			checksum, _ := pkg["outer_checksum"].(string)
			if pkg["otp_app"] == "" || len(checksum) != 64 || strings.Trim(checksum, "0123456789abcdefABCDEF") != "" {
				t.Fatalf("invalid Hex resolved Gleam package: %#v", pkg)
			}
		case "git":
			repo, _ := pkg["repo"].(string)
			commit, _ := pkg["commit"].(string)
			if !strings.HasPrefix(repo, "https://") || len(commit) != 40 {
				t.Fatalf("invalid Git resolved Gleam package: %#v", pkg)
			}
		case "local":
			path, _ := pkg["path"].(string)
			if !strings.HasPrefix(path, "..") {
				t.Fatalf("invalid local resolved Gleam package: %#v", pkg)
			}
		default:
			t.Fatalf("unknown resolved Gleam package source: %#v", pkg)
		}
		if buildTools, ok := pkg["build_tools"].([]any); !ok || len(buildTools) == 0 {
			t.Fatalf("resolved Gleam package lacks build tools: %#v", pkg)
		}
	}
	if strings.HasSuffix(fixture, "manifest-build-tools") && packages[0]["build_tools"].([]any)[0] != "rebar3" {
		t.Fatalf("expected rebar3 build tool: %#v", packages[0])
	}
	if strings.HasSuffix(fixture, "manifest-transitive") && len(packages) != 3 {
		t.Fatalf("expected transitive package closure: %#v", packages)
	}
	for name := range requirements {
		if _, ok := packageNames[name]; !ok {
			t.Fatalf("direct requirement %q has no resolved package: %#v", name, packageNames)
		}
	}
	if strings.HasSuffix(fixture, "manifest-git-local") {
		if _, ok := packageNames["git_tool"]; !ok {
			t.Fatalf("missing Git package entry: %#v", packageNames)
		}
		if _, ok := packageNames["local_tool"]; !ok {
			t.Fatalf("missing Git/local package entries: %#v", packageNames)
		}
	}
}

func TestGradleWrapperAndIvySettingsFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		directory, file string
		detector        DetectorID
	}{
		{"java/gradle-wrapper-bin", "gradle/wrapper/gradle-wrapper.properties", "java-gradle-wrapper"},
		{"java/gradle-wrapper-all-checksum", "gradle/wrapper/gradle-wrapper.properties", "java-gradle-wrapper"},
		{"java/gradle-wrapper-mirror", "gradle/wrapper/gradle-wrapper.properties", "java-gradle-wrapper"},
		{"java/gradle-wrapper-authenticated", "gradle/wrapper/gradle-wrapper.properties", "java-gradle-wrapper"},
		{"java/gradle-wrapper-file", "gradle/wrapper/gradle-wrapper.properties", "java-gradle-wrapper"},
		{"java/gradle-wrapper-network", "gradle/wrapper/gradle-wrapper.properties", "java-gradle-wrapper"},
		{"java/gradle-wrapper-escaped", "gradle/wrapper/gradle-wrapper.properties", "java-gradle-wrapper"},
		{"java/ivy-settings-basic", "ivysettings.xml", "java-ivy-settings"},
		{"java/ivy-settings-filesystem", "ivysettings.xml", "java-ivy-settings"},
		{"java/ivy-settings-chain", "ivysettings.xml", "java-ivy-settings"},
		{"java/ivy-settings-url-credentials", "ivysettings.xml", "java-ivy-settings"},
		{"java/ivy-settings-include", "ivysettings.xml", "java-ivy-settings"},
		{"java/ivy-settings-module-routing", "ivysettings.xml", "java-ivy-settings"},
		{"java/ivy-settings-dual-sftp", "ivysettings.xml", "java-ivy-settings"},
		{"java/ivy-settings-properties", "ivysettings.xml", "java-ivy-settings"},
	} {
		t.Run(fixture.directory, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", fixture.directory, fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if fixture.detector == "java-gradle-wrapper" {
				assertGradleWrapperFixture(t, fixture.directory, contents)
			} else {
				assertIvySettingsFixture(t, fixture.directory, contents)
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", fixture.directory), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.file || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected selector-only source: %+v", result.Sources)
			}
		})
	}
}

func TestHelmAndBufDependencyFixtureShapes(t *testing.T) {
	for _, fixture := range []struct{ dir, file, key string }{
		{"helm/chart-deps-registry", "Chart.yaml", "dependencies"}, {"helm/chart-deps-alias", "Chart.yaml", "dependencies"}, {"helm/chart-deps-local", "Chart.yaml", "dependencies"}, {"helm/chart-deps-oci", "Chart.yaml", "dependencies"}, {"helm/chart-deps-optional", "Chart.yaml", "dependencies"}, {"helm/chart-deps-vendored", "Chart.yaml", "dependencies"},
		{"buf/module-v1-deps", "buf.yaml", "deps"}, {"buf/module-v2-deps", "buf.yaml", "deps"}, {"buf/module-v2-private-deps", "buf.yaml", "deps"}, {"buf/module-v2-multimodule-deps", "buf.yaml", "deps"}, {"buf/module-v1beta1-deps", "buf.yaml", "deps"}, {"buf/module-v2-comments-quoted", "buf.yaml", "deps"},
	} {
		t.Run(fixture.dir, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", fixture.dir, fixture.file)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := yaml.Unmarshal(content, &doc); err != nil {
				t.Fatal(err)
			}
			entries, ok := doc[fixture.key].([]any)
			if !ok || len(entries) == 0 {
				t.Fatalf("missing %s: %#v", fixture.key, doc)
			}
			if strings.HasPrefix(fixture.dir, "buf/") {
				for _, raw := range entries {
					dep, ok := raw.(string)
					if !ok || strings.Contains(dep, "://") || len(strings.Split(strings.Split(dep, ":")[0], "/")) != 3 {
						t.Fatalf("invalid BSR dependency %#v", raw)
					}
				}
			} else {
				for _, raw := range entries {
					dep, ok := raw.(map[string]any)
					if !ok || dep["name"] == "" || dep["version"] == "" {
						t.Fatalf("invalid chart dependency %#v", raw)
					}
				}
			}
		})
	}
}

func TestRakuMetaAndGodotPluginConfigFixtureShapes(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		directory, file string
		detector        DetectorID
	}{
		{"raku/meta-runtime-array", "META6.json", "raku-meta"},
		{"raku/meta-phase-dependencies", "META6.json", "raku-meta"},
		{"raku/meta-runtime-build-map", "META6.json", "raku-meta"},
		{"raku/meta-version-api-constraints", "META6.json", "raku-meta"},
		{"raku/meta-provides-resources", "META6.json", "raku-meta"},
		{"godot/plugin-cfg-gdscript", "addons/demo_tools/plugin.cfg", "godot-plugin-cfg"},
		{"godot/plugin-cfg-csharp", "addons/csharp_tools/plugin.cfg", "godot-plugin-cfg"},
		{"godot/plugin-cfg-nested-icon", "addons/company/terrain/plugin.cfg", "godot-plugin-cfg"},
		{"godot/plugin-cfg-importer", "addons/material_importer/plugin.cfg", "godot-plugin-cfg"},
		{"godot/plugin-cfg-comments", "addons/localization/plugin.cfg", "godot-plugin-cfg"},
		{"godot/plugin-cfg-spaced-path", "addons/scene_tools/plugin.cfg", "godot-plugin-cfg"},
	} {
		t.Run(fixture.directory, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", fixture.directory, fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if fixture.detector == "raku-meta" {
				var manifest map[string]any
				if err := json.Unmarshal(contents, &manifest); err != nil {
					t.Fatalf("invalid META6 JSON: %v", err)
				}
				if manifest["name"] == "" || manifest["version"] == "" || manifest["depends"] == nil {
					t.Fatalf("invalid META6 manifest: %#v", manifest)
				}
				if fixture.directory == "raku/meta-runtime-build-map" {
					depends, ok := manifest["depends"].(map[string]any)
					if !ok {
						t.Fatalf("depends map has wrong shape: %#v", manifest["depends"])
					}
					for _, phase := range []string{"runtime", "build"} {
						entry, ok := depends[phase].(map[string]any)
						if !ok || entry["requires"] == nil {
							t.Fatalf("depends.%s requires is missing: %#v", phase, depends[phase])
						}
					}
				}
				if fixture.directory == "raku/meta-version-api-constraints" && !strings.Contains(string(contents), ":auth<") {
					t.Fatal("qualified dependencies must cover an auth constraint")
				}
			} else {
				config := parseINISection(t, contents, "plugin")
				for _, key := range []string{"name", "description", "author", "version", "script"} {
					if strings.Trim(config[key], "\\\"") == "" {
						t.Fatalf("plugin config missing %q: %#v", key, config)
					}
				}
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", fixture.directory), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.file || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected selector-only source: %+v", result.Sources)
			}
		})
	}
}

func TestBunBinaryLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []string{
		"bun-lockb-facepoke-client",
		"bun-lockb-simpleicons-current",
		"bun-lockb-simpleicons-historical",
		"bun-lockb-simpleicons-v0",
		"bun-lockb-coolify-example",
	} {
		t.Run(fixture, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "javascript", fixture)
			contents, err := os.ReadFile(filepath.Join(root, "bun.lockb"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			const header = "#!/usr/bin/env bun\nbun-lockfile-format-v0\n"
			if !strings.HasPrefix(string(contents), header) || len(contents) <= len(header) || !hasNonASCIIBinaryByte(contents[len(header):]) {
				t.Fatal("fixture is not an opaque Bun v0 binary lockfile")
			}
			var provenance map[string]string
			provenanceBytes, err := os.ReadFile(filepath.Join(root, "fixture.json"))
			if err != nil {
				t.Fatalf("read fixture provenance: %v", err)
			}
			if err := json.Unmarshal(provenanceBytes, &provenance); err != nil {
				t.Fatalf("parse fixture provenance: %v", err)
			}
			sum := sha256.Sum256(contents)
			if provenance["source"] == "" || provenance["format"] != "bun-lockfile-format-v0" || provenance["kind"] != "published legacy Bun lockfile" || provenance["sha256"] != hex.EncodeToString(sum[:]) {
				t.Fatalf("invalid fixture provenance: %#v", provenance)
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "js-bun-lockb" || result.Sources[0].Path != "bun.lockb" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected selector-only source: %+v", result.Sources)
			}
		})
	}
}

func TestImportMapFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name                                string
		minImports, minScopes, minIntegrity int
	}{
		{"importmap-bare-prefix", 4, 0, 0},
		{"importmap-url-specifiers", 4, 0, 0},
		{"importmap-scoped-overrides", 2, 2, 0},
		{"importmap-integrity", 2, 0, 2},
		{"importmap-normalized-paths", 3, 0, 0},
		{"importmap-scopes-only", 0, 2, 0},
		{"importmap-integrity-only", 0, 0, 1},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "js", fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, "importmap.json"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var document map[string]any
			if err := json.Unmarshal(contents, &document); err != nil {
				t.Fatalf("invalid import map JSON: %v", err)
			}
			assertImportMapFixtureShape(t, fixture.name, document, fixture.minImports, fixture.minScopes, fixture.minIntegrity)
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "js-importmap" || result.Sources[0].Path != "importmap.json" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected selector-only source: %+v", result.Sources)
			}
		})
	}
}

func TestVcpkgConfigurationFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name                           string
		registries, overlays, requires int
		wantDefaultRegistryNull        bool
	}{
		{"vcpkg-config-builtin-baseline", 0, 0, 0, false},
		{"vcpkg-config-git-default", 0, 0, 0, false},
		{"vcpkg-config-package-routing", 2, 0, 0, true},
		{"vcpkg-config-filesystem", 1, 0, 0, false},
		{"vcpkg-config-overlays", 0, 4, 0, false},
		{"vcpkg-config-artifact", 1, 0, 0, false},
		{"vcpkg-config-artifact-requires", 0, 0, 2, false},
		{"vcpkg-config-builtin-default", 0, 0, 0, false},
		{"vcpkg-config-git-ssh", 1, 0, 0, false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "cpp", fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, "vcpkg-configuration.json"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var configuration map[string]any
			if err := json.Unmarshal(contents, &configuration); err != nil {
				t.Fatalf("invalid vcpkg configuration JSON: %v", err)
			}
			assertVcpkgConfigurationFixtureShape(t, fixture.name, configuration, fixture.registries, fixture.overlays, fixture.requires, fixture.wantDefaultRegistryNull)
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "cpp-vcpkg-config" || result.Sources[0].Path != "vcpkg-configuration.json" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected selector-only source: %+v", result.Sources)
			}
		})
	}
}

func TestMesonWrapFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, file, section string
	}{
		{"meson-wrap-file-remote", "libpng.wrap", "wrap-file"},
		{"meson-wrap-file-patch", "zlib.wrap", "wrap-file"},
		{"meson-wrap-file-local", "fmt.wrap", "wrap-file"},
		{"meson-wrap-file-patch-directory", "default-dir.wrap", "wrap-file"},
		{"meson-wrap-git", "spdlog.wrap", "wrap-git"},
		{"meson-wrap-git-commit", "private-lib.wrap", "wrap-git"},
		{"meson-wrap-hg", "libfoo.wrap", "wrap-hg"},
		{"meson-wrap-svn", "libbar.wrap", "wrap-svn"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "cpp", fixture.name)
			path := filepath.Join(root, "subprojects", fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			wrap := parseINISection(t, contents, fixture.section)
			assertMesonWrapFixtureShape(t, fixture.name, fixture.section, contents, wrap)
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "cpp-meson-wrap" || result.Sources[0].Path != filepath.ToSlash(filepath.Join("subprojects", fixture.file)) || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected selector-only source: %+v", result.Sources)
			}
		})
	}
}

func TestCMakeModuleFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, file string
		commands   []string
	}{
		{"cmake-module-basic", "FindMathExtras.cmake", []string{"find_package(Eigen3 REQUIRED)", "find_package(ZLIB QUIET)"}},
		{"cmake-module-components", "FindQtTools.cmake", []string{"COMPONENTS Core Gui Widgets", "OPTIONAL_COMPONENTS SSL Crypto"}},
		{"cmake-module-config", "FindVendorStack.cmake", []string{"CONFIG REQUIRED", "NAMES VendorSDK vendor-sdk", "CONFIGS VendorSDKConfig.cmake", "NO_DEFAULT_PATH"}},
		{"cmake-module-version-range", "FindCodecStack.cmake", []string{"10...<12", "MODULE", "EXACT GLOBAL"}},
		{"cmake-module-transitive", "ExampleConfig.cmake", []string{"include(CMakeFindDependencyMacro)", "find_dependency(ZLIB 1.3 REQUIRED)", "find_dependency(OpenSSL 3.0 COMPONENTS SSL Crypto)"}},
		{"cmake-module-fetch-content", "Dependencies.cmake", []string{"FetchContent_Declare(", "GIT_REPOSITORY", "GIT_TAG 10.2.1", "URL_HASH SHA256="}},
		{"cmake-module-find-artifacts", "FindAcmeCodec.cmake", []string{"include(FindPackageHandleStandardArgs)", "find_path(AcmeCodec_INCLUDE_DIR", "find_library(AcmeCodec_LIBRARY", "find_package_handle_standard_args(AcmeCodec"}},
		{"cmake-module-external-project", "ExternalDependencies.cmake", []string{"include(ExternalProject)", "ExternalProject_Add(private_codec", "GIT_TAG 0123456789abcdef0123456789abcdef01234567", "URL_HASH SHA256="}},
		{"cmake-module-fetch-populate", "LegacyContent.cmake", []string{"FetchContent_Declare(legacy_fmt", "FetchContent_GetProperties(legacy_fmt)", "FetchContent_Populate(legacy_fmt)", "add_subdirectory(${legacy_fmt_SOURCE_DIR}"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "cpp", fixture.name)
			path := filepath.Join(root, "cmake", fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, command := range fixture.commands {
				if !strings.Contains(string(contents), command) {
					t.Fatalf("fixture missing %q", command)
				}
			}
			assertCMakeModuleFixtureShape(t, fixture.name, string(contents))
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			wantPath := filepath.ToSlash(filepath.Join("cmake", fixture.file))
			if len(result.Sources) != 1 || result.Sources[0].Detector != "cpp-cmake-modules" || result.Sources[0].Path != wantPath || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected selector-only source: %+v", result.Sources)
			}
		})
	}
}

func TestBazelThirdPartyBzlFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name      string
		fragments []string
	}{
		{"third-party-bzl-http-archive", []string{"http_archive(", "sha256 =", "strip_prefix ="}},
		{"third-party-bzl-http-patches", []string{"urls = [", "integrity =", "patches =", "build_file_content ="}},
		{"third-party-bzl-http-file", []string{"http_file(", "downloaded_file_path =", "executable = False"}},
		{"third-party-bzl-http-jar", []string{"http_jar(", "urls = [", "sha256 ="}},
		{"third-party-bzl-git-local", []string{"git_repository(", "commit =", "tag = \"v2.0.0\"", "native.local_repository(", "native.new_local_repository(", "build_file_content ="}},
		{"third-party-bzl-maven", []string{"maven_install(", "artifacts = [", "repositories = [", "excluded_artifacts ="}},
		{"third-party-bzl-go", []string{"go_repository(", "importpath =", "version =", "sum =", "remote ="}},
		{"third-party-bzl-module-extension", []string{"def _third_party_impl(module_ctx):", "http_archive(", "module_extension("}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "bazel", fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, "third_party", "deps.bzl"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(contents), fragment) {
					t.Fatalf("fixture missing %q", fragment)
				}
			}
			assertBazelThirdPartyFixtureShape(t, fixture.name, string(contents))
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "bazel-third-party-bzl" || result.Sources[0].Path != "third_party/deps.bzl" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected selector-only source: %+v", result.Sources)
			}
		})
	}
}

func assertBazelThirdPartyFixtureShape(t *testing.T, fixture, contents string) {
	t.Helper()
	if strings.Contains(contents, "http_archive(") || strings.Contains(contents, "http_file(") || strings.Contains(contents, "http_jar(") {
		if !strings.Contains(contents, "//tools/build_defs/repo:http.bzl\"") {
			t.Fatal("HTTP repository rules must be loaded")
		}
		if !strings.Contains(contents, "sha256 =") && !strings.Contains(contents, "integrity =") {
			t.Fatal("HTTP repository dependencies require sha256 or integrity")
		}
		for _, line := range strings.Split(contents, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "sha256 =") {
				value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "sha256 =")), "\",")
				if !isSHA256(value) || value != strings.ToLower(value) {
					t.Fatalf("invalid HTTP SHA256 %q", value)
				}
			}
			if strings.HasPrefix(line, "integrity =") {
				value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "integrity =")), "\",")
				encoded := strings.TrimPrefix(value, "sha256-")
				if encoded == value || len(encoded) == 0 {
					t.Fatalf("invalid HTTP integrity %q", value)
				}
				if decoded, err := base64.StdEncoding.DecodeString(encoded); err != nil || len(decoded) != 32 {
					t.Fatalf("invalid HTTP integrity %q", value)
				}
			}
		}
		if strings.Contains(contents, "strip_prefix =") && strings.Contains(contents, "strip_components =") {
			t.Fatal("http_archive cannot combine strip_prefix and strip_components")
		}
	}
	if strings.Contains(contents, "git_repository(") && (!(strings.Contains(contents, "commit =") || strings.Contains(contents, "tag =") || strings.Contains(contents, "branch =")) || !strings.Contains(contents, "//tools/build_defs/repo:git.bzl\"")) {
		t.Fatal("Git repository dependencies must load the rule and select a revision")
	}
	if strings.Contains(contents, "new_local_repository(") && (!strings.Contains(contents, "path =") || (strings.Contains(contents, "build_file =") && strings.Contains(contents, "build_file_content ="))) {
		t.Fatal("new_local_repository requires a path and exactly one build-file source")
	}
	if strings.Contains(contents, "module_extension(") && !strings.Contains(contents, "module_ctx") {
		t.Fatal("module extension fixture must declare an implementation")
	}
	if !strings.Contains(contents, "def third_party_dependencies():") && !strings.Contains(contents, "module_extension(") {
		t.Fatalf("third-party file %q has no dependency macro or module extension", fixture)
	}
}

func assertCMakeModuleFixtureShape(t *testing.T, fixture, contents string) {
	t.Helper()
	if strings.Contains(contents, "find_dependency(") && !strings.Contains(contents, "include(CMakeFindDependencyMacro)") {
		t.Fatal("find_dependency requires CMakeFindDependencyMacro")
	}
	if strings.Contains(contents, "FetchContent_Declare(") {
		if !strings.Contains(contents, "FetchContent_MakeAvailable(") && !strings.Contains(contents, "FetchContent_Populate(") {
			t.Fatal("declared FetchContent dependencies must be made available or populated")
		}
		if strings.Contains(contents, "GIT_REPOSITORY") && !strings.Contains(contents, "GIT_TAG") {
			t.Fatal("Git FetchContent dependency needs a tag or revision")
		}
	}
	if strings.Contains(contents, "ExternalProject_Add(") {
		if !strings.Contains(contents, "include(ExternalProject)") {
			t.Fatal("ExternalProject_Add requires the ExternalProject module")
		}
		if strings.Contains(contents, "GIT_REPOSITORY") && !strings.Contains(contents, "GIT_TAG 0123456789abcdef0123456789abcdef01234567") {
			t.Fatal("Git ExternalProject dependency must pin an immutable revision")
		}
		if strings.Contains(contents, "URL ") && !strings.Contains(contents, "URL_HASH SHA256=") {
			t.Fatal("URL ExternalProject dependency must include a SHA256 hash")
		}
	}
	if !strings.Contains(contents, "find_package(") && !strings.Contains(contents, "find_dependency(") && !strings.Contains(contents, "FetchContent_Declare(") && !strings.Contains(contents, "find_path(") && !strings.Contains(contents, "find_library(") && !strings.Contains(contents, "ExternalProject_Add(") {
		t.Fatalf("CMake module %q has no dependency declaration", fixture)
	}
}

func assertMesonWrapFixtureShape(t *testing.T, fixture, section string, contents []byte, wrap map[string]string) {
	t.Helper()
	primarySections := 0
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if line == "[wrap-file]" || line == "[wrap-git]" || line == "[wrap-hg]" || line == "[wrap-svn]" {
			primarySections++
		}
	}
	if primarySections != 1 {
		t.Fatalf("%s must have exactly one recognized wrap section", fixture)
	}
	if directory, hasDirectory := wrap["directory"]; hasDirectory && directory == "" {
		t.Fatalf("%s wrap has an empty directory", fixture)
	}
	if method, ok := wrap["method"]; ok && method != "meson" && method != "cmake" && method != "cargo" {
		t.Fatalf("invalid wrap method %q", method)
	}
	if section == "wrap-file" {
		if wrap["source_filename"] == "" {
			t.Fatal("wrap-file requires source_filename")
		}
		if sourceURL := wrap["source_url"]; sourceURL != "" {
			if !isImportMapURL(sourceURL) || !isSHA256(wrap["source_hash"]) {
				t.Fatalf("invalid remote source: %#v", wrap)
			}
		}
		if patchURL := wrap["patch_url"]; patchURL != "" && (!isImportMapURL(patchURL) || !isSHA256(wrap["patch_hash"])) {
			t.Fatalf("invalid remote patch: %#v", wrap)
		}
		return
	}
	if wrap["url"] == "" || wrap["revision"] == "" {
		t.Fatalf("%s requires URL and revision: %#v", section, wrap)
	}
	if depth, ok := wrap["depth"]; ok && (section != "wrap-git" || depth == "0") {
		t.Fatal("wrap depth must be positive")
	}
	provide := parseINISection(t, contents, "provide")
	if wrap["method"] == "cmake" || wrap["method"] == "cargo" {
		if len(provide) == 0 {
			t.Fatalf("%s wraps require provided dependency mappings", wrap["method"])
		}
	}
	for name, target := range provide {
		if name == "" || target == "" {
			t.Fatalf("invalid provided dependency %q = %q", name, target)
		}
	}
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func assertVcpkgConfigurationFixtureShape(t *testing.T, fixture string, configuration map[string]any, wantRegistries, wantOverlays, wantRequires int, wantDefaultRegistryNull bool) {
	t.Helper()
	if baseline, ok := configuration["builtin-baseline"].(string); ok && !isVcpkgGitBaseline(baseline) {
		t.Fatalf("builtin baseline must be a Git commit: %q", baseline)
	}
	if wantDefaultRegistryNull && configuration["default-registry"] != nil {
		t.Fatalf("default registry must be explicitly disabled: %#v", configuration["default-registry"])
	}
	if registry, ok := configuration["default-registry"].(map[string]any); ok {
		assertVcpkgRegistry(t, registry, false)
	}
	registries, _ := configuration["registries"].([]any)
	if len(registries) != wantRegistries {
		t.Fatalf("expected %d registries, got %#v", wantRegistries, registries)
	}
	for _, raw := range registries {
		registry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("invalid registry: %#v", raw)
		}
		assertVcpkgRegistry(t, registry, true)
	}
	overlayCount := 0
	for _, key := range []string{"overlay-ports", "overlay-triplets"} {
		if raw, ok := configuration[key]; ok {
			paths, ok := raw.([]any)
			if !ok {
				t.Fatalf("%s must be an array: %#v", key, raw)
			}
			for _, path := range paths {
				value, ok := path.(string)
				if !ok || value == "" {
					t.Fatalf("invalid %s path: %#v", key, path)
				}
			}
			overlayCount += len(paths)
		}
	}
	if overlayCount != wantOverlays {
		t.Fatalf("expected %d overlay paths, got %d in %q", wantOverlays, overlayCount, fixture)
	}
	requires, _ := configuration["requires"].(map[string]any)
	if len(requires) != wantRequires {
		t.Fatalf("expected %d artifact requirements, got %#v", wantRequires, requires)
	}
	for name, version := range requires {
		value, ok := version.(string)
		if !ok || name == "" || value == "" || !strings.Contains(name, "/") {
			t.Fatalf("invalid artifact requirement %q: %#v", name, version)
		}
	}
}

func assertVcpkgRegistry(t *testing.T, registry map[string]any, needsPackages bool) {
	t.Helper()
	kind, _ := registry["kind"].(string)
	if kind == "" {
		t.Fatal("registry kind is required")
	}
	if needsPackages && kind != "artifact" {
		packages, ok := registry["packages"].([]any)
		if !ok || len(packages) == 0 {
			t.Fatalf("registry packages are required: %#v", registry)
		}
		for _, raw := range packages {
			pattern, ok := raw.(string)
			if !ok || !isVcpkgPackagePattern(pattern) {
				t.Fatalf("invalid registry package pattern %#v", raw)
			}
		}
	} else if kind == "artifact" {
		if _, hasPackages := registry["packages"]; hasPackages {
			t.Fatalf("artifact registries must not declare port packages: %#v", registry)
		}
	}
	switch kind {
	case "git":
		repository, repositoryOK := registry["repository"].(string)
		baseline, baselineOK := registry["baseline"].(string)
		if !repositoryOK || repository == "" || !baselineOK || !isVcpkgGitBaseline(baseline) {
			t.Fatalf("invalid Git registry: %#v", registry)
		}
		if reference, hasReference := registry["reference"]; hasReference {
			if value, ok := reference.(string); !ok || value == "" {
				t.Fatalf("invalid Git registry reference: %#v", reference)
			}
		}
	case "builtin":
		baseline, ok := registry["baseline"].(string)
		if !ok || !isVcpkgGitBaseline(baseline) {
			t.Fatalf("invalid builtin registry: %#v", registry)
		}
	case "filesystem":
		path, ok := registry["path"].(string)
		if !ok || path == "" {
			t.Fatalf("invalid filesystem registry: %#v", registry)
		}
	case "artifact":
		if registry["location"] == "" || registry["name"] == "" {
			t.Fatalf("invalid artifact registry: %#v", registry)
		}
	default:
		t.Fatalf("unknown registry kind %q", kind)
	}
}

func isVcpkgGitBaseline(value string) bool {
	if len(value) != 40 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func isVcpkgPackagePattern(value string) bool {
	if value == "*" {
		return true
	}
	if strings.Count(value, "*") > 1 || (strings.Contains(value, "*") && !strings.HasSuffix(value, "*")) {
		return false
	}
	value = strings.TrimSuffix(value, "*")
	value = strings.TrimSuffix(value, "-")
	if value == "" {
		return false
	}
	for _, part := range strings.Split(value, "-") {
		if part == "" {
			return false
		}
		for _, char := range part {
			if !(char >= 'a' && char <= 'z') && !(char >= '0' && char <= '9') {
				return false
			}
		}
	}
	return true
}

func assertImportMapFixtureShape(t *testing.T, fixture string, document map[string]any, minImports, minScopes, minIntegrity int) {
	t.Helper()
	for _, key := range []string{"imports", "scopes", "integrity"} {
		raw, ok := document[key]
		if !ok {
			continue
		}
		entries, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("%s must be an object: %#v", key, raw)
		}
		if key == "scopes" {
			if len(entries) < minScopes {
				t.Fatalf("expected at least %d scopes, got %#v", minScopes, entries)
			}
			for scope, values := range entries {
				if !isImportMapURL(scope) {
					t.Fatalf("invalid scope key %q", scope)
				}
				assertImportMapSpecifierMap(t, values)
			}
			continue
		}
		if key == "integrity" {
			if len(entries) < minIntegrity {
				t.Fatalf("expected at least %d integrity mappings, got %#v", minIntegrity, entries)
			}
			for target, value := range entries {
				integrity, ok := value.(string)
				if !ok || !isImportMapURL(target) || !(strings.HasPrefix(integrity, "sha256-") || strings.HasPrefix(integrity, "sha384-") || strings.HasPrefix(integrity, "sha512-")) {
					t.Fatalf("invalid integrity mapping %q: %#v", target, value)
				}
			}
			continue
		}
		if len(entries) < minImports {
			t.Fatalf("expected at least %d imports, got %#v", minImports, entries)
		}
		assertImportMapSpecifierMap(t, entries)
	}
	if document["imports"] == nil && document["scopes"] == nil && document["integrity"] == nil {
		t.Fatalf("import map %q has no module mappings", fixture)
	}
}

func assertImportMapSpecifierMap(t *testing.T, raw any) {
	t.Helper()
	entries, ok := raw.(map[string]any)
	if !ok || len(entries) == 0 {
		t.Fatalf("invalid module specifier map: %#v", raw)
	}
	for specifier, target := range entries {
		if specifier == "" {
			t.Fatal("module specifier must not be empty")
		}
		value, ok := target.(string)
		if !ok || !isImportMapURL(value) {
			t.Fatalf("invalid mapping %q: %#v", specifier, target)
		}
		if strings.HasSuffix(specifier, "/") != strings.HasSuffix(value, "/") {
			t.Fatalf("prefix mapping must preserve trailing slash: %q -> %q", specifier, value)
		}
	}
}

func isImportMapURL(value string) bool {
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs()
}

func hasNonASCIIBinaryByte(contents []byte) bool {
	for _, b := range contents {
		if b < 0x09 || (b > 0x0d && b < 0x20) || b > 0x7e {
			return true
		}
	}
	return false
}

func parseINISection(t *testing.T, contents []byte, section string) map[string]string {
	t.Helper()
	values := map[string]string{}
	inSection := false
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSection = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]") == section
			continue
		}
		if inSection && strings.Contains(line, "=") {
			key, value, _ := strings.Cut(line, "=")
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return values
}

func assertGradleWrapperFixture(t *testing.T, fixture string, contents []byte) {
	t.Helper()
	properties := map[string]string{}
	lines := strings.Split(string(contents), "\n")
	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if strings.HasSuffix(line, "\\") && index+1 < len(lines) {
			index++
			line = strings.TrimSuffix(line, "\\") + strings.TrimSpace(lines[index])
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			key, value, found = strings.Cut(line, ":")
		}
		if !found {
			t.Fatalf("invalid wrapper property %q", line)
		}
		properties[strings.TrimSpace(key)] = strings.ReplaceAll(strings.TrimSpace(value), "\\:", ":")
	}
	url := properties["distributionUrl"]
	if !(strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "file://")) || !strings.HasSuffix(url, ".zip") {
		t.Fatalf("invalid Gradle distribution URL: %#v", properties)
	}
	if checksum := properties["distributionSha256Sum"]; checksum != "" && (len(checksum) != 64 || strings.Trim(checksum, "0123456789abcdefABCDEF") != "") {
		t.Fatalf("invalid Gradle checksum: %q", checksum)
	}
	if strings.HasSuffix(fixture, "gradle-wrapper-network") && (properties["networkTimeout"] != "30000" || properties["retries"] != "3" || properties["retryBackOffMs"] != "1000") {
		t.Fatalf("invalid Gradle network configuration: %#v", properties)
	}
	for _, key := range []string{"validateDistributionUrl"} {
		if value := properties[key]; value != "" && value != "true" && value != "false" {
			t.Fatalf("invalid Gradle boolean %s=%q", key, value)
		}
	}
	for _, key := range []string{"networkTimeout", "retries", "retryBackOffMs"} {
		if value := properties[key]; value != "" && (strings.Trim(value, "0123456789") != "" || strings.HasPrefix(value, "-")) {
			t.Fatalf("invalid non-negative Gradle integer %s=%q", key, value)
		}
	}
	if strings.HasSuffix(fixture, "gradle-wrapper-authenticated") && !strings.Contains(url, "fixture-user:fixture-password@") {
		t.Fatalf("missing authenticated Gradle distribution URL: %#v", properties)
	}
	if strings.HasSuffix(fixture, "gradle-wrapper-escaped") && !strings.Contains(url, "gradle-9.0.0-bin.zip") {
		t.Fatalf("invalid continued Gradle URL: %#v", properties)
	}
}

type ivySettingsElement struct {
	XMLName  xml.Name
	Attr     []xml.Attr           `xml:",any,attr"`
	Children []ivySettingsElement `xml:",any"`
}

func (element ivySettingsElement) has(name string) bool {
	if element.XMLName.Local == name {
		return true
	}
	for _, child := range element.Children {
		if child.has(name) {
			return true
		}
	}
	return false
}

func (element ivySettingsElement) all(name string) []ivySettingsElement {
	result := []ivySettingsElement{}
	if element.XMLName.Local == name {
		result = append(result, element)
	}
	for _, child := range element.Children {
		result = append(result, child.all(name)...)
	}
	return result
}

func (element ivySettingsElement) attribute(name string) string {
	for _, attr := range element.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func assertIvySettingsFixture(t *testing.T, fixture string, contents []byte) {
	t.Helper()
	var settings ivySettingsElement
	if err := xml.Unmarshal(contents, &settings); err != nil || settings.XMLName.Local != "ivysettings" {
		t.Fatalf("invalid Ivy settings XML: %v", err)
	}
	if strings.HasSuffix(fixture, "ivy-settings-basic") && (!settings.has("ibiblio") || !settings.has("caches")) {
		t.Fatalf("invalid basic Ivy settings: %#v", settings)
	}
	if strings.HasSuffix(fixture, "ivy-settings-filesystem") && (!settings.has("filesystem") || !settings.has("ivy") || !settings.has("artifact")) {
		t.Fatalf("invalid filesystem Ivy settings: %#v", settings)
	}
	if strings.HasSuffix(fixture, "ivy-settings-chain") && (!settings.has("chain") || !settings.has("resolver")) {
		t.Fatalf("invalid chained Ivy settings: %#v", settings)
	}
	if strings.HasSuffix(fixture, "ivy-settings-url-credentials") && (!settings.has("credentials") || !settings.has("url")) {
		t.Fatalf("invalid credentialed Ivy settings: %#v", settings)
	}
	if strings.HasSuffix(fixture, "ivy-settings-include") {
		if !settings.has("include") || !strings.Contains(string(contents), "ivysettings-shared.xml") {
			t.Fatalf("invalid included Ivy settings: %#v", settings)
		}
		included, err := os.ReadFile(filepath.Join("..", "..", "testdata", fixture, "ivysettings-shared.xml"))
		var shared ivySettingsElement
		if err != nil || xml.Unmarshal(included, &shared) != nil || shared.XMLName.Local != "ivysettings" || len(shared.all("ibiblio")) != 1 || shared.all("ibiblio")[0].attribute("name") != "shared" {
			t.Fatalf("invalid included Ivy settings file: %v", err)
		}
	}
	if strings.HasSuffix(fixture, "ivy-settings-module-routing") && (!settings.has("modules") || !settings.has("module")) {
		t.Fatalf("invalid Ivy module routing settings: %#v", settings)
	}
	if strings.HasSuffix(fixture, "ivy-settings-dual-sftp") && (!settings.has("dual") || !settings.has("sftp")) {
		t.Fatalf("invalid dual/SFTP Ivy settings: %#v", settings)
	}
	if strings.HasSuffix(fixture, "ivy-settings-properties") && (!settings.has("properties") || !settings.has("property")) {
		t.Fatalf("invalid property-backed Ivy settings: %#v", settings)
	}
	resolverNames := map[string]struct{}{}
	for _, tag := range []string{"ibiblio", "filesystem", "url", "chain", "dual", "sftp"} {
		for _, resolver := range settings.all(tag) {
			name := resolver.attribute("name")
			if name == "" {
				t.Fatalf("Ivy %s resolver lacks a name: %#v", tag, resolver)
			}
			resolverNames[name] = struct{}{}
		}
	}
	for _, setting := range settings.all("settings") {
		if defaultResolver := setting.attribute("defaultResolver"); defaultResolver != "" {
			if _, exists := resolverNames[defaultResolver]; !exists && !strings.HasSuffix(fixture, "ivy-settings-include") {
				t.Fatalf("Ivy default resolver %q is not declared", defaultResolver)
			}
		}
	}
	for _, resolver := range settings.all("resolver") {
		if ref := resolver.attribute("ref"); ref == "" {
			t.Fatalf("Ivy resolver reference has no ref: %#v", resolver)
		} else if _, exists := resolverNames[ref]; !exists {
			t.Fatalf("Ivy resolver reference %q is not declared", ref)
		}
	}
	for _, pattern := range append(settings.all("ivy"), settings.all("artifact")...) {
		if pattern.attribute("pattern") == "" && pattern.attribute("ref") == "" {
			t.Fatalf("Ivy resolver pattern is empty: %#v", pattern)
		}
	}
	for _, credentials := range settings.all("credentials") {
		for _, attribute := range []string{"host", "realm", "username", "passwd"} {
			if credentials.attribute(attribute) == "" {
				t.Fatalf("Ivy credentials lack %s: %#v", attribute, credentials)
			}
		}
	}
	for _, module := range settings.all("module") {
		for _, attribute := range []string{"organisation", "name", "resolver"} {
			if module.attribute(attribute) == "" {
				t.Fatalf("Ivy module rule lacks %s: %#v", attribute, module)
			}
		}
		if _, exists := resolverNames[module.attribute("resolver")]; !exists {
			t.Fatalf("Ivy module resolver %q is not declared", module.attribute("resolver"))
		}
	}
}

func assertNpmrcFixture(t *testing.T, contents []byte) {
	t.Helper()
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if !strings.Contains(line, "=") {
			t.Fatalf("invalid npmrc assignment %q", line)
		}
		key := strings.TrimSpace(strings.SplitN(line, "=", 2)[0])
		if strings.HasPrefix(line, "//") && !strings.Contains(key, ":_") && !strings.Contains(key, ":username") && !strings.Contains(key, ":email") && !strings.Contains(key, ":certfile") && !strings.Contains(key, ":keyfile") {
			t.Fatalf("unscoped npm credential configuration %q", line)
		}
		for _, authKey := range []string{"_auth", "_authToken", "username", "_password", "email", "certfile", "keyfile"} {
			if key == authKey {
				t.Fatalf("unscoped npm credential configuration %q", line)
			}
		}
	}
}

func assertYarnrcFixtureShape(t *testing.T, name string, document map[string]any) {
	t.Helper()
	if strings.Contains(name, "plugins") {
		if _, ok := document["plugins"].([]any); !ok {
			t.Fatal("Yarn plugins must be a list")
		}
	}
	if strings.Contains(name, "registries") {
		for _, key := range []string{"npmScopes", "npmRegistries"} {
			if _, ok := document[key].(map[string]any); !ok {
				t.Fatalf("%s must be a mapping", key)
			}
		}
	}
	if strings.Contains(name, "package-extensions") {
		if _, ok := document["packageExtensions"].(map[string]any); !ok {
			t.Fatal("packageExtensions must be a mapping")
		}
	}
	if strings.Contains(name, "catalogs") {
		for _, key := range []string{"catalog", "catalogs"} {
			if _, ok := document[key].(map[string]any); !ok {
				t.Fatalf("%s must be a mapping", key)
			}
		}
	}
	if strings.Contains(name, "git-policy") || strings.Contains(name, "package-gates") {
		for _, key := range []string{"approvedGitRepositories", "npmPreapprovedPackages"} {
			if value, exists := document[key]; exists {
				if _, ok := value.([]any); !ok {
					t.Fatalf("%s must be a list", key)
				}
			}
		}
	}
	if strings.Contains(name, "platforms") {
		if _, ok := document["supportedArchitectures"].(map[string]any); !ok {
			t.Fatal("supportedArchitectures must be a mapping")
		}
	}
}

func TestCppMesonFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name          string
		fragments     []string
		occurrences   map[string]int
		expectedPaths []string
	}{
		{name: "meson", fragments: []string{`dependency('zlib')`}, occurrences: map[string]int{"dependency(": 1}},
		{name: "meson-dependency-versions", fragments: []string{`dependency('fmt', version : '>=10.0')`, `dependency('boost', version : '>=1.82', modules : ['filesystem', 'program_options'])`, `dependency('qt6', version : '>=6.6', modules : ['Core', 'Gui'])`}, occurrences: map[string]int{"dependency(": 3}},
		{name: "meson-dependency-methods", fragments: []string{`dependency('libpng', method : 'pkg-config', required : true)`, `dependency('OpenSSL', method : 'cmake', modules : ['SSL', 'Crypto'])`, `dependency('threads')`}, occurrences: map[string]int{"dependency(": 3}},
		{name: "meson-optional-static", fragments: []string{`dependency('libunwind', required : false, static : true)`, `dependency('backtrace', required : false, disabler : true)`, `dependency('project-internal', required : false, allow_fallback : false)`}, occurrences: map[string]int{"dependency(": 3}},
		{name: "meson-dependency-fallbacks", fragments: []string{`fallback : ['fmt', 'fmt_dep']`, `default_options : ['default_library=static']`, `dependency('catch2', fallback : 'catch2')`}, occurrences: map[string]int{"dependency(": 2}},
		{name: "meson-subproject-manual", fragments: []string{`subproject(`, `'libsimple'`, `default_options : ['default_library=static']`, `version : '>=1.0'`, `libsimple_project.get_variable('libsimple_dep')`}, occurrences: map[string]int{"subproject(": 1, ".get_variable(": 1}},
		{name: "meson-alternative-names", fragments: []string{`dependency('png', 'libpng', version : ['>=1.6.0', '<1.7.0'], required : true)`, `dependency('openssl', 'OpenSSL', version : ['>=3.0', '<4.0'])`}, occurrences: map[string]int{"dependency(": 2}},
		{name: "meson-cross-provider-controls", fragments: []string{`native : true`, `language : 'c'`, `include_type : 'system'`, `not_found_message : 'pkgconf is optional for this build'`, `allow_fallback : true`, `fallback : ['embedded-library', 'embedded_dep']`}, occurrences: map[string]int{"dependency(": 2}},
		{name: "meson-override-fallback", fragments: []string{`dependency('catch2', fallback : 'catch2')`}, occurrences: map[string]int{"dependency(": 1}, expectedPaths: []string{"meson.build", "subprojects/catch2/meson.build"}},
		{name: "meson-compiler-libraries", fragments: []string{`meson.get_compiler('c')`, `cc.find_library('m', required : true)`, `cc.find_library('dl', required : false)`, `cc.find_library('vendor_codec', dirs : ['/opt/vendor/lib'], required : false, static : true)`, `dependency('iconv', required : false)`}, occurrences: map[string]int{".find_library(": 3, "dependency(": 1}},
		{name: "meson-declare-dependency", fragments: []string{`dependency('zlib')`, `static_library('core', 'core.cpp', dependencies : zlib_dep)`, `declare_dependency(`, `dependencies : [zlib_dep]`, `variables : {'api_level' : '1'}`}, occurrences: map[string]int{" = dependency(": 1, "declare_dependency(": 1}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "cpp", fixture.name, "meson.build"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			for declaration, expectedCount := range fixture.occurrences {
				if count := strings.Count(string(content), declaration); count != expectedCount {
					t.Fatalf("expected %d occurrences of %q, got %d", expectedCount, declaration, count)
				}
			}
			if fixture.name == "meson-override-fallback" {
				provider, err := os.ReadFile(filepath.Join("..", "..", "testdata", "cpp", fixture.name, "subprojects", "catch2", "meson.build"))
				if err != nil {
					t.Fatalf("read fallback provider: %v", err)
				}
				for _, fragment := range []string{`declare_dependency(link_with : catch2_library)`, `meson.override_dependency('catch2', catch2_dep)`} {
					if !strings.Contains(string(provider), fragment) {
						t.Fatalf("expected fallback provider to contain %q", fragment)
					}
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "cpp", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			expectedPaths := fixture.expectedPaths
			if len(expectedPaths) == 0 {
				expectedPaths = []string{"meson.build"}
			}
			if len(result.Sources) != len(expectedPaths) {
				t.Fatalf("expected %d sources, got %+v", len(expectedPaths), result.Sources)
			}
			gotPaths := make([]string, 0, len(result.Sources))
			for _, source := range result.Sources {
				if source.Detector != "cpp-meson" {
					t.Fatalf("expected cpp-meson source, got %+v", source)
				}
				if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
					t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
				}
				gotPaths = append(gotPaths, source.Path)
			}
			slices.Sort(expectedPaths)
			slices.Sort(gotPaths)
			if !slices.Equal(gotPaths, expectedPaths) {
				t.Fatalf("expected paths %v, got %v", expectedPaths, gotPaths)
			}
		})
	}
}

func TestCppAutotoolsFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name          string
		filename      string
		fragments     []string
		occurrences   map[string]int
		expectedPaths []string
		pathFragments map[string]string
	}{
		{name: "autotools", filename: "configure.ac", fragments: []string{`PKG_CHECK_MODULES([ZLIB], [zlib >= 1.2.11])`}, occurrences: map[string]int{"PKG_CHECK_MODULES(": 1}},
		{name: "autotools-pkg-config", filename: "configure.ac", fragments: []string{`PKG_PROG_PKG_CONFIG`, `PKG_CHECK_MODULES([GLIB], [glib-2.0 >= 2.76 gio-2.0])`, `PKG_CHECK_MODULES([JSON], [json-c >= 0.17], [], [AC_MSG_ERROR([json-c is required])])`, `PKG_CHECK_MODULES_STATIC([ARCHIVE], [libarchive >= 3.7])`, `PKG_CHECK_EXISTS([libcurl >= 8.0]`}, occurrences: map[string]int{"PKG_CHECK_MODULES(": 2, "PKG_CHECK_MODULES_STATIC(": 1, "PKG_CHECK_EXISTS(": 1}},
		{name: "autotools-headers-libraries", filename: "configure.ac", fragments: []string{`AC_CHECK_HEADERS([zlib.h openssl/ssl.h sqlite3.h]`, `AC_CHECK_HEADER([curl/curl.h]`, `AC_CHECK_LIB([z], [inflate])`, `AC_CHECK_LIB([ssl], [SSL_new]`, `[-lcrypto]`, `AC_CHECK_FUNC([strlcpy]`}, occurrences: map[string]int{"AC_CHECK_HEADERS(": 1, "AC_CHECK_HEADER(": 1, "AC_CHECK_LIB(": 2, "AC_CHECK_FUNC(": 1}},
		{name: "autotools-search-libraries", filename: "configure.ac", fragments: []string{`AC_SEARCH_LIBS([dlopen], [dl dld]`, `AC_SEARCH_LIBS([gethostbyname], [nsl socket]`, `[-lresolv]`, `AC_CHECK_FUNCS([clock_gettime pthread_create])`}, occurrences: map[string]int{"AC_SEARCH_LIBS(": 2, "AC_CHECK_FUNCS(": 1}},
		{name: "autotools-optional-external", filename: "configure.ac", fragments: []string{`AC_ARG_WITH([readline]`, `AC_CHECK_HEADERS([readline/readline.h])`, `AC_CHECK_LIB([readline], [readline]`, `[-lncurses]`, `AC_ARG_ENABLE([tracing]`, `AC_ARG_WITH([foo]`, `--with-foo=DIR`, `CPPFLAGS="-I$with_foo/include $CPPFLAGS"`, `LDFLAGS="-L$with_foo/lib $LDFLAGS"`, `AC_CHECK_LIB([foo], [foo_init]`}, occurrences: map[string]int{"AC_ARG_WITH(": 2, "AC_CHECK_LIB(": 2, "AC_ARG_ENABLE(": 1}},
		{name: "autotools-subdirectories", filename: "configure.ac", fragments: []string{`AC_CONFIG_SUBDIRS([third_party/libsimple third_party/libextra])`}, occurrences: map[string]int{"AC_CONFIG_SUBDIRS(": 1}, expectedPaths: []string{"configure.ac", "third_party/libextra/configure.ac", "third_party/libsimple/configure.in", "third_party/libsimple/plugins/helper/configure.in"}, pathFragments: map[string]string{"third_party/libextra/configure.ac": `PKG_CHECK_MODULES([LIBEXTRA], [libevent >= 2.1])`, "third_party/libsimple/configure.in": `AC_CHECK_LIB([m], [cos])`, "third_party/libsimple/plugins/helper/configure.in": `AC_CHECK_HEADER([helper/api.h])`}},
		{name: "autotools-automake", filename: "configure.ac", fragments: []string{`AM_INIT_AUTOMAKE([foreign])`, `PKG_CHECK_MODULES([CURL], [libcurl >= 8.0])`, `AC_CONFIG_FILES([Makefile src/Makefile])`}, occurrences: map[string]int{"AM_INIT_AUTOMAKE(": 1, "PKG_CHECK_MODULES(": 1, "AC_CONFIG_FILES(": 1}},
		{name: "autotools-tool-requirements", filename: "configure.in", fragments: []string{`AC_PATH_PROG([DOXYGEN], [doxygen], [no])`, `AC_CHECK_TOOL([AR], [ar])`, `AC_CHECK_PROGS([PROTOC], [protoc protoc-c])`, `PKG_PROG_PKG_CONFIG([0.29])`}, occurrences: map[string]int{"AC_PATH_PROG(": 1, "AC_CHECK_TOOL(": 1, "AC_CHECK_PROGS(": 1, "PKG_PROG_PKG_CONFIG(": 1}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "cpp", fixture.name, fixture.filename))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			for declaration, expectedCount := range fixture.occurrences {
				if count := strings.Count(string(content), declaration); count != expectedCount {
					t.Fatalf("expected %d occurrences of %q, got %d", expectedCount, declaration, count)
				}
			}
			for path, fragment := range fixture.pathFragments {
				child, err := os.ReadFile(filepath.Join("..", "..", "testdata", "cpp", fixture.name, path))
				if err != nil {
					t.Fatalf("read child fixture %s: %v", path, err)
				}
				if !strings.Contains(string(child), fragment) {
					t.Fatalf("expected child fixture %s to contain %q", path, fragment)
				}
			}
			if fixture.name == "autotools-automake" {
				makefile, err := os.ReadFile(filepath.Join("..", "..", "testdata", "cpp", fixture.name, "src", "Makefile.am"))
				if err != nil {
					t.Fatalf("read Automake fixture: %v", err)
				}
				for _, fragment := range []string{`network_client_LDADD = $(CURL_LIBS)`, `AM_CPPFLAGS = $(CURL_CFLAGS)`} {
					if !strings.Contains(string(makefile), fragment) {
						t.Fatalf("expected Automake fixture to contain %q", fragment)
					}
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "cpp", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			expectedPaths := fixture.expectedPaths
			if len(expectedPaths) == 0 {
				expectedPaths = []string{fixture.filename}
			}
			if len(result.Sources) != len(expectedPaths) {
				t.Fatalf("expected %d sources, got %+v", len(expectedPaths), result.Sources)
			}
			gotPaths := make([]string, 0, len(result.Sources))
			for _, source := range result.Sources {
				if source.Detector != "cpp-autotools" {
					t.Fatalf("expected cpp-autotools source, got %+v", source)
				}
				if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
					t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
				}
				gotPaths = append(gotPaths, source.Path)
			}
			slices.Sort(expectedPaths)
			slices.Sort(gotPaths)
			if !slices.Equal(gotPaths, expectedPaths) {
				t.Fatalf("expected paths %v, got %v", expectedPaths, gotPaths)
			}
		})
	}
}

func TestBazelWorkspaceFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name          string
		filename      string
		fragments     []string
		occurrences   map[string]int
		expectedPaths []string
	}{
		{name: "workspace", filename: "WORKSPACE", fragments: []string{`workspace(name = "my_project")`}, occurrences: map[string]int{"workspace(": 1}},
		{name: "workspace-bazel", filename: "WORKSPACE.bazel", fragments: []string{`workspace(name = "my_project")`}, occurrences: map[string]int{"workspace(": 1}},
		{name: "workspace-http-archive", filename: "WORKSPACE", fragments: []string{`load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")`, `name = "bazel_skylib"`, `"https://mirror.example.test/bazel-skylib-1.4.2.tar.gz"`, `sha256 = "66ffd9315665bfaafc96b52278f57c7e2dd09f5ede279ea6d39b2be471e7e3aa"`, `strip_prefix = "bazel-skylib-1.4.2"`}, occurrences: map[string]int{"http_archive(": 1}},
		{name: "workspace-http-build-content", filename: "WORKSPACE.bazel", fragments: []string{`http_archive(`, `build_file_content = "exports_files([\"openssl.h\"])"`, `http_file(`, `downloaded_file_path = "custom-rules.deb"`, `http_jar(`, `name = "ssl_jar"`}, occurrences: map[string]int{"http_archive(": 1, "http_file(": 1, "http_jar(": 1}},
		{name: "workspace-git-repository", filename: "WORKSPACE", fragments: []string{`load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")`, `remote = "https://github.com/bazelbuild/bazel.git"`, `commit = "93f38093f8e24875c1d015e67311853756bdb27e"`, `init_submodules = False`, `sparse_checkout_patterns = ["src", "tools"]`, `build_file = "//third_party:bazel_pinned_git.BUILD"`}, occurrences: map[string]int{"git_repository(": 1}},
		{name: "workspace-local-repositories", filename: "WORKSPACE", fragments: []string{`load("@bazel_tools//tools/build_defs/repo:local.bzl", "local_repository", "new_local_repository")`, `local_repository(`, `repo_mapping = {"@legacy_dep": "@modern_dep"}`, `new_local_repository(`, `build_file_content = "cc_library(name = \"makefile_library\")"`}, occurrences: map[string]int{"local_repository(": 2, "new_local_repository(": 1}, expectedPaths: []string{"WORKSPACE", "third_party/already_bazelized_library/WORKSPACE"}},
		{name: "workspace-dependency-macros", filename: "WORKSPACE.bazel", fragments: []string{`name = "rules_java"`, `rules_java_dependencies", "rules_java_toolchains"`, `rules_java_dependencies()`, `rules_java_toolchains()`}, occurrences: map[string]int{"http_archive(": 1, "rules_java_dependencies(": 1, "rules_java_toolchains(": 1}},
		{name: "workspace-http-integrity", filename: "WORKSPACE", fragments: []string{`integrity = "sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU="`, `canonical_id = "patched-dependency-1.0"`, `patches = ["//third_party:patched-dependency.patch"]`, `patch_strip = 1`}, occurrences: map[string]int{"http_archive(": 1}},
		{name: "workspace-legacy-new-git", filename: "WORKSPACE", fragments: []string{`# Legacy repository rule retained only to cover old WORKSPACE declarations.`, `new_git_repository`, `commit = "93f38093f8e24875c1d015e67311853756bdb27e"`, `build_file_content = "exports_files([\"README.md\"])"`}, occurrences: map[string]int{"new_git_repository(": 1}},
		{name: "workspace-bind-aliases", filename: "WORKSPACE", fragments: []string{`name = "openssl_repo"`, `build_file = "//third_party:openssl.BUILD"`, `# Legacy bind alias retained only for old WORKSPACE declarations.`, `bind(`, `actual = "@openssl_repo//:openssl-lib"`}, occurrences: map[string]int{"http_archive(": 1, "bind(": 1}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bazel", fixture.name, fixture.filename))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			for declaration, expectedCount := range fixture.occurrences {
				if count := strings.Count(string(content), declaration); count != expectedCount {
					t.Fatalf("expected %d occurrences of %q, got %d", expectedCount, declaration, count)
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "bazel", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			expectedPaths := fixture.expectedPaths
			if len(expectedPaths) == 0 {
				expectedPaths = []string{fixture.filename}
			}
			if len(result.Sources) != len(expectedPaths) {
				t.Fatalf("expected %d sources, got %+v", len(expectedPaths), result.Sources)
			}
			gotPaths := make([]string, 0, len(result.Sources))
			for _, source := range result.Sources {
				if source.Detector != "bazel-workspace" {
					t.Fatalf("expected bazel-workspace source, got %+v", source)
				}
				if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
					t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
				}
				gotPaths = append(gotPaths, source.Path)
			}
			slices.Sort(expectedPaths)
			slices.Sort(gotPaths)
			if !slices.Equal(gotPaths, expectedPaths) {
				t.Fatalf("expected paths %v, got %v", expectedPaths, gotPaths)
			}
		})
	}
}

func TestBazelModuleFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name          string
		fragments     []string
		occurrences   map[string]int
		expectedPaths []string
	}{
		{name: "module", fragments: []string{`module(name = "my_project", version = "1.0")`}, occurrences: map[string]int{"module(": 1}},
		{name: "module-dependencies", fragments: []string{`repo_name = "module_deps"`, `bazel_dep(name = "bazel_skylib", version = "1.6.1")`, `repo_name = "cc_rules"`, `dev_dependency = True`, `repo_name = None`}, occurrences: map[string]int{"bazel_dep(": 4}},
		{name: "module-registry-overrides", fragments: []string{`single_version_override(`, `module_name = "rules_cc"`, `registry = "https://registry.example.test"`, `patches = ["//third_party:rules_cc.patch"]`, `multiple_version_override(`, `versions = ["1.5.0"]`}, occurrences: map[string]int{"single_version_override(": 1, "multiple_version_override(": 1}},
		{name: "module-nonregistry-overrides", fragments: []string{`archive_override(`, `urls = ["https://github.com/bazelbuild/rules_java/releases/download/6.1.1/rules_java-6.1.1.tar.gz"]`, `integrity = "sha256-dkAqUK5oWdUL167YwbjvCdrlwQNbs8p9J29/POZZgYo="`, `git_override(`, `remote = "https://github.com/bazelbuild/bazel-skylib.git"`, `tag = "1.6.1"`, `local_path_override(`, `path = "third_party/local_helpers"`}, occurrences: map[string]int{"archive_override(": 1, "git_override(": 1, "local_path_override(": 1}, expectedPaths: []string{"MODULE.bazel", "third_party/local_helpers/MODULE.bazel"}},
		{name: "module-extensions", fragments: []string{`bazel_dep(name = "rules_jvm_external", version = "4.5")`, `maven = use_extension("@rules_jvm_external//:extensions.bzl", "maven")`, `maven.install(artifacts = ["org.junit:junit:4.13.2"])`, `maven.artifact(`, `use_repo(maven, "maven", local_maven = "maven")`, `dev_maven = use_extension("@rules_jvm_external//:extensions.bzl", "maven", dev_dependency = True)`, `use_repo(dev_maven, **{"maven.2": "maven"})`}, occurrences: map[string]int{"use_extension(": 2, "maven.install(": 1, "maven.artifact(": 1, "use_repo(": 2}},
		{name: "module-extension-repo-overrides", fragments: []string{`maven = use_extension("@rules_jvm_external//:extensions.bzl", "maven")`, `local_repository = use_repo_rule("@bazel_tools//tools/build_defs/repo:local.bzl", "local_repository")`, `local_repository(name = "vendored_maven", path = "third_party/vendored_maven")`, `override_repo(maven, maven = "vendored_maven")`, `inject_repo(maven, injected_maven = "vendored_maven")`}, occurrences: map[string]int{"use_extension(": 1, "use_repo_rule(": 1, "local_repository(": 1, "override_repo(": 1, "inject_repo(": 1}, expectedPaths: []string{"MODULE.bazel", "third_party/vendored_maven/MODULE.bazel"}},
		{name: "module-repo-rule", fragments: []string{`use_repo_rule("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")`, `name = "direct_http_archive"`, `dev_dependency = True`}, occurrences: map[string]int{"use_repo_rule(": 1, "http_archive(": 1}},
		{name: "module-includes", fragments: []string{`include("//:dependencies.MODULE.bazel")`}, occurrences: map[string]int{"include(": 1}},
		{name: "module-toolchains", fragments: []string{`register_toolchains("@rules_go//go:toolchain", dev_dependency = True)`, `register_execution_platforms("@rules_go//go:platform")`, `flag_alias(name = "go_debug", starlark_flag = "@rules_go//go/config:debug")`}, occurrences: map[string]int{"register_toolchains(": 1, "register_execution_platforms(": 1, "flag_alias(": 1}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bazel", fixture.name, "MODULE.bazel"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			for declaration, expectedCount := range fixture.occurrences {
				if count := strings.Count(string(content), declaration); count != expectedCount {
					t.Fatalf("expected %d occurrences of %q, got %d", expectedCount, declaration, count)
				}
			}
			if fixture.name == "module-includes" {
				included, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bazel", fixture.name, "dependencies.MODULE.bazel"))
				if err != nil {
					t.Fatalf("read included module declarations: %v", err)
				}
				for _, fragment := range []string{`bazel_dep(name = "rules_go", version = "0.49.0")`, `bazel_dep(name = "gazelle", version = "0.37.0", dev_dependency = True)`} {
					if !strings.Contains(string(included), fragment) {
						t.Fatalf("expected included declarations to contain %q", fragment)
					}
				}
				if count := strings.Count(string(included), "bazel_dep("); count != 2 {
					t.Fatalf("expected 2 included bazel_dep declarations, got %d", count)
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "bazel", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			expectedPaths := fixture.expectedPaths
			if len(expectedPaths) == 0 {
				expectedPaths = []string{"MODULE.bazel"}
			}
			if len(result.Sources) != len(expectedPaths) {
				t.Fatalf("expected %d sources, got %+v", len(expectedPaths), result.Sources)
			}
			gotPaths := make([]string, 0, len(result.Sources))
			for _, source := range result.Sources {
				if source.Detector != "bazel-module" {
					t.Fatalf("expected bazel-module source, got %+v", source)
				}
				if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
					t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
				}
				gotPaths = append(gotPaths, source.Path)
			}
			slices.Sort(expectedPaths)
			slices.Sort(gotPaths)
			if !slices.Equal(gotPaths, expectedPaths) {
				t.Fatalf("expected paths %v, got %v", expectedPaths, gotPaths)
			}
		})
	}
}

func TestBazelModuleLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name              string
		registryFiles     int
		yankedVersions    int
		extensionContexts []string
		repositorySpecs   int
		notFound          int
		malformed         bool
		registrySuffixes  []string
	}{
		{name: "module-lock", registryFiles: 2},
		{name: "module-lock-yanked", registryFiles: 1, yankedVersions: 1},
		{name: "module-lock-not-found", registryFiles: 2, notFound: 1},
		{name: "module-lock-registry-resolution", registryFiles: 4, notFound: 1, registrySuffixes: []string{"bazel_registry.json", "MODULE.bazel", "source.json"}},
		{name: "module-lock-general-extension", extensionContexts: []string{"general"}, repositorySpecs: 1},
		{name: "module-lock-platform-extension", extensionContexts: []string{"os:linux,arch:amd64", "os:windows,arch:amd64"}},
		{name: "module-lock-minimal"},
		{name: "module-lock-empty", malformed: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "bazel", fixture.name, "MODULE.bazel.lock")
			var lock struct {
				Version       int                       `json:"lockFileVersion"`
				RegistryFiles map[string]string         `json:"registryFileHashes"`
				Yanked        map[string]string         `json:"selectedYankedVersions"`
				Extensions    map[string]map[string]any `json:"moduleExtensions"`
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read Bazel lockfile: %v", err)
			}
			if err := json.Unmarshal(contents, &lock); err != nil {
				t.Fatalf("parse Bazel lockfile: %v", err)
			}
			if fixture.malformed {
				if lock.Version != 0 {
					t.Fatalf("expected deliberately malformed lockfile without version, got %#v", lock)
				}
			} else if lock.Version != 10 {
				t.Fatalf("unexpected lockfile version %d", lock.Version)
			}
			if len(lock.RegistryFiles) != fixture.registryFiles {
				t.Fatalf("unexpected registry file count: got %d want %d", len(lock.RegistryFiles), fixture.registryFiles)
			}
			notFound := 0
			for file, hash := range lock.RegistryFiles {
				if !strings.HasPrefix(file, "https://") || (hash != "not found" && (len(hash) != 64 || strings.Trim(hash, "0123456789abcdef") != "")) {
					t.Fatalf("invalid registry file hash %q: %q", file, hash)
				}
				if hash == "not found" {
					notFound++
				}
			}
			if notFound != fixture.notFound {
				t.Fatalf("unexpected not-found registry entries: got %d want %d", notFound, fixture.notFound)
			}
			for _, suffix := range fixture.registrySuffixes {
				found := false
				for file := range lock.RegistryFiles {
					found = found || strings.HasSuffix(file, suffix)
				}
				if !found {
					t.Fatalf("expected registry file ending in %q: %#v", suffix, lock.RegistryFiles)
				}
			}
			if len(lock.Yanked) != fixture.yankedVersions {
				t.Fatalf("unexpected yanked-version count: got %d want %d", len(lock.Yanked), fixture.yankedVersions)
			}
			for moduleVersion, reason := range lock.Yanked {
				if !strings.Contains(moduleVersion, "@") || reason == "" {
					t.Fatalf("invalid yanked module entry %q: %q", moduleVersion, reason)
				}
			}
			var contexts []string
			repositorySpecs := 0
			for extension, platforms := range lock.Extensions {
				if !strings.HasPrefix(extension, "@@") || !strings.Contains(extension, "//") || strings.Count(extension, "%") != 1 {
					t.Fatalf("invalid module extension key %q", extension)
				}
				for platform, entry := range platforms {
					fields, ok := entry.(map[string]any)
					if !ok || !validBazelLockfileContext(platform) {
						t.Fatalf("invalid extension entry %q/%q: %#v", extension, platform, entry)
					}
					for _, digestField := range []string{"bzlTransitiveDigest", "usagesDigest"} {
						digest, ok := fields[digestField].(string)
						decoded, err := base64.StdEncoding.DecodeString(digest)
						if !ok || err != nil || len(decoded) != 32 {
							t.Fatalf("invalid %s in %q/%q: %#v", digestField, extension, platform, fields[digestField])
						}
					}
					for _, inputField := range []string{"recordedFileInputs", "recordedDirentsInputs", "envVariables"} {
						if _, ok := fields[inputField].(map[string]any); !ok {
							t.Fatalf("missing %s in %q/%q: %#v", inputField, extension, platform, fields)
						}
					}
					if _, ok := fields["recordedRepoMappingEntries"].([]any); !ok {
						t.Fatalf("missing recordedRepoMappingEntries in %q/%q: %#v", extension, platform, fields)
					}
					if metadata, exists := fields["moduleExtensionMetadata"]; exists {
						metadataMap, ok := metadata.(map[string]any)
						if !ok {
							t.Fatalf("invalid extension metadata in %q/%q: %#v", extension, platform, metadata)
						}
						if _, ok := metadataMap["reproducible"].(bool); !ok {
							t.Fatalf("extension metadata lacks reproducible boolean: %#v", metadataMap)
						}
						for _, key := range []string{"rootModuleDirectDeps", "rootModuleDirectDevDeps"} {
							if _, ok := metadataMap[key].([]any); !ok {
								t.Fatalf("extension metadata lacks %s array: %#v", key, metadataMap)
							}
						}
					}
					specs, ok := fields["generatedRepoSpecs"].(map[string]any)
					if !ok {
						t.Fatalf("missing generatedRepoSpecs in %q/%q: %#v", extension, platform, fields)
					}
					for name, rawSpec := range specs {
						spec, ok := rawSpec.(map[string]any)
						attributes, hasAttributes := spec["attributes"].(map[string]any)
						if !ok || !hasAttributes || len(attributes) == 0 {
							t.Fatalf("invalid generated repository %q: %#v", name, rawSpec)
						}
						if bzlFile, legacy := spec["bzlFile"].(string); legacy {
							if !strings.HasPrefix(bzlFile, "@@") || spec["ruleClassName"] == "" {
								t.Fatalf("invalid legacy repository spec %q: %#v", name, spec)
							}
						} else if repoRuleID, current := spec["repoRuleId"].(string); !current || repoRuleID == "" {
							t.Fatalf("missing repository-spec discriminator for %q: %#v", name, spec)
						}
						repositorySpecs++
					}
					contexts = append(contexts, platform)
				}
			}
			slices.Sort(contexts)
			expectedContexts := slices.Clone(fixture.extensionContexts)
			slices.Sort(expectedContexts)
			if !slices.Equal(contexts, expectedContexts) {
				t.Fatalf("unexpected extension contexts: got %#v want %#v", contexts, expectedContexts)
			}
			if repositorySpecs != fixture.repositorySpecs {
				t.Fatalf("unexpected generated repository spec count: got %d want %d", repositorySpecs, fixture.repositorySpecs)
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "bazel-module-lock" || result.Sources[0].Path != "MODULE.bazel.lock" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestGopkgTomlFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name        string
		constraints []string
		overrides   []string
		required    []string
		ignored     []string
		noverify    []string
		sources     map[string]string
		versions    map[string]string
		prune       bool
	}{
		{name: "gopkg-toml-versions", constraints: []string{"github.com/pkg/errors", "github.com/spf13/cobra", "golang.org/x/net"}},
		{name: "gopkg-toml-branch-source", constraints: []string{"github.com/acme/forked-library", "github.com/acme/pinned-library"}, sources: map[string]string{"github.com/acme/forked-library": "https://github.com/acme-forks/forked-library.git"}},
		{name: "gopkg-toml-overrides", constraints: []string{"github.com/prometheus/client_golang"}, overrides: []string{"github.com/prometheus/common", "github.com/acme/transitive-fork"}},
		{name: "gopkg-toml-graph-rules", constraints: []string{"github.com/vektra/mockery"}, required: []string{"github.com/golangci/golangci-lint/cmd/golangci-lint", "github.com/vektra/mockery/v2/cmd/mockery"}, ignored: []string{"github.com/acme/application/internal/generated", "github.com/acme/optional-feature*"}, noverify: []string{"github.com/acme/unverified-vendor", "WORKSPACE"}},
		{name: "gopkg-toml-metadata-prune", constraints: []string{"github.com/acme/tooling"}, prune: true},
		{name: "gopkg-toml-source-import-path", constraints: []string{"github.com/acme/direct-library"}, overrides: []string{"github.com/acme/transitive-library"}, sources: map[string]string{"github.com/acme/direct-library": "github.com/acme-forks/direct-library", "github.com/acme/transitive-library": "github.com/acme-forks/transitive-library"}},
		{name: "gopkg-toml-version-expressions", constraints: []string{"github.com/acme/tagged-library", "github.com/acme/wildcard-library", "github.com/acme/hyphen-library", "github.com/acme/excluded-library"}, versions: map[string]string{"github.com/acme/tagged-library": "release-2024-09", "github.com/acme/wildcard-library": "1.2.x", "github.com/acme/hyphen-library": "1.2 - 1.4.5", "github.com/acme/excluded-library": ">=1.2.0, !=1.3.0, <2.0.0"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "go", fixture.name, "Gopkg.toml")
			var manifest struct {
				Metadata    map[string]any `toml:"metadata"`
				Required    []string       `toml:"required"`
				Ignored     []string       `toml:"ignored"`
				Noverify    []string       `toml:"noverify"`
				Constraints []struct {
					Name     string         `toml:"name"`
					Version  string         `toml:"version"`
					Branch   string         `toml:"branch"`
					Revision string         `toml:"revision"`
					Source   string         `toml:"source"`
					Metadata map[string]any `toml:"metadata"`
				} `toml:"constraint"`
				Overrides []struct {
					Name     string         `toml:"name"`
					Version  string         `toml:"version"`
					Branch   string         `toml:"branch"`
					Revision string         `toml:"revision"`
					Source   string         `toml:"source"`
					Metadata map[string]any `toml:"metadata"`
				} `toml:"override"`
				Prune struct {
					UnusedPackages bool `toml:"unused-packages"`
					NonGo          bool `toml:"non-go"`
					GoTests        bool `toml:"go-tests"`
					Project        []struct {
						Name    string `toml:"name"`
						NonGo   bool   `toml:"non-go"`
						GoTests bool   `toml:"go-tests"`
					} `toml:"project"`
				} `toml:"prune"`
			}
			if _, err := toml.DecodeFile(path, &manifest); err != nil {
				t.Fatalf("parse Gopkg.toml: %v", err)
			}
			constraintNames := make([]string, 0, len(manifest.Constraints))
			for _, rule := range manifest.Constraints {
				assertGopkgTomlRule(t, rule.Name, rule.Version, rule.Branch, rule.Revision, rule.Source)
				if want, ok := fixture.sources[rule.Name]; ok && rule.Source != want {
					t.Fatalf("unexpected source for %q: got %q want %q", rule.Name, rule.Source, want)
				}
				if want, ok := fixture.versions[rule.Name]; ok && rule.Version != want {
					t.Fatalf("unexpected version for %q: got %q want %q", rule.Name, rule.Version, want)
				}
				if rule.Name == "github.com/acme/tooling" && rule.Metadata["purpose"] != "code-generation" {
					t.Fatalf("unexpected constraint metadata: %#v", rule.Metadata)
				}
				constraintNames = append(constraintNames, rule.Name)
			}
			if !slices.Equal(constraintNames, fixture.constraints) {
				t.Fatalf("unexpected constraints: got %#v want %#v", constraintNames, fixture.constraints)
			}
			overrideNames := make([]string, 0, len(manifest.Overrides))
			for _, rule := range manifest.Overrides {
				assertGopkgTomlRule(t, rule.Name, rule.Version, rule.Branch, rule.Revision, rule.Source)
				if want, ok := fixture.sources[rule.Name]; ok && rule.Source != want {
					t.Fatalf("unexpected source for %q: got %q want %q", rule.Name, rule.Source, want)
				}
				if rule.Name == "github.com/acme/transitive-fork" && rule.Metadata["reason"] != "temporary-transitive-fix" {
					t.Fatalf("unexpected override metadata: %#v", rule.Metadata)
				}
				overrideNames = append(overrideNames, rule.Name)
			}
			if !slices.Equal(overrideNames, fixture.overrides) {
				t.Fatalf("unexpected overrides: got %#v want %#v", overrideNames, fixture.overrides)
			}
			if !slices.Equal(manifest.Required, fixture.required) || !slices.Equal(manifest.Ignored, fixture.ignored) || !slices.Equal(manifest.Noverify, fixture.noverify) {
				t.Fatalf("unexpected graph rules: required=%#v ignored=%#v noverify=%#v", manifest.Required, manifest.Ignored, manifest.Noverify)
			}
			if fixture.prune && (manifest.Metadata["owner"] != "platform-team" || manifest.Metadata["compliance"] != "reviewed" || !manifest.Prune.UnusedPackages || !manifest.Prune.NonGo || !manifest.Prune.GoTests || len(manifest.Prune.Project) != 1 || manifest.Prune.Project[0].Name != "github.com/acme/generated-assets" || manifest.Prune.Project[0].NonGo || !manifest.Prune.Project[0].GoTests) {
				t.Fatalf("invalid prune configuration: %#v", manifest.Prune)
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "go-gopkg-toml" || result.Sources[0].Path != "Gopkg.toml" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestGodepFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name            string
		importPath      string
		goVersion       string
		godepVersion    string
		hasGodepVersion bool
		imports         []string
		packages        []string
		comments        int
	}{
		{name: "godep", importPath: "github.com/example/app"},
		{name: "godep-basic", importPath: "github.com/acme/service", goVersion: "go1.7.6", godepVersion: "v80", hasGodepVersion: true, imports: []string{"github.com/pkg/errors", "golang.org/x/net/context"}},
		{name: "godep-comments", importPath: "github.com/acme/cli", goVersion: "go1.6.4", imports: []string{"github.com/cloudfoundry/cli/plugin", "github.com/spf13/cobra"}, comments: 2},
		{name: "godep-packages", importPath: "github.com/acme/monorepo", goVersion: "go1.8.7", godepVersion: "v80", hasGodepVersion: true, imports: []string{"github.com/gorilla/mux"}, packages: []string{"./...", "./cmd/server", "./cmd/worker"}},
		{name: "godep-vcs-revisions", importPath: ".", goVersion: "go1.5.4", imports: []string{"code.google.com/p/go-netrc/netrc", "github.com/gorilla/mux"}},
		{name: "godep-empty-metadata", importPath: "github.com/acme/no-external-deps", goVersion: "go1.9.7", godepVersion: "v80", hasGodepVersion: true, packages: []string{"."}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "go", fixture.name, "Godeps", "Godeps.json")
			var manifest struct {
				ImportPath   string   `json:"ImportPath"`
				GoVersion    string   `json:"GoVersion"`
				GodepVersion string   `json:"GodepVersion"`
				Packages     []string `json:"Packages"`
				Deps         []struct {
					ImportPath string `json:"ImportPath"`
					Comment    string `json:"Comment"`
					Rev        string `json:"Rev"`
				} `json:"Deps"`
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read Godeps manifest: %v", err)
			}
			if err := json.Unmarshal(contents, &manifest); err != nil {
				t.Fatalf("parse Godeps manifest: %v", err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(contents, &raw); err != nil {
				t.Fatalf("parse raw Godeps manifest: %v", err)
			}
			if manifest.ImportPath != fixture.importPath || manifest.GoVersion != fixture.goVersion || manifest.GodepVersion != fixture.godepVersion {
				t.Fatalf("unexpected Godeps metadata: %#v", manifest)
			}
			if _, exists := raw["GodepVersion"]; exists != fixture.hasGodepVersion {
				t.Fatalf("unexpected GodepVersion presence: %t", exists)
			}
			if _, exists := raw["Deps"]; !exists {
				t.Fatal("generated Godeps manifest must explicitly include Deps")
			}
			var imports []string
			comments := 0
			for _, dependency := range manifest.Deps {
				if dependency.ImportPath == "" || dependency.Rev == "" || strings.ContainsAny(dependency.Rev, " \t\n") {
					t.Fatalf("invalid Godeps dependency: %#v", dependency)
				}
				if dependency.Comment != "" {
					comments++
				}
				imports = append(imports, dependency.ImportPath)
			}
			if !slices.Equal(imports, fixture.imports) || !slices.Equal(manifest.Packages, fixture.packages) || comments != fixture.comments {
				t.Fatalf("unexpected Godeps content: imports=%#v packages=%#v comments=%d", imports, manifest.Packages, comments)
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "go", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "go-godep" || result.Sources[0].Path != "Godeps/Godeps.json" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

type packagesLockEntry struct {
	Type         string            `json:"type"`
	Requested    string            `json:"requested"`
	Resolved     string            `json:"resolved"`
	ContentHash  string            `json:"contentHash"`
	Dependencies map[string]string `json:"dependencies"`
}

func TestPackagesLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name            string
		version         int
		frameworks      []string
		packages        map[string][]string
		directNodes     int
		transitiveNodes int
		projectNodes    int
		centralNodes    int
		emptyGraph      bool
	}{
		{name: "packages-lock-with-deps", version: 1, frameworks: []string{".NETFramework,Version=v4.8"}, packages: map[string][]string{".NETFramework,Version=v4.8": {"Newtonsoft.Json"}}, directNodes: 1},
		{name: "packages-lock-direct-transitive", version: 1, frameworks: []string{"net7.0"}, packages: map[string][]string{"net7.0": {"NETStandard.Library", "Microsoft.NETCore.Platforms"}}, directNodes: 1, transitiveNodes: 1},
		{name: "packages-lock-multitarget", version: 1, frameworks: []string{".NETFramework,Version=v4.8", "net8.0"}, packages: map[string][]string{".NETFramework,Version=v4.8": {"Newtonsoft.Json"}, "net8.0": {"Newtonsoft.Json"}}, directNodes: 2},
		{name: "packages-lock-project-references", version: 1, frameworks: []string{".NETCoreApp,Version=v5.0"}, packages: map[string][]string{".NETCoreApp,Version=v5.0": {"Web.Host", "Core.Library", "Leaf.Library", "Newtonsoft.Json"}}, projectNodes: 3, transitiveNodes: 1},
		{name: "packages-lock-central-transitive", version: 2, frameworks: []string{"net8.0"}, packages: map[string][]string{"net8.0": {"System.Collections.Immutable"}}, centralNodes: 1},
		{name: "packages-lock-empty-dependencies", version: 1, emptyGraph: true},
		{name: "packages-lock-direct-no-requested", version: 1, frameworks: []string{"netstandard2.0"}, packages: map[string][]string{"netstandard2.0": {"NETStandard.Library"}}, directNodes: 1},
		{name: "packages-lock-no-deps", version: 1, emptyGraph: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "dotnet", fixture.name, "packages.lock.json")
			var lock struct {
				Version      int                                     `json:"version"`
				Dependencies map[string]map[string]packagesLockEntry `json:"dependencies"`
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read NuGet lockfile: %v", err)
			}
			if err := json.Unmarshal(contents, &lock); err != nil {
				t.Fatalf("parse NuGet lockfile: %v", err)
			}
			if lock.Version != fixture.version || len(lock.Dependencies) != len(fixture.frameworks) {
				t.Fatalf("unexpected NuGet lock metadata: %#v", lock)
			}
			frameworks := make([]string, 0, len(lock.Dependencies))
			directNodes, transitiveNodes, projectNodes, centralNodes := 0, 0, 0, 0
			for framework, entries := range lock.Dependencies {
				frameworks = append(frameworks, framework)
				packageNames := make([]string, 0, len(entries))
				for name, entry := range entries {
					if name == "" || (entry.Type != "Direct" && entry.Type != "Transitive" && entry.Type != "Project" && entry.Type != "CentralTransitive") {
						t.Fatalf("invalid NuGet lock entry %q: %#v", name, entry)
					}
					if (entry.Type == "Direct" || entry.Type == "CentralTransitive") && entry.Requested == "" {
						t.Fatalf("direct NuGet lock entry %q has no requested range: %#v", name, entry)
					}
					if entry.Type == "Transitive" && entry.Requested != "" {
						t.Fatalf("transitive NuGet lock entry %q unexpectedly has requested range: %#v", name, entry)
					}
					switch entry.Type {
					case "Direct":
						directNodes++
					case "Transitive":
						transitiveNodes++
					case "Project":
						projectNodes++
					}
					if entry.Type == "Project" {
						if entry.Resolved != "" || entry.ContentHash != "" {
							t.Fatalf("invalid project lock entry %q: %#v", name, entry)
						}
					} else {
						if entry.Resolved == "" || entry.ContentHash == "" {
							t.Fatalf("package lock entry %q has no resolution or integrity hash: %#v", name, entry)
						}
						decoded, err := base64.StdEncoding.DecodeString(entry.ContentHash)
						if err != nil || len(decoded) != 64 {
							t.Fatalf("invalid SHA-512 content hash for %q: %q", name, entry.ContentHash)
						}
					}
					if entry.Type == "CentralTransitive" {
						centralNodes++
					}
					for dependency, version := range entry.Dependencies {
						if dependency == "" || version == "" || !containsPackageFold(entries, dependency) {
							t.Fatalf("lock entry %q has unresolved dependency %q=%q", name, dependency, version)
						}
					}
					packageNames = append(packageNames, name)
				}
				slices.Sort(packageNames)
				want := append([]string(nil), fixture.packages[framework]...)
				slices.Sort(want)
				if !slices.Equal(packageNames, want) {
					t.Fatalf("unexpected packages for %q: got %#v want %#v", framework, packageNames, want)
				}
			}
			slices.Sort(frameworks)
			wantFrameworks := append([]string(nil), fixture.frameworks...)
			slices.Sort(wantFrameworks)
			if !slices.Equal(frameworks, wantFrameworks) || directNodes != fixture.directNodes || transitiveNodes != fixture.transitiveNodes || projectNodes != fixture.projectNodes || centralNodes != fixture.centralNodes {
				t.Fatalf("unexpected NuGet lock graph: frameworks=%#v direct=%d transitive=%d projects=%d central=%d", frameworks, directNodes, transitiveNodes, projectNodes, centralNodes)
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			presence := PresencePresent
			if fixture.emptyGraph {
				presence = PresenceAbsent
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "dotnet-packages-lock" || result.Sources[0].Path != "packages.lock.json" || result.Sources[0].Analysis != (SourceAnalysis{Presence: presence, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func containsPackageFold(entries map[string]packagesLockEntry, name string) bool {
	for candidate := range entries {
		if strings.EqualFold(candidate, name) {
			return true
		}
	}
	return false
}

func TestPaketDependenciesFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name                                    string
		nuget, clitool, github, gist, http, git int
		groups                                  []string
		externalLock                            bool
		requiredLines                           []string
		comments                                int
	}{
		{name: "paket-dependencies-basic", nuget: 3},
		{name: "paket-dependencies-sources", nuget: 2, requiredLines: []string{"source .", "source C:\\nugets", "source ~/project/nugets", "source \\\\server\\share", "source https://packages.example.invalid/nuget/v3/index.json username: \"%PRIVATE_FEED_USER%\" password: \"%PRIVATE_FEED_PASS%\" authtype: \"basic\""}},
		{name: "paket-dependencies-file-sources", github: 4, gist: 2, http: 2, git: 3, requiredLines: []string{"github fsprojects/Paket:main src/Paket.Core/DependenciesFile.fs", "github fsprojects/Paket:9.0.0", "github fsharp/private src/myprivate/file.fs githubAuthKey", "gist Thorium/6088882", "http file:///c:/artifacts/local.dll lib/local.dll", "git git@github.com:fsharp/FAKE.git", "git file:///c:/repos/AskMe 97ee5ae7074b"}},
		{name: "paket-dependencies-groups", nuget: 5, groups: []string{"Build", "Tests"}},
		{name: "paket-dependencies-options", nuget: 2, comments: 1, requiredLines: []string{"version 8.0.0 --prefer-nuget", "references: strict", "framework: auto-detect", "generate_load_scripts: true", "simplify: never", "nuget Serilog >= 3.1 redirects: off import_targets: false", "nuget System.Text.Json 8.* copy_local: false content: none copy_content_to_output_dir: never"}},
		{name: "paket-dependencies-constraints", nuget: 9, clitool: 1, requiredLines: []string{"nuget Unconstrained.Package", "nuget Exact.Implicit 1.2.3", "nuget Exact.Explicit = 1.2.3", "nuget Conflict.Override == 2.0.0", "nuget Compound.Range >= 1.2.3 < 1.5", "nuget Pessimistic.Range ~> 4.2.1", "nuget Preview.Channel >= 3 beta rc", "nuget Strategy.Package @>= 2.0", "nuget Override.Strategy !>= 1.0", "clitool dotnetsay 2.1.7"}},
		{name: "paket-dependencies-external-lock", nuget: 1, externalLock: true, requiredLines: []string{"external_lock https://example.invalid/azure-functions.paket.lock"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "dotnet", fixture.name, "paket.dependencies")
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read Paket dependencies: %v", err)
			}
			var nuget, clitool, github, gist, http, git, comments int
			var groups []string
			externalLock := false
			for _, rawLine := range strings.Split(string(contents), "\n") {
				line := strings.TrimSpace(rawLine)
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
					comments++
					continue
				}
				fields := strings.Fields(line)
				switch fields[0] {
				case "nuget":
					nuget++
				case "clitool":
					clitool++
				case "github":
					github++
				case "gist":
					gist++
				case "http":
					http++
				case "git":
					git++
				case "group":
					if len(fields) != 2 {
						t.Fatalf("invalid Paket group declaration %q", line)
					}
					groups = append(groups, fields[1])
				case "external_lock":
					externalLock = true
				}
			}
			for _, requiredLine := range fixture.requiredLines {
				if !containsTrimmedLine(string(contents), requiredLine) {
					t.Fatalf("Paket fixture is missing required declaration %q", requiredLine)
				}
			}
			if nuget != fixture.nuget || clitool != fixture.clitool || github != fixture.github || gist != fixture.gist || http != fixture.http || git != fixture.git || !slices.Equal(groups, fixture.groups) || externalLock != fixture.externalLock || comments != fixture.comments {
				t.Fatalf("unexpected Paket dependency declarations: nuget=%d clitool=%d github=%d gist=%d http=%d git=%d groups=%#v externalLock=%t comments=%d", nuget, clitool, github, gist, http, git, groups, externalLock, comments)
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "dotnet-paket-dependencies" || result.Sources[0].Path != "paket.dependencies" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestPaketLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name          string
		sections      []string
		requiredLines []string
		commits       int
	}{
		{name: "paket-lock-basic", sections: []string{"NUGET"}, requiredLines: []string{"remote: https://api.nuget.org/v3/index.json", "Newtonsoft.Json (13.0.3)", "NUnit (3.14.0)"}},
		{name: "paket-lock-transitive-framework", sections: []string{"NUGET"}, requiredLines: []string{"Microsoft.Bcl.Async (1.0.168) - >= net40 < net45", "Microsoft.Bcl (>= 1.1.8)", "NUnit (3.0.0-alpha-4)"}},
		{name: "paket-lock-groups", sections: []string{"NUGET", "GROUP Build", "NUGET", "GROUP Legacy", "NUGET"}, requiredLines: []string{"FAKE (6.0.0)", "FSharp.Compiler.Service (>= 43.7)", "FSharp.Compiler.Service (43.7.400)", "Newtonsoft.Json (6.0.8)"}},
		{name: "paket-lock-github-gist", sections: []string{"NUGET", "GITHUB", "GIST"}, requiredLines: []string{"Octokit (0.4.1)", "remote: forki/FsUnit", "FsUnit.fs (7623fc13439f0e60bd05c1ed3b5f6dcb937fe468)", "remote: Thorium/6088882", "FULLPROJECT"}, commits: 3},
		{name: "paket-lock-http", sections: []string{"HTTP"}, requiredLines: []string{"remote: http://www.fssnip.net/raw/1M/test1.fs", "test1.fs", "docs/Paket.md", "remote: file:///c:/artifacts/local.dll", "tools.zip", "path: tools"}},
		{name: "paket-lock-git", sections: []string{"NUGET", "GIT"}, requiredLines: []string{"remote: paket-files/github.com/forki/nupkgtest/bin", "Argu (1.1.3)", "path: /bin/", "remote: file:///c:/repos/AskMe"}, commits: 2},
		{name: "paket-lock-restrictions", sections: []string{"GROUP Runtime", "NUGET", "GROUP Tools", "NUGET"}, requiredLines: []string{"STRATEGY: MIN", "RESTRICTION: || (== net8.0)", "Microsoft.Extensions.Logging (8.0.0) - restriction: || (== net8.0)", "STRATEGY: MAX", "RESTRICTION: || (>= net6.0)"}},
		{name: "paket-lock-sources", sections: []string{"NUGET"}, requiredLines: []string{"remote: https://packages.example.invalid/nuget/v3/index.json username: \"%PRIVATE_FEED_USER%\" password: \"%PRIVATE_FEED_PASS%\" authtype: \"ntlm\"", "remote: C:\\nugets", "remote: \\\\server\\share"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "dotnet", fixture.name, "paket.lock")
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read Paket lock: %v", err)
			}
			var sections []string
			commits := 0
			for _, rawLine := range strings.Split(string(contents), "\n") {
				line := strings.TrimSpace(rawLine)
				if line == "NUGET" || line == "GITHUB" || line == "GIST" || line == "HTTP" || line == "GIT" || strings.HasPrefix(line, "GROUP ") {
					sections = append(sections, line)
				}
				for _, field := range strings.Fields(line) {
					commit := strings.Trim(field, "()")
					if strings.HasPrefix(field, "(") && strings.HasSuffix(field, ")") && len(commit) == 40 {
						if strings.Trim(commit, "0123456789abcdef") != "" {
							t.Fatalf("invalid locked Git revision %q", commit)
						}
						commits++
					}
				}
			}
			for _, requiredLine := range fixture.requiredLines {
				if !containsTrimmedLine(string(contents), requiredLine) {
					t.Fatalf("Paket lock fixture is missing %q", requiredLine)
				}
			}
			assertPaketLockStructure(t, string(contents))
			if !slices.Equal(sections, fixture.sections) || commits != fixture.commits {
				t.Fatalf("unexpected Paket lock structure: sections=%#v commits=%d", sections, commits)
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "dotnet-paket-lock" || result.Sources[0].Path != "paket.lock" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestPaketReferencesFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name           string
		packages       []string
		groups         []string
		childLines     []string
		childOwners    map[string]string
		fileReferences []string
		comments       int
	}{
		{name: "paket-references", packages: []string{"Newtonsoft.Json", "FSharp.Core"}},
		{name: "paket-references-groups", packages: []string{"Newtonsoft.Json", "UnionArgParser", "DotNetZip", "NUnit", "NUnit.Runners", "FAKE"}, groups: []string{"Test", "Build"}, childLines: []string{"NUnit copy_local: false"}},
		{name: "paket-references-overrides", packages: []string{"Newtonsoft.Json", "Microsoft.Bcl.Build", "Fody", "DotNetZip", "FSharp.Core", "Suave", "Interop.Library"}, childLines: []string{"Newtonsoft.Json copy_local: false specific_version: false", "Microsoft.Bcl.Build import_targets: false", "Fody content: once", "DotNetZip framework: >= net45", "FSharp.Core redirects: on", "Suave license_download: true", "Interop.Library embed_interop_types: true"}},
		{name: "paket-references-exclusions-aliases", packages: []string{"PackageA", "Dapper", "NUnit"}, childLines: []string{"exclude A1.dll", "exclude A2.dll", "alias A1.dll Name2,Name3", "alias A2.dll MyAlias1", "exclude nunit.framework.dll"}, childOwners: map[string]string{"exclude A1.dll": "PackageA", "exclude A2.dll": "PackageA", "alias A1.dll Name2,Name3": "PackageA", "alias A2.dll MyAlias1": "PackageA", "exclude nunit.framework.dll": "NUnit"}},
		{name: "paket-references-files", packages: []string{"Newtonsoft.Json"}, fileReferences: []string{"File: FsUnit.fs", "File: src/Helpers.fs Helpers", "File: docs/Paket.md .", "File: Generated.fs Generated link: false"}},
		{name: "paket-references-comments", packages: []string{"Serilog", "Serilog.Sinks.Console"}, comments: 2},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "dotnet", fixture.name, "paket.references")
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read Paket references: %v", err)
			}
			var packages, groups, fileReferences []string
			comments := 0
			inGroup := false
			currentPackage := ""
			for _, rawLine := range strings.Split(string(contents), "\n") {
				line := strings.TrimSpace(rawLine)
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
					comments++
					continue
				}
				if strings.HasPrefix(line, "group ") {
					groups = append(groups, strings.TrimPrefix(line, "group "))
					inGroup = true
					currentPackage = ""
					continue
				}
				if strings.HasPrefix(line, "File:") {
					fileReferences = append(fileReferences, line)
					continue
				}
				if len(rawLine) == len(strings.TrimLeft(rawLine, " \t")) || (inGroup && !strings.HasPrefix(line, "exclude ") && !strings.HasPrefix(line, "alias ")) {
					currentPackage = strings.Fields(line)[0]
					packages = append(packages, currentPackage)
				} else if !slices.Contains(fixture.childLines, line) {
					t.Fatalf("unexpected Paket reference child line %q", line)
				} else if owner := fixture.childOwners[line]; owner != "" && currentPackage != owner {
					t.Fatalf("Paket reference child line %q attached to %q, want %q", line, currentPackage, owner)
				}
			}
			for _, childLine := range fixture.childLines {
				if !containsTrimmedLine(string(contents), childLine) {
					t.Fatalf("Paket reference fixture is missing %q", childLine)
				}
			}
			if !slices.Equal(packages, fixture.packages) || !slices.Equal(groups, fixture.groups) || !slices.Equal(fileReferences, fixture.fileReferences) || comments != fixture.comments {
				t.Fatalf("unexpected Paket references: packages=%#v groups=%#v files=%#v comments=%d", packages, groups, fileReferences, comments)
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "dotnet-paket-references" || result.Sources[0].Path != "paket.references" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestPaketProjectSpecificReferencesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "dotnet", "paket-references-project-specific")
	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan fixture: %v", err)
	}
	if len(result.Sources) != 3 {
		t.Fatalf("expected shared and project-specific Paket references, got %+v", result.Sources)
	}
	paths := make([]string, 0, len(result.Sources))
	for _, source := range result.Sources {
		if source.Detector != "dotnet-paket-references" || source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
			t.Fatalf("unexpected source: %+v", source)
		}
		paths = append(paths, source.Path)
	}
	slices.Sort(paths)
	if !slices.Equal(paths, []string{"Empty.csproj.paket.references", "Example.vbproj.paket.references", "paket.references"}) {
		t.Fatalf("unexpected project-specific Paket reference paths: %#v", paths)
	}
}

func TestDotnetToolsManifestFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name         string
		path         string
		isRoot       bool
		tools        []string
		rollForwards int
	}{
		{name: "tools-manifest", isRoot: true},
		{name: "tools-manifest-basic", isRoot: true, tools: []string{"dotnet-ef"}, rollForwards: 1},
		{name: "tools-manifest-multiple", isRoot: true, tools: []string{"docfx", "dotnet-coverage", "powershell"}, rollForwards: 2},
		{name: "tools-manifest-roll-forward", tools: []string{"incrementalist.cmd", "reportgenerator"}, rollForwards: 2},
		{name: "tools-manifest-prerelease", isRoot: true, tools: []string{"csharpier"}},
		{name: "tools-manifest-empty", isRoot: true},
		{name: "tools-manifest-root", path: "dotnet-tools.json", isRoot: true, tools: []string{"paket"}, rollForwards: 1},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			relativePath := fixture.path
			if relativePath == "" {
				relativePath = ".config/dotnet-tools.json"
			}
			path := filepath.Join("..", "..", "testdata", "dotnet", fixture.name, relativePath)
			var manifest struct {
				Version int  `json:"version"`
				IsRoot  bool `json:"isRoot"`
				Tools   map[string]struct {
					Version     string   `json:"version"`
					Commands    []string `json:"commands"`
					RollForward *bool    `json:"rollForward"`
				} `json:"tools"`
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read tool manifest: %v", err)
			}
			if err := json.Unmarshal(contents, &manifest); err != nil {
				t.Fatalf("parse tool manifest: %v", err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(contents, &raw); err != nil {
				t.Fatalf("parse raw tool manifest: %v", err)
			}
			for _, requiredKey := range []string{"version", "isRoot", "tools"} {
				if _, exists := raw[requiredKey]; !exists {
					t.Fatalf("tool manifest is missing required %q key", requiredKey)
				}
			}
			if manifest.Version != 1 || manifest.IsRoot != fixture.isRoot {
				t.Fatalf("unexpected tool manifest metadata: %#v", manifest)
			}
			toolNames := make([]string, 0, len(manifest.Tools))
			rollForwards := 0
			for name, tool := range manifest.Tools {
				if name == "" || tool.Version == "" || len(tool.Commands) == 0 {
					t.Fatalf("invalid tool manifest entry %q: %#v", name, tool)
				}
				for _, command := range tool.Commands {
					if command == "" || strings.ContainsAny(command, " \t\n") {
						t.Fatalf("invalid tool command %q for %q", command, name)
					}
				}
				if tool.RollForward != nil {
					rollForwards++
				}
				toolNames = append(toolNames, name)
			}
			slices.Sort(toolNames)
			if !slices.Equal(toolNames, fixture.tools) || rollForwards != fixture.rollForwards {
				t.Fatalf("unexpected tool manifest tools: names=%#v rollForward=%d", toolNames, rollForwards)
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "dotnet", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "dotnet-tools-manifest" || result.Sources[0].Path != relativePath || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestDirectoryBuildFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name     string
		filename string
		packages []string
		sources  int
	}{
		{name: "directory-build-props", filename: "Directory.Build.props"},
		{name: "directory-build-targets", filename: "Directory.Build.targets"},
		{name: "directory-build-props-package-reference", filename: "Directory.Build.props", packages: []string{"StyleCop.Analyzers", "Microsoft.SourceLink.GitHub"}},
		{name: "directory-build-props-conditional", filename: "Directory.Build.props", packages: []string{"Microsoft.Extensions.Hosting", "Microsoft.Windows.Compatibility"}},
		{name: "directory-build-props-metadata", filename: "Directory.Build.props", packages: []string{"Nerdbank.GitVersioning"}},
		{name: "directory-build-targets-package-reference", filename: "Directory.Build.targets", packages: []string{"Microsoft.NET.Test.Sdk"}},
		{name: "directory-build-targets-update", filename: "Directory.Build.targets", packages: []string{"Newtonsoft.Json", "Microsoft.CodeAnalysis.NetAnalyzers"}},
		{name: "directory-build-targets-multitarget", filename: "Directory.Build.targets", packages: []string{"System.Text.Json", "System.Diagnostics.DiagnosticSource"}},
		{name: "directory-build-props-parent-import", filename: "Directory.Build.props", packages: []string{"System.Text.Json"}, sources: 2},
		{name: "directory-build-targets-remove", filename: "Directory.Build.targets", packages: []string{"Existing.Build.Package"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "dotnet", fixture.name, fixture.filename)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read Directory.Build file: %v", err)
			}
			decoder := xml.NewDecoder(strings.NewReader(string(contents)))
			var packages []string
			for {
				token, err := decoder.Token()
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Fatalf("parse Directory.Build XML: %v", err)
				}
				start, ok := token.(xml.StartElement)
				if !ok || start.Name.Local != "PackageReference" {
					continue
				}
				name := ""
				removed := false
				for _, attribute := range start.Attr {
					if attribute.Name.Local == "Include" || attribute.Name.Local == "Update" {
						name = attribute.Value
					}
					if attribute.Name.Local == "Remove" {
						removed = true
					}
				}
				if removed {
					continue
				}
				if name == "" {
					t.Fatalf("PackageReference without Include or Update: %#v", start)
				}
				packages = append(packages, name)
			}
			if !slices.Equal(packages, fixture.packages) {
				t.Fatalf("unexpected Directory.Build package references: got %#v want %#v", packages, fixture.packages)
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			wantSources := fixture.sources
			if wantSources == 0 {
				wantSources = 1
			}
			if len(result.Sources) != wantSources || result.Sources[0].Detector != "dotnet-directory-build" || result.Sources[0].Path != fixture.filename || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestGradleSettingsFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name     string
		filename string
		detector DetectorID
		required []string
		sources  int
	}{
		{name: "gradle-settings-basic", filename: "settings.gradle", detector: "java-gradle-settings", required: []string{"id 'com.gradleup.shadow' version '8.3.4'", "include 'api', 'service:worker'"}},
		{name: "gradle-settings-catalog", filename: "settings.gradle", detector: "java-gradle-settings", required: []string{"RepositoriesMode.FAIL_ON_PROJECT_REPOS", "from('com.example:build-catalog:1.2.3')", "from(files('gradle/test.versions.toml'))", "library('groovy-json', 'org.codehaus.groovy', 'groovy-json').versionRef('groovy')", "plugin('versions', 'com.github.ben-manes.versions').version('0.52.0')"}},
		{name: "gradle-settings-composite", filename: "settings.gradle", detector: "java-gradle-settings", required: []string{"includeBuild('build-logic')", "substitute module('com.example:conventions') using project(':')", "includeBuild('../shared-platform') { name = 'shared-platform' }"}},
		{name: "gradle-settings-plugin-resolution", filename: "settings.gradle", detector: "java-gradle-settings", required: []string{"useModule('com.example:internal-gradle-plugin:1.4.0')", "useVersion('3.2.1')"}},
		{name: "gradle-settings-repositories", filename: "settings.gradle", detector: "java-gradle-settings", required: []string{"includeGroupByRegex('com\\\\.example(\\\\..*)?')", "metadataSources { artifact() }"}},
		{name: "gradle-settings-settings-plugin", filename: "settings.gradle", detector: "java-gradle-settings", required: []string{"id 'com.gradle.develocity' version '3.19.2'"}},
		{name: "gradle-settings-dynamic-plugin", filename: "settings.gradle", detector: "java-gradle-settings", required: []string{"id 'com.example.hello' version \"${helloPluginVersion}\""}},
		{name: "gradle-settings-source-control", filename: "settings.gradle", detector: "java-gradle-settings", required: []string{"gitRepository(uri('https://github.com/example/utilities.git'))", "producesModule('com.example:utilities')", "producesModule('com.example:utilities-test-fixtures')"}},
		{name: "gradle-settings-buildscript", filename: "settings.gradle", detector: "java-gradle-settings", required: []string{"classpath 'com.example:legacy-settings-plugin:2.5.0'"}},
		{name: "gradle-settings-minimal", filename: "build-logic/settings.gradle", detector: "java-gradle-settings", required: []string{"rootProject.name = 'build-logic'"}},
		{name: "gradle-settings-kts-basic", filename: "settings.gradle.kts", detector: "java-gradle-settings-kts", required: []string{"id(\"com.gradleup.shadow\") version \"8.3.4\"", "include(\"api\", \"service:worker\")"}},
		{name: "gradle-settings-kts-catalog", filename: "settings.gradle.kts", detector: "java-gradle-settings-kts", required: []string{"RepositoriesMode.FAIL_ON_PROJECT_REPOS", "from(\"com.example:build-catalog:1.2.3\")", "from(files(\"gradle/test.versions.toml\"))", "library(\"groovy-json\", \"org.codehaus.groovy\", \"groovy-json\").versionRef(\"groovy\")", "plugin(\"versions\", \"com.github.ben-manes.versions\").version(\"0.52.0\")"}},
		{name: "gradle-settings-kts-composite", filename: "settings.gradle.kts", detector: "java-gradle-settings-kts", required: []string{"includeBuild(\"build-logic\")", "substitute(module(\"com.example:conventions\")).using(project(\":\"))", "includeBuild(\"../shared-platform\") { name = \"shared-platform\" }"}},
		{name: "gradle-settings-kts-plugin-resolution", filename: "settings.gradle.kts", detector: "java-gradle-settings-kts", required: []string{"useModule(\"com.example:internal-gradle-plugin:1.4.0\")", "useVersion(\"3.2.1\")"}},
		{name: "gradle-settings-kts-repositories", filename: "settings.gradle.kts", detector: "java-gradle-settings-kts", required: []string{"includeGroupByRegex(\"com\\\\.example(\\\\..*)?\")", "metadataSources { artifact() }"}},
		{name: "gradle-settings-kts-settings-plugin", filename: "settings.gradle.kts", detector: "java-gradle-settings-kts", required: []string{"id(\"com.gradle.develocity\") version \"3.19.2\""}},
		{name: "gradle-settings-kts-dynamic-plugin", filename: "settings.gradle.kts", detector: "java-gradle-settings-kts", required: []string{"id(\"com.example.hello\") version providers.gradleProperty(\"helloPluginVersion\").get()"}},
		{name: "gradle-settings-kts-source-control", filename: "settings.gradle.kts", detector: "java-gradle-settings-kts", required: []string{"gitRepository(uri(\"https://github.com/example/utilities.git\"))", "producesModule(\"com.example:utilities\")", "producesModule(\"com.example:utilities-test-fixtures\")"}},
		{name: "gradle-settings-kts-buildscript", filename: "settings.gradle.kts", detector: "java-gradle-settings-kts", required: []string{"classpath(\"com.example:legacy-settings-plugin:2.5.0\")"}},
		{name: "gradle-settings-kts-minimal", filename: "build-logic/settings.gradle.kts", detector: "java-gradle-settings-kts", required: []string{"rootProject.name = \"build-logic-kts\""}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "java", fixture.name, fixture.filename)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read settings fixture: %v", err)
			}
			for _, expected := range fixture.required {
				if !strings.Contains(string(contents), expected) {
					t.Errorf("fixture is missing %q", expected)
				}
			}
			if strings.HasPrefix(fixture.name, "gradle-settings") && strings.Contains(string(contents), "pluginManagement {") && strings.TrimSpace(strings.Split(string(contents), "pluginManagement {")[0]) != "" {
				t.Error("pluginManagement must be the first settings block")
			}
			if strings.Contains(fixture.name, "catalog") {
				catalogPath := filepath.Join(filepath.Dir(path), "gradle", "test.versions.toml")
				catalog, err := os.ReadFile(catalogPath)
				if err != nil {
					t.Fatalf("read imported version catalog: %v", err)
				}
				for _, expected := range []string{"[versions]", "groovy-core", "com.gradleup.shadow"} {
					if !strings.Contains(string(catalog), expected) {
						t.Errorf("imported version catalog is missing %q", expected)
					}
				}
			}
			scanRoot := filepath.Dir(path)
			if strings.Contains(fixture.filename, "/") {
				scanRoot = filepath.Join("..", "..", "testdata", "java", fixture.name)
			}
			result, err := Scan(scanRoot, nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.filename || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestRubyAppraisalAndCartfileResolvedFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name     string
		path     string
		detector DetectorID
		required []string
		absent   []string
		sources  int
		resolved []string
	}{
		{name: "appraisal", path: "gemfiles/rails_60.gemfile", detector: "ruby-appraisal", required: []string{"This file was generated by Appraisal", "source \"https://rubygems.org\"", "gem \"rails\", \"~> 6.0\""}},
		{name: "appraisal-basic", path: "gemfiles/rails_61.gemfile", detector: "ruby-appraisal", required: []string{"source \"https://rubygems.org\"", "gem \"rails\", \"~> 6.1.7\""}},
		{name: "appraisal-groups", path: "gemfiles/rails_70.gemfile", detector: "ruby-appraisal", required: []string{"gem \"rails\", \">= 7.0\", \"< 7.1\"", "group :test, :development", "require: false"}},
		{name: "appraisal-git", path: "gemfiles/edge_active_record.gemfile", detector: "ruby-appraisal", required: []string{"git: \"https://github.com/rails/rails.git\"", "github: \"rails/arel\"", "ref: \"0123456789abcdef0123456789abcdef01234567\""}},
		{name: "appraisal-local", path: "gemfiles/local_adapter.gemfile", detector: "ruby-appraisal", required: []string{"path: \"../my_adapter\"", "git: \"file:///tmp/plugin_fixture\"", "tag: \"v1.2.3\""}},
		{name: "appraisal-platforms", path: "gemfiles/platforms.gemfile", detector: "ruby-appraisal", required: []string{"platforms: %i[mri mingw x64_mingw]", "platforms :jruby", "jdbc-postgres"}},
		{name: "appraisal-eval", path: "gemfiles/modular.gemfile", detector: "ruby-appraisal", required: []string{"eval_gemfile \"shared/testing.gemfile\"", "group: :audit"}, sources: 2},
		{name: "appraisal-removal", path: "gemfiles/without_legacy.gemfile", detector: "ruby-appraisal", required: []string{"inherited legacy_adapter was removed", "gem \"minitest\", \"~> 5.25\""}, absent: []string{"gem \"legacy_adapter\""}},
		{name: "appraisal-gemspec", path: "gemfiles/library.gemfile", detector: "ruby-appraisal", required: []string{"gemspec path: \"../my_library.gemspec\", development_group: :development", "gem \"rake\", \"~> 13.2\""}},
		{name: "appraisal-source-variants", path: "gemfiles/sources.gemfile", detector: "ruby-appraisal", required: []string{"source \"https://gems.example.test\" do", "bitbucket: \"example/api_client\"", "gist: \"0123456789abcdef0123456789abcdef\"", "submodules: true"}},
		{name: "cartfile-resolved", path: "Cartfile.resolved", detector: "ios-cartfile-resolved", required: []string{"github \"Alamofire/Alamofire\" \"5.8.0\""}, resolved: []string{"github \"Alamofire/Alamofire\" \"5.8.0\"", "github \"onevcat/Kingfisher\" \"7.10.0\""}},
		{name: "cartfile-resolved-github", path: "Cartfile.resolved", detector: "ios-cartfile-resolved", required: []string{"github \"Alamofire/Alamofire\" \"5.10.2\"", "github \"onevcat/Kingfisher\" \"7.12.0\""}, resolved: []string{"github \"Alamofire/Alamofire\" \"5.10.2\"", "github \"onevcat/Kingfisher\" \"7.12.0\""}},
		{name: "cartfile-resolved-commit", path: "Cartfile.resolved", detector: "ios-cartfile-resolved", required: []string{"ReactiveSwift", "a1b2c3d4e5f6789012345678901234567890abcd"}, resolved: []string{"github \"ReactiveCocoa/ReactiveSwift\" \"a1b2c3d4e5f6789012345678901234567890abcd\"", "github \"github/Carthage\" \"0123456789abcdef0123456789abcdef01234567\""}},
		{name: "cartfile-resolved-git", path: "Cartfile.resolved", detector: "ios-cartfile-resolved", required: []string{"https://git.example.test/mobile/networking.git", "ssh://git@git.example.test:2222/mobile/analytics.git"}, resolved: []string{"git \"https://git.example.test/mobile/networking.git\" \"7d3c1a9b8e6f4d2c0b1234567890abcdef123456\"", "git \"ssh://git@git.example.test:2222/mobile/analytics.git\" \"v2.4.1\""}},
		{name: "cartfile-resolved-binary", path: "Cartfile.resolved", detector: "ios-cartfile-resolved", required: []string{"binary \"https://downloads.example.test/Framework.json\" \"3.2.1\"", "1.0.0-beta.2"}, resolved: []string{"binary \"https://downloads.example.test/Framework.json\" \"3.2.1\"", "binary \"https://cdn.example.test/Analytics.json\" \"1.0.0-beta.2\""}},
		{name: "cartfile-resolved-enterprise", path: "Cartfile.resolved", detector: "ios-cartfile-resolved", required: []string{"https://github.enterprise.test/mobile/private-framework", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}, resolved: []string{"github \"https://github.enterprise.test/mobile/private-framework\" \"4.0.0\"", "github \"https://github.enterprise.test/mobile/legacy-framework\" \"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef\""}},
		{name: "cartfile-resolved-mixed", path: "Cartfile.resolved", detector: "ios-cartfile-resolved", required: []string{"file:///opt/carthage/OfflineFramework.json", "file:///Users/build/SharedFramework.git", "realm/realm-swift"}, resolved: []string{"binary \"file:///opt/carthage/OfflineFramework.json\" \"2.0.0\"", "git \"file:///Users/build/SharedFramework.git\" \"release-2025-01\"", "github \"realm/realm-swift\" \"10.55.0\""}},
		{name: "cartfile-resolved-local-paths", path: "Cartfile.resolved", detector: "ios-cartfile-resolved", required: []string{"git@github.com:example/PrivateFramework.git", "../LocalFramework", "vendor/RelativeFramework.json", "/opt/artifacts/AbsoluteFramework.json"}, resolved: []string{"git \"git@github.com:example/PrivateFramework.git\" \"0123456789abcdef0123456789abcdef01234567\"", "git \"../LocalFramework\" \"release-2025-02\"", "binary \"vendor/RelativeFramework.json\" \"2.3.1\"", "binary \"/opt/artifacts/AbsoluteFramework.json\" \"1.2.3\""}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata")
			if strings.HasPrefix(fixture.name, "appraisal") {
				root = filepath.Join(root, "ruby", fixture.name)
			} else {
				root = filepath.Join(root, "ios", fixture.name)
			}
			contents, err := os.ReadFile(filepath.Join(root, fixture.path))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, expected := range fixture.required {
				if !strings.Contains(string(contents), expected) {
					t.Errorf("fixture is missing %q", expected)
				}
			}
			for _, unexpected := range fixture.absent {
				if strings.Contains(string(contents), unexpected) {
					t.Errorf("fixture unexpectedly contains %q", unexpected)
				}
			}
			if fixture.detector == "ios-cartfile-resolved" {
				lines := make([]string, 0)
				identifiers := make(map[string]bool)
				for _, line := range strings.Split(strings.TrimSpace(string(contents)), "\n") {
					fields := strings.Fields(line)
					if len(fields) != 3 || (fields[0] != "github" && fields[0] != "git" && fields[0] != "binary") || !strings.HasPrefix(fields[1], "\"") || !strings.HasSuffix(fields[1], "\"") || !strings.HasPrefix(fields[2], "\"") || !strings.HasSuffix(fields[2], "\"") {
						t.Fatalf("invalid Cartfile.resolved declaration %q", line)
					}
					identifier := fields[1]
					if identifiers[identifier] {
						t.Fatalf("duplicate Cartfile.resolved dependency %q", identifier)
					}
					identifiers[identifier] = true
					lines = append(lines, line)
				}
				if !slices.Equal(lines, fixture.resolved) {
					t.Fatalf("unexpected resolved declarations: got %#v want %#v", lines, fixture.resolved)
				}
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			wantSources := fixture.sources
			if wantSources == 0 {
				wantSources = 1
			}
			if len(result.Sources) != wantSources || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.path || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestRebarFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name     string
		filename string
		detector DetectorID
		required []string
	}{
		{name: "rebar-config-basic", filename: "rebar.config", detector: "erlang-rebar-config", required: []string{"cowboy", "{jsx, \"~> 3.1\"}", "~> 2.0", "~> 2.1.2", "~> 2.1.3-dev"}},
		{name: "rebar-config-packages", filename: "rebar.config", detector: "erlang-rebar-config", required: []string{"{pkg, uuid_erl}", "{my_app, \"1.2.3\", {pkg, my_app_fork}}", "~> 1.2.1"}},
		{name: "rebar-config-git", filename: "rebar.config", detector: "erlang-rebar-config", required: []string{"{tag, \"2.12.0\"}", "{branch, \"main\"}", "{ref, \"0123456789abcdef0123456789abcdef01234567\"}", "git://github.com/example/unversioned_git.git", "unversioned_hg"}},
		{name: "rebar-config-sources", filename: "rebar.config", detector: "erlang-rebar-config", required: []string{"{hg,", "git_subdir", "apps/sys_config"}},
		{name: "rebar-config-profiles", filename: "rebar.config", detector: "erlang-rebar-config", required: []string{"{default, [{deps,", "{test, [{deps,", "{plugins, [rebar3_hex, {rebar3_format, {git,", "{project_plugins, [{rebar3_ex_doc", "rebar3_lint"}},
		{name: "rebar-config-conflicts", filename: "rebar.config", detector: "erlang-rebar-config", required: []string{"deps_error_on_conflict", "rebar_packages_cdn"}},
		{name: "rebar-lock-packages", filename: "rebar.lock", detector: "erlang-rebar-lock", required: []string{"\"1.1.0\"", "pkg_hash", "<<\"cowboy\">>"}},
		{name: "rebar-lock-git", filename: "rebar.lock", detector: "erlang-rebar-lock", required: []string{"{git,", "{ref,\"0123456789abcdef0123456789abcdef01234567\"}"}},
		{name: "rebar-lock-hg", filename: "rebar.lock", detector: "erlang-rebar-lock", required: []string{"{hg,", "hg.example.test"}},
		{name: "rebar-lock-pkg-alias", filename: "rebar.lock", detector: "erlang-rebar-lock", required: []string{"uuid_erl", "my_app_fork", "pkg_hash"}},
		{name: "rebar-lock-transitive", filename: "rebar.lock", detector: "erlang-rebar-lock", required: []string{"<<\"top\">>", "},1}", "},2}", "pkg_hash", "pkg_hash_ext"}},
		{name: "rebar-lock-git-subdir", filename: "rebar.lock", detector: "erlang-rebar-lock", required: []string{"git_subdir", "\"ra\"", "\"apps/sys_config\""}},
		{name: "rebar-lock-empty", filename: "rebar.lock", detector: "erlang-rebar-lock", required: []string{"{\"1.2.0\",[]}", "{pkg_hash,[]}", "{pkg_hash_ext,[]}"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "erlang", fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, expected := range fixture.required {
				if !strings.Contains(string(contents), expected) {
					t.Errorf("fixture is missing %q", expected)
				}
			}
			if fixture.detector == "erlang-rebar-lock" {
				terms := strings.Split(strings.TrimSpace(string(contents)), ".\n")
				if len(terms) != 2 || !strings.HasPrefix(terms[0], "{\"") || (!strings.HasPrefix(terms[0], "{\"1.1.0\"") && !strings.HasPrefix(terms[0], "{\"1.2.0\"")) || !strings.HasPrefix(terms[1], "[{pkg_hash,") {
					t.Fatalf("invalid rebar lock term structure: %q", contents)
				}
				if strings.HasPrefix(terms[0], "{\"1.2.0\"") && !strings.Contains(terms[1], "{pkg_hash_ext,") {
					t.Fatalf("rebar 1.2 lock is missing pkg_hash_ext metadata: %q", contents)
				}
				if strings.Contains(terms[1], "pkg_hash_ext") && !strings.Contains(terms[1], "pkg_hash") {
					t.Fatalf("extended package hashes require package hashes: %q", contents)
				}
				for _, line := range strings.Split(terms[0], "},{") {
					if strings.Contains(line, "{pkg,") && (!strings.Contains(line, "<<\"") || !strings.Contains(line, "\">>")) {
						t.Fatalf("invalid package lock entry %q", line)
					}
				}
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.filename || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestClojureManifestFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name     string
		filename string
		detector DetectorID
		required []string
		sources  int
	}{
		{name: "deps-edn-basic", filename: "deps.edn", detector: "clojure-deps-edn", required: []string{":deps", "org.clojure/clojure", ":mvn/version \"1.12.0\""}},
		{name: "deps-edn-git", filename: "deps.edn", detector: "clojure-deps-edn", required: []string{":git/url", ":git/tag \"v1.2.3\"", ":git/sha \"0123456\""}},
		{name: "deps-edn-local", filename: "deps.edn", detector: "clojure-deps-edn", required: []string{":local/root \"../project\"", "driver.jar", ":deps/root \"libs/example\""}},
		{name: "deps-edn-aliases", filename: "deps.edn", detector: "clojure-deps-edn", required: []string{":extra-deps", ":override-deps", "criterium/criterium"}},
		{name: "deps-edn-replace", filename: "deps.edn", detector: "clojure-deps-edn", required: []string{":replace-deps", ":deps {io.github.clojure/tools.build", ":exec-fn"}},
		{name: "deps-edn-options", filename: "deps.edn", detector: "clojure-deps-edn", required: []string{":mvn/repos", ":exclusions", "$tests"}},
		{name: "deps-edn-git-hosts", filename: "deps.edn", detector: "clojure-deps-edn", required: []string{"ssh://git@example.test/team/private-lib.git", ":git/sha \"0123456789abcdef0123456789abcdef01234567\"", "io.gitlab.example/tools", "io.codeberg.example/utility"}},
		{name: "deps-edn-alias-resolution", filename: "deps.edn", detector: "clojure-deps-edn", required: []string{":default-deps", "org.clojure/data.json", ":classpath-overrides", "target/generated.jar", ":mvn/local-repo"}},
		{name: "project-clj-basic", filename: "project.clj", detector: "clojure-project-clj", required: []string{"defproject", ":dependencies", "cheshire \"5.13.0\""}},
		{name: "project-clj-options", filename: "project.clj", detector: "clojure-project-clj", required: []string{":exclusions", ":classifier \"tests\"", ":scope \"provided\""}},
		{name: "project-clj-artifacts", filename: "project.clj", detector: "clojure-project-clj", required: []string{"\"commons-io:commons-io\"", ":native-prefix \"linux-x86-64\"", ":extension \"pom\""}},
		{name: "project-clj-checkouts", filename: "project.clj", detector: "clojure-project-clj", required: []string{"example/local-lib \"0.2.0\"", ":checkout-deps-shares [:source-paths :resource-paths]"}, sources: 2},
		{name: "project-clj-managed", filename: "project.clj", detector: "clojure-project-clj", required: []string{":managed-dependencies", "slf4j-api", "jackson-databind"}},
		{name: "project-clj-managed-usage", filename: "project.clj", detector: "clojure-project-clj", required: []string{"slf4j-api :exclusions", "testing nil :classifier \"tests\""}},
		{name: "project-clj-profiles", filename: "project.clj", detector: "clojure-project-clj", required: []string{":plugins", ":hooks false", ":middleware false", "^{:pom-scope :provided}", "lein-test-refresh"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "clojure", fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, expected := range fixture.required {
				if !strings.Contains(string(contents), expected) {
					t.Errorf("fixture is missing %q", expected)
				}
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			wantSources := fixture.sources
			if wantSources == 0 {
				wantSources = 1
			}
			found := false
			for _, source := range result.Sources {
				if source.Detector == fixture.detector && source.Path == fixture.filename && source.Analysis == (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
					found = true
				}
			}
			if len(result.Sources) != wantSources || !found {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestBootAndCabalFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		language, name, filename string
		detector                 DetectorID
		required                 []string
	}{
		{"clojure", "boot-basic", "build.boot", "clojure-boot", []string{":dependencies '[[org.clojure/clojure \"1.12.0\"]", "[org.clojure/tools.cli \"1.1.230\"]"}},
		{"clojure", "boot-scoped", "build.boot", "clojure-boot", []string{"[adzerk/boot-cljs \"2.1.5\" :scope \"test\"]", "[adzerk/boot-reload \"0.6.0\" :scope \"test\"]"}},
		{"clojure", "boot-repositories", "build.boot", "clojure-boot", []string{"[\"private\" {:url \"https://repo.example.test/maven/\"", "[com.cemerick/piggieback \"0.2.2\"]"}},
		{"clojure", "boot-merge-env", "build.boot", "clojure-boot", []string{"boot/merge-env!", "[org.clojure/data.json \"2.5.1\"]"}},
		{"clojure", "boot-task-deps", "build.boot", "clojure-boot", []string{"(deftask development", "[org.clojure/tools.namespace \"1.5.0\"]"}},
		{"clojure", "boot-functional", "build.boot", "clojure-boot", []string{":dependencies #(conj %", "[example/private-lib \"1.2.3\"", ":exclusions [org.clojure/clojure]"}},
		{"clojure", "boot-direct-vector", "build.boot", "clojure-boot", []string{":dependencies [[cheshire \"5.13.0\"]", "[org.clojure/tools.reader \"[1.4.0,1.5.0)\"]"}},
		{"haskell", "cabal-basic", "example.cabal", "haskell-cabal", []string{"build-depends", "bytestring ==0.12.1.0"}},
		{"haskell", "cabal-components", "example.cabal", "haskell-cabal", []string{"executable", "test-suite"}},
		{"haskell", "cabal-conditions", "example.cabal", "haskell-cabal", []string{"if flag(networking)", "if os(windows)"}},
		{"haskell", "cabal-tools-native", "example.cabal", "haskell-cabal", []string{"build-tool-depends", "pkgconfig-depends", "setup-depends"}},
		{"haskell", "cabal-sublibraries", "example.cabal", "haskell-cabal", []string{"library internal", "example-sublibraries:internal", "servant:{servant,servant-server}"}},
		{"haskell", "cabal-common-benchmark", "example.cabal", "haskell-cabal", []string{"common shared-deps", "criterion ==1.6.* || ==1.7.*", "benchmark example-bench"}},
		{"haskell", "cabal-legacy-tools", "example.cabal", "haskell-cabal", []string{"cabal-version: 2.4", "build-tools: alex >=3.2 && <3.3, happy >=1.20"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", fixture.language, fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.filename))
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range fixture.required {
				if !strings.Contains(string(contents), expected) {
					t.Errorf("fixture is missing %q", expected)
				}
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.filename || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestStackFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name, file string
		detector   DetectorID
		required   []string
	}{
		{"stack-basic", "stack.yaml", "haskell-stack", []string{"snapshot: lts-24.43", "packages"}}, {"stack-hackage", "stack.yaml", "haskell-stack", []string{"@rev:0"}}, {"stack-git", "stack.yaml", "haskell-stack", []string{"git:", "github:", "commit:"}}, {"stack-archive", "stack.yaml", "haskell-stack", []string{"url:", "./vendor/local-package"}}, {"stack-flags", "stack.yaml", "haskell-stack", []string{"flags:", "drop-packages:"}}, {"stack-projects", "stack.yaml", "haskell-stack", []string{"resolver:", "extra-package-dbs:"}}, {"stack-completed", "stack.yaml", "haskell-stack", []string{"hackage:", "hg:", "subdirs: [., library]"}},
		{"stack-lock-hackage", "stack.yaml.lock", "haskell-stack-lock", []string{"hackage:", "pantry-tree:"}}, {"stack-lock-snapshot", "stack.yaml.lock", "haskell-stack-lock", []string{"snapshots:", "original: lts-24.43"}}, {"stack-lock-git", "stack.yaml.lock", "haskell-stack-lock", []string{"git:", "pantry-tree:"}}, {"stack-lock-archive", "stack.yaml.lock", "haskell-stack-lock", []string{"url:", "size: 613"}}, {"stack-lock-multiple", "stack.yaml.lock", "haskell-stack-lock", []string{"alpha-1.0", "beta-2.0"}}, {"stack-lock-empty", "stack.yaml.lock", "haskell-stack-lock", []string{"packages: []", "snapshots: []"}}, {"stack-lock-megarepo", "stack.yaml.lock", "haskell-stack-lock", []string{"autogenerated", "subdir: .", "hg:"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "haskell", fixture.name)
			contents, err := os.ReadFile(filepath.Join(root, fixture.file))
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range fixture.required {
				if !strings.Contains(string(contents), s) {
					t.Errorf("missing %q", s)
				}
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != fixture.detector || result.Sources[0].Path != fixture.file {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestCabalProjectFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, f := range []struct {
		name, file string
		detector   DetectorID
		want       string
	}{
		{"cabal-project-basic", "cabal.project", "haskell-cabal-project", "bytestring ==0.12.1.0"}, {"cabal-project-local", "cabal.project", "haskell-cabal-project", "optional-packages"}, {"cabal-project-git", "cabal.project", "haskell-cabal-project", "source-repository-package"}, {"cabal-project-monorepo", "cabal.project", "haskell-cabal-project", "subdir: cborg"}, {"cabal-project-solver", "cabal.project", "haskell-cabal-project", "allow-newer: ^all"},
		{"cabal-freeze-basic", "cabal.project.freeze", "haskell-cabal-project-freeze", "any.base ==4.18.2.1"}, {"cabal-freeze-flags", "cabal.project.freeze", "haskell-cabal-project-freeze", "aeson +ordered-keymap"}, {"cabal-freeze-source", "cabal.project.freeze", "haskell-cabal-project-freeze", "index-state"}, {"cabal-freeze-multiple", "cabal.project.freeze", "haskell-cabal-project-freeze", "any.directory"}, {"cabal-freeze-empty", "cabal.project.freeze", "haskell-cabal-project-freeze", "constraints:"}, {"cabal-freeze-generated", "cabal.project.freeze", "haskell-cabal-project-freeze", "any.aeson +ordered-keymap"},
	} {
		t.Run(f.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "haskell", f.name)
			b, err := os.ReadFile(filepath.Join(root, f.file))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), f.want) {
				t.Errorf("missing %q", f.want)
			}
			r, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.ContainsFunc(r.Sources, func(source DependencySourceResult) bool {
				return source.Detector == f.detector && source.Path == f.file
			}) {
				t.Fatalf("unexpected source: %+v", r.Sources)
			}
		})
	}
}

func assertPaketLockStructure(t *testing.T, contents string) {
	t.Helper()
	lines := strings.Split(contents, "\n")
	currentNuget, nextNuget := -1, 0
	packages := make(map[int]map[string]bool)
	children := make(map[int][]string)
	for index, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "NUGET" {
			currentNuget = nextNuget
			nextNuget++
			packages[currentNuget] = make(map[string]bool)
			continue
		}
		if line == "GITHUB" || line == "GIST" || line == "HTTP" || line == "GIT" || strings.HasPrefix(line, "GROUP ") {
			currentNuget = -1
			continue
		}
		if strings.HasPrefix(line, "remote:") {
			for _, following := range lines[index+1:] {
				if strings.TrimSpace(following) == "" {
					continue
				}
				if len(following) == len(strings.TrimLeft(following, " ")) {
					t.Fatalf("Paket lock remote has no indented entry: %q", line)
				}
				break
			}
		}
		if currentNuget < 0 || !strings.Contains(line, " (") {
			continue
		}
		name := strings.TrimSpace(line[:strings.Index(line, " (")])
		version := line[strings.Index(line, " (")+2:]
		version = strings.TrimSpace(version[:strings.Index(version, ")")])
		if name == "" || version == "" {
			t.Fatalf("invalid Paket NuGet entry %q", line)
		}
		indent := len(rawLine) - len(strings.TrimLeft(rawLine, " "))
		if indent == 4 {
			packages[currentNuget][name] = true
		} else if indent >= 6 {
			children[currentNuget] = append(children[currentNuget], name)
		}
	}
	for section, dependencies := range children {
		for _, dependency := range dependencies {
			if !packages[section][dependency] {
				t.Fatalf("Paket NuGet dependency %q is not resolved in its lock section", dependency)
			}
		}
	}
}

func containsTrimmedLine(contents, want string) bool {
	for _, line := range strings.Split(contents, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

type glideLockEntry struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Repository  string   `yaml:"repo"`
	VCS         string   `yaml:"vcs"`
	Subpackages []string `yaml:"subpackages"`
	OS          []string `yaml:"os"`
	Arch        []string `yaml:"arch"`
}

func TestGlideLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name        string
		imports     []string
		testImports []string
	}{
		{name: "glide-lock-basic", imports: []string{"github.com/codegangsta/cli", "github.com/Masterminds/semver"}},
		{name: "glide-lock-test-imports", imports: []string{"github.com/Masterminds/semver"}, testImports: []string{"github.com/spf13/cobra"}},
		{name: "glide-lock-repository-vcs", imports: []string{"github.com/Masterminds/vcs"}},
		{name: "glide-lock-subpackages", imports: []string{"gopkg.in/yaml.v2"}},
		{name: "glide-lock-platforms", imports: []string{"github.com/codegangsta/cli"}},
		{name: "glide-lock-non-git", imports: []string{"example.com/legacy.svn"}},
		{name: "glide-lock-empty"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "go", fixture.name, "glide.lock")
			var lock struct {
				Hash        string           `yaml:"hash"`
				Updated     time.Time        `yaml:"updated"`
				Imports     []glideLockEntry `yaml:"imports"`
				TestImports []glideLockEntry `yaml:"testImports"`
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read Glide lockfile: %v", err)
			}
			if err := yaml.Unmarshal(contents, &lock); err != nil {
				t.Fatalf("parse Glide lockfile: %v", err)
			}
			if len(lock.Hash) != 64 || strings.Trim(lock.Hash, "0123456789abcdef") != "" || lock.Updated.IsZero() {
				t.Fatalf("invalid Glide lockfile metadata: %#v", lock)
			}
			imports := make([]string, 0, len(lock.Imports))
			for _, dependency := range lock.Imports {
				assertGlideLockEntry(t, dependency)
				imports = append(imports, dependency.Name)
				if dependency.Name == "github.com/Masterminds/vcs" && (dependency.Repository != "git@github.com:Masterminds/vcs.git" || dependency.VCS != "git") {
					t.Fatalf("invalid repository override: %#v", dependency)
				}
				if dependency.Name == "gopkg.in/yaml.v2" && !slices.Equal(dependency.Subpackages, []string{".", "internal"}) {
					t.Fatalf("invalid locked subpackages: %#v", dependency)
				}
				if fixture.name == "glide-lock-platforms" && (!slices.Equal(dependency.OS, []string{"linux", "darwin"}) || !slices.Equal(dependency.Arch, []string{"amd64", "arm64"})) {
					t.Fatalf("invalid platform-filtered lock: %#v", dependency)
				}
				if fixture.name == "glide-lock-non-git" && (dependency.Repository != "https://svn.apache.org/repos/asf/commons/proper/lang/trunk" || dependency.VCS != "svn" || dependency.Version != "4123") {
					t.Fatalf("invalid Subversion lock: %#v", dependency)
				}
			}
			testImports := make([]string, 0, len(lock.TestImports))
			for _, dependency := range lock.TestImports {
				assertGlideLockEntry(t, dependency)
				if dependency.Name == "github.com/spf13/cobra" && (dependency.Repository != "https://github.com/spf13/cobra.git" || dependency.VCS != "git" || !slices.Equal(dependency.Subpackages, []string{"."})) {
					t.Fatalf("invalid test-import lock metadata: %#v", dependency)
				}
				testImports = append(testImports, dependency.Name)
			}
			if !slices.Equal(imports, fixture.imports) || !slices.Equal(testImports, fixture.testImports) {
				t.Fatalf("unexpected Glide locks: imports=%#v testImports=%#v", imports, testImports)
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "go-glide-lock" || result.Sources[0].Path != "glide.lock" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func assertGlideLockEntry(t *testing.T, dependency glideLockEntry) {
	t.Helper()
	if dependency.Name == "" || dependency.Version == "" || strings.ContainsAny(dependency.Version, " \t\n") {
		t.Fatalf("invalid resolved Glide lock entry: %#v", dependency)
	}
	if dependency.VCS != "" && dependency.VCS != "git" && dependency.VCS != "svn" && dependency.VCS != "hg" && dependency.VCS != "bzr" {
		t.Fatalf("unsupported Glide VCS %q", dependency.VCS)
	}
	if dependency.VCS == "git" && (len(dependency.Version) != 40 || strings.Trim(dependency.Version, "0123456789abcdef") != "") {
		t.Fatalf("invalid Git revision %q", dependency.Version)
	}
	if dependency.Repository != "" && strings.ContainsAny(dependency.Repository, " \t\n") {
		t.Fatalf("invalid Glide repository %q", dependency.Repository)
	}
}

func assertGopkgTomlRule(t *testing.T, name, version, branch, revision, source string) {
	t.Helper()
	if name == "" {
		t.Fatal("Gopkg.toml dependency rule has no name")
	}
	versionRules := 0
	for _, value := range []string{version, branch, revision} {
		if value != "" {
			versionRules++
		}
	}
	if versionRules > 1 {
		t.Fatalf("Gopkg.toml rule %q has %d version rules", name, versionRules)
	}
	if source != "" && strings.ContainsAny(source, " \t\n") {
		t.Fatalf("Gopkg.toml rule %q has invalid source %q", name, source)
	}
}

func validBazelLockfileContext(context string) bool {
	if context == "general" {
		return true
	}
	parts := strings.Split(context, ",")
	if len(parts) > 2 || len(parts) == 0 {
		return false
	}
	hasOS, hasArch := false, false
	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, "os:") && len(strings.TrimPrefix(part, "os:")) > 0 && !hasOS:
			hasOS = true
		case strings.HasPrefix(part, "arch:") && len(strings.TrimPrefix(part, "arch:")) > 0 && !hasArch:
			hasArch = true
		default:
			return false
		}
	}
	return hasOS || hasArch
}

func TestBazelBuildFileFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name        string
		fragments   []string
		occurrences map[string]int
	}{
		{name: "build-bazel", fragments: []string{`cc_binary(`, `name = "hello"`}, occurrences: map[string]int{"cc_binary(": 1}},
		{name: "build-bazel-cc-dependencies", fragments: []string{`implementation_deps = ["@fmt//:fmt"]`, `textual_hdrs = ["macros.inc"]`, `malloc = "@bazel_tools//tools/cpp:malloc"`, `toolchains = ["@bazel_tools//tools/cpp:toolchain_type"]`, `dynamic_deps = ["@grpc//:grpc++"]`}, occurrences: map[string]int{"cc_library(": 1, "cc_binary(": 1, "cc_shared_library(": 1}},
		{name: "build-bazel-java-dependencies", fragments: []string{`exports = ["@maven//:org_jspecify_jspecify"]`, `runtime_deps = ["@maven//:org_slf4j_slf4j_api"]`, `exported_plugins = ["@maven//:com_google_errorprone_error_prone_core"]`, `deploy_env = [":host_application"]`}, occurrences: map[string]int{"java_library(": 1, "java_binary(": 2}},
		{name: "build-bazel-python-configurable", fragments: []string{`deps = select({`, `":linux": ["@pypi//requests"]`, `"//conditions:default": ["@pypi//urllib3"]`, `imports = ["."]`, `pyi_deps = ["@pypi_typing_extensions//:typing_extensions"]`}, occurrences: map[string]int{"config_setting(": 1, "py_library(": 1, "py_binary(": 1}},
		{name: "build-bazel-genrule-tools", fragments: []string{`cmd = "$(location :version_tool) $(SRCS) > $@"`, `tools = [":version_tool"]`, `toolchains = ["@bazel_tools//tools/cpp:toolchain_type"]`, `output_to_bindir = True`}, occurrences: map[string]int{"genrule(": 1, "sh_binary(": 1}},
		{name: "build-bazel-platform-toolchains", fragments: []string{`constraint_setting(name = "runtime")`, `toolchain = "@formatters//:toolchain"`, `toolchain_type = ":formatter_toolchain_type"`, `exec_compatible_with = ["@platforms//os:linux"]`, `target_settings = [":optimized_build"]`, `constraint_values = [":container_runtime"]`}, occurrences: map[string]int{"constraint_setting(": 1, "constraint_value(": 1, "toolchain_type(": 1, "config_setting(": 1, "toolchain(": 1, "platform(": 1}},
		{name: "build-bazel-alias-filegroups", fragments: []string{`exports_files(["public.proto"])`, `"@googleapis//google/api:annotations_proto"`, `actual = "@com_google_protobuf//:protobuf"`}, occurrences: map[string]int{"filegroup(": 1, "alias(": 1, "cc_library(": 1}},
		{name: "build-bazel-external-loads", fragments: []string{`load("@rules_cc//cc:defs.bzl", "cc_library")`, `load("@rules_proto//proto:defs.bzl", "proto_library")`, `deps = ["@googleapis//google/api:annotations_proto"]`, `deps = [":api_proto", "@grpc//:grpc++"]`}, occurrences: map[string]int{"proto_library(": 1, "cc_library(": 1}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bazel", fixture.name, "BUILD.bazel"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			for declaration, expectedCount := range fixture.occurrences {
				if count := strings.Count(string(content), declaration); count != expectedCount {
					t.Fatalf("expected %d occurrences of %q, got %d", expectedCount, declaration, count)
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "bazel", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "bazel-build-file" || source.Path != "BUILD.bazel" {
				t.Fatalf("expected BUILD.bazel bazel-build-file source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestNxFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name            string
		fragments       []string
		expectedSources map[string]string
	}{
		{name: "config", fragments: []string{`"defaultBase": "main"`, `"default": ["{projectRoot}/**/*"]`}},
		// Retained only as a legacy compatibility fixture: Nx ignores this workspace-level field since v16.
		{name: "implicit-dependencies", fragments: []string{`"implicitDependencies"`, `"global-config.json": ["api", "web"]`, `"shared-*"`}},
		{name: "target-default-pipelines", fragments: []string{`"dependsOn": ["^build"]`, `"inputs": ["production", "^production"]`, `"projects": ["api", "web"]`}},
		{name: "current-pipeline-variants", fragments: []string{`"dependencies": true`, `"projects": "{dependencies}"`, `"^build-*"`}},
		{name: "target-default-filters", fragments: []string{`"plugin": "@nx/vitest"`, `"plugin": "@nx/jest"`, `"projects": ["apps/*", "!apps/legacy"]`}},
		{name: "target-default-executor-and-glob", fragments: []string{`"@nx/js:tsc"`, `"e2e-ci--**/**"`, `"executor": "@nx/jest:jest"`}},
		{name: "named-inputs", fragments: []string{`"production": ["default", "!{projectRoot}/**/*.spec.ts", "^production"]`, `"sharedGlobals": ["{workspaceRoot}/global-config.json"]`, `"inputs": ["...", "{workspaceRoot}/babel.config.json"]`}},
		{name: "plugins", fragments: []string{`"@nx/webpack/plugin"`, `"plugin": "@nx/vite/plugin"`, `"plugin": "@nx/jest/plugin"`}},
		{name: "dependency-providers", fragments: []string{`"globalGenerators": ["@acme/nx-plugin:sync-workspace"]`, `"disabledTaskSyncGenerators": ["@nx/js:tsc-sync"]`, `"@acme/nx-rules:enforce-boundaries"`, `"./tools/nx-rules/enforce-tags"`, `"nx-cloud://acme/compliance-rules/no-private-imports"`}},
		{name: "extends-and-release", fragments: []string{`"extends": "./nx.base.json"`, `"preserveMatchingDependencyRanges": true`, `"updateDependents": "always"`, `"projectsRelationship": "fixed"`}},
		{name: "extends-preset", fragments: []string{`"extends": "nx/presets/npm.json"`, `"dependsOn": ["^build"]`}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "nx", fixture.name, "nx.json"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var parsed map[string]any
			if err := json.Unmarshal(content, &parsed); err != nil {
				t.Fatalf("fixture must contain valid JSON: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "nx", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "js-nx" || source.Path != "nx.json" {
				t.Fatalf("expected nx.json js-nx source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}

	result, err := Scan(filepath.Join("..", "..", "testdata", "nx", "project-json"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan project.json boundary fixture: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected project.json not to be selected as js-nx, got %+v", result.Sources)
	}
}

func TestLernaFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name            string
		fragments       []string
		expectedSources map[string]string
	}{
		{name: "config", fragments: []string{`"version": "1.0.0"`, `"packages": ["packages/*"]`}},
		{name: "fixed-packages", fragments: []string{`"version": "3.4.5"`, `"packages": ["packages/*", "tools/*"]`, `"$schema"`}},
		{name: "independent-packages", fragments: []string{`"version": "independent"`, `"modules/*"`, `"examples/*"`}},
		{name: "pnpm-workspaces", fragments: []string{`"npmClient": "pnpm"`}, expectedSources: map[string]string{"lerna.json": "js-lerna", "pnpm-workspace.yaml": "js-pnpm-workspace"}},
		{name: "workspace-inheritance", fragments: []string{`"npmClient": "yarn"`, `"version": "1.0.0"`}, expectedSources: map[string]string{"lerna.json": "js-lerna", "package.json": "js"}},
		{name: "bun-client", fragments: []string{`"npmClient": "bun"`, `"packages": ["packages/*"]`}},
		{name: "version-command", fragments: []string{`"allowBranch": ["main", "release/*"]`, `"conventionalCommits": true`, `"forcePublish": ["@acme/core", "@acme/cli"]`, `"ignoreChanges"`}},
		{name: "publish-command", fragments: []string{`"distTag": "next"`, `"registry": "https://registry.example.test"`, `"verifyAccess": false`}},
		{name: "run-command", fragments: []string{`"includeDependencies": true`, `"scope": ["@acme/app-*"]`, `"ignore": ["@acme/legacy"]`}},
		{name: "dependency-graph-filters", fragments: []string{`"since": "origin/main"`, `"excludeDependents": true`, `"scope": ["@acme/*"]`}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "lerna", fixture.name, "lerna.json"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var parsed map[string]any
			if err := json.Unmarshal(content, &parsed); err != nil {
				t.Fatalf("fixture must contain valid JSON: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			switch fixture.name {
			case "fixed-packages", "independent-packages":
				packages, ok := parsed["packages"].([]any)
				if !ok || len(packages) != 2 {
					t.Fatalf("expected two package globs, got %#v", parsed["packages"])
				}
			case "version-command":
				command, _ := parsed["command"].(map[string]any)
				version, _ := command["version"].(map[string]any)
				if forcePublish, ok := version["forcePublish"].([]any); !ok || len(forcePublish) != 2 {
					t.Fatalf("expected version.forcePublish string array, got %#v", version["forcePublish"])
				}
			case "publish-command":
				command, _ := parsed["command"].(map[string]any)
				publish, _ := command["publish"].(map[string]any)
				if registry, ok := publish["registry"].(string); !ok || registry == "" {
					t.Fatalf("expected command.publish.registry string, got %#v", publish["registry"])
				}
			case "run-command":
				command, _ := parsed["command"].(map[string]any)
				run, _ := command["run"].(map[string]any)
				if includeDependencies, ok := run["includeDependencies"].(bool); !ok || !includeDependencies {
					t.Fatalf("expected command.run.includeDependencies true, got %#v", run["includeDependencies"])
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "lerna", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			expectedSources := fixture.expectedSources
			if expectedSources == nil {
				expectedSources = map[string]string{"lerna.json": "js-lerna"}
			}
			if len(result.Sources) != len(expectedSources) {
				t.Fatalf("expected %d sources, got %+v", len(expectedSources), result.Sources)
			}
			for _, source := range result.Sources {
				if expectedDetector, ok := expectedSources[source.Path]; !ok || source.Detector != DetectorID(expectedDetector) {
					t.Fatalf("unexpected source %+v; expected %v", source, expectedSources)
				}
				if source.Detector == "js-lerna" && source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
					t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
				}
			}
		})
	}

	pnpmWorkspace, err := os.ReadFile(filepath.Join("..", "..", "testdata", "lerna", "pnpm-workspaces", "pnpm-workspace.yaml"))
	if err != nil {
		t.Fatalf("read pnpm workspace fixture: %v", err)
	}
	var pnpmDefinition struct {
		Packages []string `yaml:"packages"`
	}
	if err := yaml.Unmarshal(pnpmWorkspace, &pnpmDefinition); err != nil {
		t.Fatalf("parse pnpm workspace fixture: %v", err)
	}
	if !slices.Equal(pnpmDefinition.Packages, []string{"packages/*", "apps/*"}) {
		t.Fatalf("expected pnpm workspace package globs, got %v", pnpmDefinition.Packages)
	}
}

func TestRushFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name            string
		fragments       []string
		expectedSources map[string]string
	}{
		{name: "config", fragments: []string{`"rushVersion": "5.100.0"`, `"projects": []`}},
		{name: "projects-inventory", fragments: []string{`"packageName": "@acme/web"`, `"projectFolder": "apps/web"`, `"tags": ["frontend", "production"]`}},
		{name: "decoupled-publish", fragments: []string{`"decoupledLocalDependencies": ["@acme/toolchain"]`, `"shouldPublish": true`, `"publishFolder": "temp/publish"`}, expectedSources: map[string]string{"rush.json": "js-rush", "libraries/library/package.json": "js", "tools/toolchain/package.json": "js"}},
		{name: "package-manager-npm", fragments: []string{`"npmVersion": "10.8.2"`, `"@acme/npm-project"`}},
		{name: "package-manager-yarn", fragments: []string{`"yarnVersion": "1.22.22"`, `"@acme/yarn-project"`}},
		{name: "repository-events", fragments: []string{`"url": "https://github.com/acme/monorepo"`, `"defaultRemote": "upstream"`, `"preRushBuild": ["common/scripts/pre-rush-build.js"]`}},
		{name: "policies-tags", fragments: []string{`"allowedProjectTags"`, `"reviewCategories": ["production", "tools"]`, `"ignoredNpmScopes": ["@types"]`}},
		{name: "installation-variants", fragments: []string{`"variantName": "previous-sdk"`, `"description": "Build using the previous SDK dependency versions"`}},
		{name: "version-policy-project", fragments: []string{`"versionPolicyName": "acme-lockstep"`, `"@acme/versioned-library"`}},
		{name: "subspaces", fragments: []string{`"subspaceName": "frontend"`, `"@acme/default-package"`, `"@acme/frontend-package"`}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "rush", fixture.name, "rush.json"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var parsed map[string]any
			if err := json.Unmarshal(content, &parsed); err != nil {
				t.Fatalf("fixture must contain valid JSON: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			if _, ok := parsed["rushVersion"].(string); !ok {
				t.Fatalf("expected rushVersion string, got %#v", parsed["rushVersion"])
			}
			projects, ok := parsed["projects"].([]any)
			if !ok {
				t.Fatalf("expected projects array, got %#v", parsed["projects"])
			}
			for _, rawProject := range projects {
				project, ok := rawProject.(map[string]any)
				if !ok {
					t.Fatalf("expected project object, got %#v", rawProject)
				}
				if _, ok := project["packageName"].(string); !ok {
					t.Fatalf("expected project packageName string, got %#v", project)
				}
				if _, ok := project["projectFolder"].(string); !ok {
					t.Fatalf("expected project projectFolder string, got %#v", project)
				}
			}
			managerKeys := 0
			for _, key := range []string{"pnpmVersion", "npmVersion", "yarnVersion"} {
				if _, ok := parsed[key].(string); ok {
					managerKeys++
				}
			}
			if managerKeys != 1 {
				t.Fatalf("expected exactly one package-manager version field, got %#v", parsed)
			}
			if fixture.name == "installation-variants" {
				variants, _ := parsed["variants"].([]any)
				if len(variants) != 1 {
					t.Fatalf("expected one installation variant, got %#v", parsed["variants"])
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "rush", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			expectedSources := fixture.expectedSources
			if expectedSources == nil {
				expectedSources = map[string]string{"rush.json": "js-rush"}
			}
			if len(result.Sources) != len(expectedSources) {
				t.Fatalf("expected %d sources, got %+v", len(expectedSources), result.Sources)
			}
			for _, source := range result.Sources {
				if expectedDetector, ok := expectedSources[source.Path]; !ok || source.Detector != DetectorID(expectedDetector) {
					t.Fatalf("unexpected source %+v; expected %v", source, expectedSources)
				}
				if source.Detector == "js-rush" && source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
					t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
				}
			}
		})
	}

	variantVersions, err := os.ReadFile(filepath.Join("..", "..", "testdata", "rush", "installation-variants", "common", "config", "rush", "variants", "previous-sdk", "common-versions.json"))
	if err != nil {
		t.Fatalf("read variant versions fixture: %v", err)
	}
	var variantConfig map[string]map[string]string
	if err := json.Unmarshal(variantVersions, &variantConfig); err != nil {
		t.Fatalf("parse variant versions fixture: %v", err)
	}
	if variantConfig["preferredVersions"]["@acme/sdk"] != "^1.4.0" {
		t.Fatalf("expected variant SDK dependency pin, got %#v", variantConfig)
	}

	policyData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "rush", "version-policy-project", "common", "config", "rush", "version-policies.json"))
	if err != nil {
		t.Fatalf("read version policies fixture: %v", err)
	}
	var policies struct {
		VersionPolicies []struct {
			PolicyName string `json:"policyName"`
		} `json:"versionPolicies"`
	}
	if err := json.Unmarshal(policyData, &policies); err != nil || len(policies.VersionPolicies) != 1 || policies.VersionPolicies[0].PolicyName != "acme-lockstep" {
		t.Fatalf("expected referenced acme-lockstep version policy, got %#v (err=%v)", policies, err)
	}

	subspacesData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "rush", "subspaces", "common", "config", "rush", "subspaces.json"))
	if err != nil {
		t.Fatalf("read subspaces fixture: %v", err)
	}
	var subspaces struct {
		SubspacesEnabled bool     `json:"subspacesEnabled"`
		SubspaceNames    []string `json:"subspaceNames"`
	}
	if err := json.Unmarshal(subspacesData, &subspaces); err != nil || !subspaces.SubspacesEnabled || !slices.Equal(subspaces.SubspaceNames, []string{"frontend"}) {
		t.Fatalf("expected enabled frontend subspace, got %#v (err=%v)", subspaces, err)
	}

	consumerData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "rush", "decoupled-publish", "libraries", "library", "package.json"))
	if err != nil {
		t.Fatalf("read decoupled consumer package fixture: %v", err)
	}
	var consumer struct {
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(consumerData, &consumer); err != nil || consumer.DevDependencies["@acme/toolchain"] != "workspace:^1.0.0" {
		t.Fatalf("expected decoupled toolchain workspace dependency, got %#v (err=%v)", consumer, err)
	}

	nestedResult, err := Scan(filepath.Join("..", "..", "testdata", "rush", "nested-config"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan nested Rush fixture: %v", err)
	}
	if len(nestedResult.Sources) != 1 || nestedResult.Sources[0].Detector != "js-rush" || nestedResult.Sources[0].Path != "config/rush.json" {
		t.Fatalf("expected nested config/rush.json js-rush source, got %+v", nestedResult.Sources)
	}
}

func TestTurboFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name            string
		fragments       []string
		expectedSources map[string]string
	}{
		{name: "config", fragments: []string{`"pipeline"`, `"build": { "outputs": ["dist/**"] }`}},
		{name: "task-dependency-relationships", fragments: []string{`"dependsOn": ["^build", "generate", "@acme/shared#compile"]`, `"dependsOn": ["build"]`, `"!.next/cache/**"`}},
		{name: "task-inputs-environment", fragments: []string{`"$TURBO_DEFAULT$"`, `"!README.md"`, `"$TURBO_ROOT$/tsconfig.json"`, `"!src/**/*.test.ts"`, `"MY_API_*"`, `"!MY_API_URL"`, `"passThroughEnv"`}},
		{name: "global-hash-inputs", fragments: []string{`"globalDependencies"`, `"globalEnv"`, `"globalPassThroughEnv"`, `"envMode": "strict"`}},
		{name: "watch-and-interaction", fragments: []string{`"persistent": true`, `"interruptible": true`, `"interactive": true`, `"cache": false`}},
		{name: "task-log-cache-options", fragments: []string{`"outputLogs": "errors-only"`, `"errorsOnlyShowHash": true`}},
		{name: "package-configuration", fragments: []string{`"tasks"`, `"dependsOn": ["^build"]`}, expectedSources: map[string]string{"turbo.json": "js-turbo", "apps/web/turbo.json": "js-turbo", "packages/turbo-config/turbo.json": "js-turbo", "package.json": "js", "apps/web/package.json": "js", "packages/turbo-config/package.json": "js"}},
		{name: "qualified-root-and-parallel-tasks", fragments: []string{`"web#lint"`, `"dependsOn": ["utils#build"]`, `"//#lint:root"`, `"with": ["api#dev", "web#dev"]`}},
		// Turbo v1 compatibility only: `pipeline` was renamed to `tasks` in Turbo 2.0.
		{name: "legacy-pipeline", fragments: []string{`"pipeline"`, `"dependsOn": ["^build"]`, `"outputs": []`}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "turbo", fixture.name, "turbo.json"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var parsed map[string]any
			if err := json.Unmarshal(content, &parsed); err != nil {
				t.Fatalf("fixture must contain valid JSON: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			if _, hasTasks := parsed["tasks"]; !hasTasks {
				if _, hasPipeline := parsed["pipeline"]; !hasPipeline {
					t.Fatalf("expected tasks or legacy pipeline, got %#v", parsed)
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "turbo", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			expectedSources := fixture.expectedSources
			if expectedSources == nil {
				expectedSources = map[string]string{"turbo.json": "js-turbo"}
			}
			if len(result.Sources) != len(expectedSources) {
				t.Fatalf("expected %d sources, got %+v", len(expectedSources), result.Sources)
			}
			for _, source := range result.Sources {
				if expectedDetector, ok := expectedSources[source.Path]; !ok || source.Detector != DetectorID(expectedDetector) {
					t.Fatalf("unexpected source %+v; expected %v", source, expectedSources)
				}
				if source.Detector == "js-turbo" && source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
					t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
				}
			}
		})
	}

	for _, relativePath := range []string{"apps/web/turbo.json", "packages/turbo-config/turbo.json"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "turbo", "package-configuration", filepath.FromSlash(relativePath)))
		if err != nil {
			t.Fatalf("read package configuration %s: %v", relativePath, err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(content, &parsed); err != nil {
			t.Fatalf("parse package configuration %s: %v", relativePath, err)
		}
		extends, ok := parsed["extends"].([]any)
		if !ok || len(extends) == 0 || extends[0] != "//" {
			t.Fatalf("expected %s to extend root first, got %#v", relativePath, parsed["extends"])
		}
	}
	webConfig, err := os.ReadFile(filepath.Join("..", "..", "testdata", "turbo", "package-configuration", "apps", "web", "turbo.json"))
	if err != nil {
		t.Fatalf("read web package configuration: %v", err)
	}
	if !strings.Contains(string(webConfig), `"outputs": ["$TURBO_EXTENDS$"`) || !strings.Contains(string(webConfig), `"dependsOn": ["$TURBO_EXTENDS$"`) {
		t.Fatalf("expected package config to extend inherited output and dependency arrays")
	}
}

func TestPantsFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name      string
		fragments []string
		sources   int
	}{
		{name: "config", fragments: []string{`pants_version = "2.32.0"`, `backend_packages = ["pants.backend.python"]`}},
		{name: "backend-plugins", fragments: []string{`pythonpath = ["%(buildroot)s/pants-plugins"]`, `backend_packages.add`, `plugins = ["ansicolors==1.18.0"]`}},
		{name: "python-resolves", fragments: []string{`enable_resolves = true`, `default_resolve = "web"`, `data-science = "3rdparty/python/data-science.lock"`, `[python.resolves_to_constraints_file]`}},
		{name: "python-inference", fragments: []string{`assets = true`, `ambiguity_resolution = "by_source_root"`, `unowned_dependency_behavior = "error"`, `ignored_unowned_imports`}},
		{name: "source-roots", fragments: []string{`root_patterns = ["/", "src/python", "tests/python", "pants-plugins"]`}},
		{name: "tool-resolve", fragments: []string{`pytest = "3rdparty/python/pytest.lock"`, `install_from_resolve = "pytest"`, `requirements = ["//3rdparty/python:pytest", "//3rdparty/python:pytest-cov"]`}},
		{name: "python-binaries", fragments: []string{`interpreter_constraints = ["CPython>=3.10,<3.13"]`, `[pex-binary-defaults]`, `emit_warnings = false`, `install_from_resolve = "tools"`}},
		{name: "python-repositories", fragments: []string{`"https://packages.acme.test/simple/"`, `find_links = ["%(buildroot)s/3rdparty/python/wheels"]`, `[python-repos.path_mappings]`}},
		{name: "jvm-resolves", fragments: []string{`pants.backend.experimental.java`, `default_resolve = "default"`, `analytics = "3rdparty/jvm/analytics.lock"`}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "pants", fixture.name, "pants.toml"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var parsed map[string]any
			if err := toml.Unmarshal(content, &parsed); err != nil {
				t.Fatalf("fixture must contain valid TOML: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			global, ok := parsed["GLOBAL"].(map[string]any)
			if !ok || global["pants_version"] == nil {
				t.Fatalf("expected GLOBAL.pants_version, got %#v", parsed)
			}
			if python, ok := parsed["python"].(map[string]any); ok {
				if defaultResolve, ok := python["default_resolve"].(string); ok {
					resolves, _ := python["resolves"].(map[string]any)
					if _, exists := resolves[defaultResolve]; !exists {
						t.Fatalf("expected python.default_resolve %q in python.resolves", defaultResolve)
					}
				}
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "pants", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "pants-config" || source.Path != "pants.toml" {
				t.Fatalf("expected pants.toml pants-config source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}

	markers := []string{"src/python/SOURCE_ROOT", "tests/python/pyproject.toml"}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join("..", "..", "testdata", "pants", "source-roots", filepath.FromSlash(marker))); err != nil {
			t.Fatalf("expected source-root marker %s: %v", marker, err)
		}
	}
}

func TestGitSubmoduleFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name      string
		fragments []string
		sections  int
	}{
		{name: "submodules", fragments: []string{`[submodule "vendor/lib"]`, `path = vendor/lib`, `url = https://github.com/example/lib.git`}, sections: 1},
		{name: "submodules-multiple", fragments: []string{`[submodule "libfoo"]`, `path = include/foo`, `git@github.com:acme/libbar.git`}, sections: 2},
		{name: "submodules-relative", fragments: []string{`url = ../shared-utils.git`, `url = ./theme.git`, `path = themes/nested/theme`}, sections: 2},
		{name: "submodules-tracking", fragments: []string{`branch = main`, `update = rebase`, `fetchRecurseSubmodules = on-demand`}, sections: 1},
		{name: "submodules-current-branch", fragments: []string{`branch = .`, `update = merge`, `ssh://git@example.test/acme/release-tools.git`}, sections: 1},
		{name: "submodules-shallow-ignore", fragments: []string{`shallow = true`, `ignore = dirty`, `update = none`, `ignore = all`}, sections: 2},
		{name: "submodules-custom-name", fragments: []string{`[submodule "web-ui-source"]`, `path = apps/web-ui`, `branch = release/2026`, `ignore = untracked`}, sections: 1},
		{name: "submodules-url-transports", fragments: []string{`[submodule "local cache"]`, `file:///srv/git/local-cache.git`, `git://git.example.test/acme/git-protocol.git`, `update = checkout`, `ignore = none`, `fetchRecurseSubmodules = true`, `fetchRecurseSubmodules = false`}, sections: 2},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "git", fixture.name, ".gitmodules"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			if count := strings.Count(string(content), "[submodule "); count != fixture.sections {
				t.Fatalf("expected %d submodule sections, got %d", fixture.sections, count)
			}
			sections := strings.Split(string(content), "[submodule ")
			paths := map[string]bool{}
			for _, section := range sections[1:] {
				pathCount, urlCount := 0, 0
				var path string
				for _, line := range strings.Split(section, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "path = ") {
						pathCount++
						path = strings.TrimPrefix(line, "path = ")
					}
					if strings.HasPrefix(line, "url = ") {
						urlCount++
					}
				}
				if pathCount != 1 || urlCount != 1 || path == "" || strings.HasSuffix(path, "/") || paths[path] {
					t.Fatalf("invalid submodule section: path=%q pathCount=%d urlCount=%d", path, pathCount, urlCount)
				}
				paths[path] = true
			}

			result, err := Scan(filepath.Join("..", "..", "testdata", "git", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "git-submodules" || source.Path != ".gitmodules" {
				t.Fatalf("expected .gitmodules git-submodules source, got %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestNixDefaultShellFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name      string
		filename  string
		fragments []string
	}{
		{name: "default-shell", filename: "default.nix", fragments: []string{`pkgs.mkShell`, `buildInputs`}},
		{name: "shell-mk-shell", filename: "shell.nix", fragments: []string{`nativeBuildInputs`, `buildInputs`, `nativeCheckInputs`, `checkInputs`}},
		{name: "shell-packages-inputs-from", filename: "shell.nix", fragments: []string{`packages = [ pkgs.nodejs pkgs.pnpm ];`, `inputsFrom = [ pkgs.hello pkgs.zlib ];`, `shellHook`}},
		{name: "default-mk-derivation", filename: "default.nix", fragments: []string{`mkDerivation`, `propagatedBuildInputs`, `strictDeps = true`}},
		{name: "default-fetch-tarball", filename: "default.nix", fragments: []string{`builtins.fetchTarball`, `sha256 =`, `nixos-24.11.tar.gz`}},
		{name: "default-fetch-git", filename: "default.nix", fragments: []string{`builtins.fetchGit`, `ref =`, `rev =`}},
		{name: "default-propagated-cross", filename: "default.nix", fragments: []string{`depsBuildBuild`, `nativeBuildInputs`, `depsBuildTarget`, `depsHostHost`, `buildInputs`, `depsTargetTarget`, `depsTargetTargetPropagated`}},
		{name: "shell-call-package", filename: "shell.nix", fragments: []string{`pkgs.callPackage`, `nativeBuildInputs`, `buildInputs`}},
		{name: "default-fetch-from-github", filename: "default.nix", fragments: []string{`pkgs.fetchFromGitHub`, `owner = "acme"`, `rev = "v1.0.0"`, `hash = "sha256-`}},
		{name: "default-composed-inputs", filename: "default.nix", fragments: []string{`with pkgs;`, `lib.optionals stdenv.isDarwin`, `lib.optional stdenv.isLinux`, `++`}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "nix", fixture.name, fixture.filename))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", "nix", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "nix-default-shell" || source.Path != fixture.filename {
				t.Fatalf("unexpected source %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestNixFlakeFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name      string
		fragments []string
		sources   int
	}{
		{name: "flake", fragments: []string{}},
		{name: "flake-basic-inputs", fragments: []string{`inputs.nixpkgs.url`, `github:NixOS/nixpkgs/nixos-24.11`, `outputs =`}},
		{name: "flake-follows", fragments: []string{`inputs.nixpkgs.follows = "nixpkgs"`, `home-manager/nixpkgs`}},
		{name: "flake-nonflake-source", fragments: []string{`flake = false`, `submodules = true`, `git+https://`}},
		{name: "flake-self-fetch-options", fragments: []string{`inputs.self.submodules = true`, `inputs.self.lfs = true`}},
		{name: "flake-output-packages", fragments: []string{`packages.${system}.default`, `devShells.${system}.default`, `pkgs.mkShell`}},
		{name: "flake-local-path", fragments: []string{`path:./tools`, `git+ssh://`, `shallow = true`}},
		{name: "flake-attribute-inputs", fragments: []string{`type = "github"`, `owner = "edolstra"`, `type = "indirect"`, `id = "nixpkgs"`}},
		{name: "flake-transitive-overrides", fragments: []string{`nixops.inputs.nixpkgs`, `owner = "acme"`, `home-manager.inputs.nixpkgs.follows`}},
		{name: "flake-provider-queries", fragments: []string{`gitlab:NixOS/nixpkgs`, `narHash=sha256-`, `local.url = "./tools"`, `path:./tools`, `&dir=nix`}, sources: 2},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "nix", fixture.name, "flake.nix"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", "nix", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			expectedSources := fixture.sources
			if expectedSources == 0 {
				expectedSources = 1
			}
			if len(result.Sources) != expectedSources {
				t.Fatalf("expected %d sources, got %+v", expectedSources, result.Sources)
			}
			for _, source := range result.Sources {
				if source.Detector != "nix-flake" {
					t.Fatalf("unexpected source %+v", source)
				}
				if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
					t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
				}
			}
		})
	}
}

func TestNixFlakeLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name      string
		fragments []string
	}{
		{name: "flake-lock", fragments: []string{}},
		{name: "flake-lock-github", fragments: []string{`"type": "github"`, `"narHash"`, `"lastModified"`}},
		{name: "flake-lock-follows", fragments: []string{`"inputs": { "nixpkgs": ["nixpkgs"] }`, `"home-manager"`}},
		{name: "flake-lock-nonflake", fragments: []string{`"flake":false`, `"type":"git"`, `"ref":"main"`}},
		{name: "flake-lock-tarball", fragments: []string{`"type":"tarball"`, `"lastModified":1730000000`, `latest.tar.gz`}},
		{name: "flake-lock-cycle", fragments: []string{`"a":[]`, `"b":["b"]`, `"repo":"a"`}},
		{name: "flake-lock-indirect-path", fragments: []string{`"type": "indirect"`, `"id": "nixpkgs"`, `"type": "path"`, `"dir": "nix"`}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "nix", fixture.name, "flake.lock"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var lock struct {
				Version int                        `json:"version"`
				Root    string                     `json:"root"`
				Nodes   map[string]json.RawMessage `json:"nodes"`
			}
			if err := json.Unmarshal(content, &lock); err != nil {
				t.Fatalf("valid lock JSON: %v", err)
			}
			if lock.Version != 7 || lock.Root == "" || lock.Nodes[lock.Root] == nil {
				t.Fatalf("invalid lock graph header: %#v", lock)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", "nix", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "nix-flake-lock" || source.Path != "flake.lock" {
				t.Fatalf("unexpected source %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestHelmChartLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name      string
		fragments []string
	}{
		{name: "chart-lock", fragments: []string{}},
		{name: "chart-lock-multiple", fragments: []string{`name: nginx`, `name: memcached`, `https://charts.bitnami.com/bitnami`}},
		{name: "chart-lock-aliases", fragments: []string{`alias: cache-primary`, `repository: "@bitnami"`, `repository: "alias:bitnami"`}},
		{name: "chart-lock-local", fragments: []string{`file://../shared-library`}},
		{name: "chart-lock-oci", fragments: []string{`oci://registry.example.test/helm`, `version: 2.7.0`}},
		{name: "chart-lock-conditions", fragments: []string{`condition: metrics.enabled,global.metrics.enabled`, `tags:`, `import-values:`}},
		{name: "chart-lock-plugin", fragments: []string{`s3://helm-charts/releases`}},
		{name: "chart-lock-vendored", fragments: []string{`name: local-subchart`, `version: 0.4.0`}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "helm", fixture.name, "Chart.lock"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var lock struct {
				Dependencies []map[string]any `yaml:"dependencies"`
				Digest       string           `yaml:"digest"`
				Generated    string           `yaml:"generated"`
			}
			if err := yaml.Unmarshal(content, &lock); err != nil {
				t.Fatalf("valid lock YAML: %v", err)
			}
			if len(lock.Dependencies) == 0 {
				t.Fatalf("invalid Chart.lock shape: %#v", lock)
			}
			if !strings.HasPrefix(lock.Digest, "sha256:") || lock.Generated == "" {
				t.Fatalf("invalid Chart.lock integrity metadata: %#v", lock)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", "helm", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "helm-chart-lock" || source.Path != "Chart.lock" {
				t.Fatalf("unexpected source %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestHomebrewBrewfileLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name    string
		entries map[string][]string
		fields  []string
	}{
		{name: "brewfile-lock", entries: map[string][]string{"brew": {"git"}}, fields: []string{"version"}},
		{name: "brewfile-lock-formulae", entries: map[string][]string{"brew": {"git", "postgresql@16", "acme/tools/internal-cli"}}, fields: []string{"version"}},
		{name: "brewfile-lock-desktop", entries: map[string][]string{"cask": {"firefox", "wezterm"}, "mas": {"Refined GitHub"}}, fields: []string{"version", "id"}},
		{name: "brewfile-lock-taps", entries: map[string][]string{"tap": {"acme/tools"}, "brew": {"acme/tools/internal-cli"}}, fields: []string{"revision", "version"}},
		{name: "brewfile-lock-language-tools", entries: map[string][]string{"brew": {"golangci-lint", "cargo-audit"}, "cask": {"visual-studio-code"}}, fields: []string{"version", "linux"}},
		{name: "brewfile-lock-platform-tools", entries: map[string][]string{"brew": {"kubectl"}, "mas": {"Xcode"}}, fields: []string{"id", "version"}},
		{name: "brewfile-lock-source-integrity", entries: map[string][]string{"brew": {"swiftformat"}}, fields: []string{"bottle", "root_url", "arm64_sonoma", "sha256"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "homebrew", fixture.name, "Brewfile.lock.json"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var lock struct {
				Entries map[string]map[string]json.RawMessage `json:"entries"`
				System  map[string]json.RawMessage            `json:"system"`
			}
			if err := json.Unmarshal(content, &lock); err != nil {
				t.Fatalf("valid lock JSON: %v", err)
			}
			if len(lock.Entries) == 0 || len(lock.System) == 0 || !strings.Contains(string(content), `"HOMEBREW_VERSION"`) {
				t.Fatalf("invalid Brewfile.lock.json shape: %#v", lock)
			}
			for entryType, names := range fixture.entries {
				entries := lock.Entries[entryType]
				for _, name := range names {
					raw := entries[name]
					if len(raw) == 0 {
						t.Fatalf("expected %q entry %q, got %#v", entryType, name, lock.Entries)
					}
					var entry map[string]json.RawMessage
					if err := json.Unmarshal(raw, &entry); err != nil {
						t.Fatalf("decode %q entry %q: %v", entryType, name, err)
					}
					switch entryType {
					case "brew", "cask":
						if len(entry["version"]) == 0 {
							t.Fatalf("expected version on %q entry %q: %#v", entryType, name, entry)
						}
					case "mas":
						if len(entry["id"]) == 0 || len(entry["version"]) == 0 {
							t.Fatalf("expected id and version on MAS entry %q: %#v", name, entry)
						}
					case "tap":
						var revision string
						if err := json.Unmarshal(entry["revision"], &revision); err != nil || len(revision) != 40 {
							t.Fatalf("expected 40-character tap revision on %q: %#v", name, entry)
						}
					}
				}
			}
			for _, field := range fixture.fields {
				if !strings.Contains(string(content), `"`+field+`"`) {
					t.Fatalf("expected fixture to contain %q", field)
				}
			}
			if fixture.name == "brewfile-lock-source-integrity" {
				var formula struct {
					Bottle struct {
						RootURL string `json:"root_url"`
						Files   map[string]struct {
							SHA256 string `json:"sha256"`
						} `json:"files"`
					} `json:"bottle"`
				}
				if err := json.Unmarshal(lock.Entries["brew"]["swiftformat"], &formula); err != nil {
					t.Fatalf("decode bottle metadata: %v", err)
				}
				if formula.Bottle.RootURL == "" || len(formula.Bottle.Files) != 2 {
					t.Fatalf("invalid bottle metadata: %#v", formula.Bottle)
				}
				for platform, file := range formula.Bottle.Files {
					if len(file.SHA256) != 64 {
						t.Fatalf("expected 64-character bottle checksum for %q, got %q", platform, file.SHA256)
					}
				}
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", "homebrew", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "homebrew-brewfile-lock" || source.Path != "Brewfile.lock.json" {
				t.Fatalf("unexpected source %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestBufLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name      string
		version   string
		deps      int
		fragments []string
	}{
		{name: "lock", version: "v1", deps: 1, fragments: []string{"remote: buf.build", "owner: acme", "repository: paymentapis"}},
		{name: "lock-v1-multiple", version: "v1", deps: 2, fragments: []string{"remote: buf.testing", "repository: date", "repository: extension", "shake256:"}},
		{name: "lock-v1beta1", version: "v1beta1", deps: 1, fragments: []string{"remote: bufbuild.test", "repository: second", "shake256:"}},
		{name: "lock-v2-single", version: "v2", deps: 1, fragments: []string{"name: buf.build/googleapis/googleapis"}},
		{name: "lock-v2-multiple-deps", version: "v2", deps: 2, fragments: []string{"name: buf.testing/acme/date", "name: buf.testing/acme/extension"}},
		{name: "lock-v2-commit-variant", version: "v2", deps: 1, fragments: []string{"commit: 004180b77378443887d3b55cabc00384"}},
		{name: "lock-v2-legacy-metadata", version: "v2", deps: 1, fragments: []string{"Historical metadata", "branch: main", "create_time:"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "buf", fixture.name, "buf.lock"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var lock struct {
				Version string           `yaml:"version"`
				Deps    []map[string]any `yaml:"deps"`
			}
			if err := yaml.Unmarshal(content, &lock); err != nil {
				t.Fatalf("valid lock YAML: %v", err)
			}
			if lock.Version != fixture.version || len(lock.Deps) != fixture.deps {
				t.Fatalf("unexpected lock header: %#v", lock)
			}
			for _, dep := range lock.Deps {
				commit, _ := dep["commit"].(string)
				digest, _ := dep["digest"].(string)
				if len(commit) != 32 || !isLowerHex(commit) || !isBufLockDigest(digest) {
					t.Fatalf("invalid module pin: %#v", dep)
				}
				if lock.Version == "v2" {
					name, _ := dep["name"].(string)
					if parts := strings.Split(name, "/"); len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
						t.Fatalf("expected v2 module name, got %#v", dep)
					}
				} else {
					for _, key := range []string{"remote", "owner", "repository"} {
						if value, _ := dep[key].(string); value == "" {
							t.Fatalf("expected %q on legacy module pin: %#v", key, dep)
						}
					}
				}
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", "buf", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "buf-lock" || source.Path != "buf.lock" {
				t.Fatalf("unexpected source %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestPuppetfileFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name      string
		fragments []string
	}{
		{name: "puppetfile", fragments: []string{"forge", "puppetlabs-stdlib", "'9.0.0'"}},
		{name: "puppetfile-forge-current", fragments: []string{"puppetlabs/stdlib", "puppetlabs/apache", "puppetlabs/ntp', :latest"}},
		{name: "puppetfile-forge-constraints", fragments: []string{"forge", "puppetlabs-concat", ">= 8.0.0 < 10.0.0", "~> 9.0"}},
		{name: "puppetfile-git-ref", fragments: []string{":git =>", ":ref =>", "ssh://git@"}},
		{name: "puppetfile-git-tag-commit", fragments: []string{":tag => '0.9.0'", ":commit => '8df51aa'"}},
		{name: "puppetfile-git-branch", fragments: []string{":branch => 'proxy_match'", ":branch => :control_branch", ":default_branch => 'main'"}},
		{name: "puppetfile-install-paths", fragments: []string{"moduledir 'thirdparty'", ":install_path => 'site-data'", ":install_path => ''"}},
		{name: "puppetfile-git-default-branch", fragments: []string{":ref => 'environment/production'", ":default_branch => 'main'", "unversioned_data"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "puppet", fixture.name, "Puppetfile"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			if !strings.Contains(string(content), "mod '") {
				t.Fatalf("expected Puppetfile module declaration: %s", content)
			}
			if strings.Contains(string(content), ":default_branch") &&
				!strings.Contains(string(content), ":ref") &&
				!strings.Contains(string(content), ":tag") &&
				!strings.Contains(string(content), ":commit") &&
				!strings.Contains(string(content), ":branch") {
				t.Fatalf("default_branch requires a Git version option: %s", content)
			}
			if strings.Contains(fixture.name, "git") || strings.Contains(fixture.name, "install-paths") {
				if !strings.Contains(string(content), ":git =>") {
					t.Fatalf("expected Git declaration: %s", content)
				}
			}
			if fixture.name == "puppetfile-forge-current" {
				if strings.Contains(string(content), "forge ") || !strings.Contains(string(content), "mod 'puppetlabs/") {
					t.Fatalf("expected canonical r10k Forge declarations without legacy forge setting: %s", content)
				}
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", "puppet", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "puppet-puppetfile" || source.Path != "Puppetfile" {
				t.Fatalf("unexpected source %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestChefBerksfileFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name      string
		cookbooks int
		fragments []string
	}{
		{name: "berksfile", cookbooks: 1, fragments: []string{"source", "metadata", "cookbook 'nginx', '~> 3.0'"}},
		{name: "berksfile-sources", cookbooks: 2, fragments: []string{"supermarket.example.test", "supermarket.chef.io", "metadata", "cookbook 'apt'"}},
		{name: "berksfile-legacy-source", cookbooks: 1, fragments: []string{"Historical Berkshelf syntax", "site :opscode"}},
		{name: "berksfile-path", cookbooks: 2, fragments: []string{"path: '../library-cookbook'", "path: 'test/fixtures/test-helper'"}},
		{name: "berksfile-git-pins", cookbooks: 3, fragments: []string{"git:", "branch: 'smartos-dev'", "tag: '1.2.3'", "ref: 'eef7e65806e7ff3bdbe148e27c447ef4a8bc3881'"}},
		{name: "berksfile-git-monorepo", cookbooks: 1, fragments: []string{"git@git.example.test", "rel: 'cookbooks/library-cookbook'"}},
		{name: "berksfile-github", cookbooks: 3, fragments: []string{"github: 'example/library-cookbook'", "tag: 'v2.4.0'", "github: 'example/revision-cookbook'", "ref: 'eef7e65806e7ff3bdbe148e27c447ef4a8bc3881'"}},
		{name: "berksfile-groups", cookbooks: 3, fragments: []string{"group :test do", "group: :test", "solver :gecode"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "chef", fixture.name, "Berksfile"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			if count := strings.Count(string(content), "cookbook '"); count != fixture.cookbooks {
				t.Fatalf("expected %d cookbook declarations, got %d: %s", fixture.cookbooks, count, content)
			}
			if fixture.name == "berksfile-sources" {
				privateIndex := strings.Index(string(content), "https://supermarket.example.test")
				publicIndex := strings.Index(string(content), "https://supermarket.chef.io")
				if privateIndex < 0 || publicIndex < 0 || privateIndex >= publicIndex || strings.Count(string(content), "source '") != 2 {
					t.Fatalf("expected private source before public fallback: %s", content)
				}
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", "chef", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "chef-berksfile" || source.Path != "Berksfile" {
				t.Fatalf("unexpected source %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestChefBerksfileLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name      string
		fragments []string
		legacy    bool
	}{
		{name: "berksfile-lock", fragments: []string{"nginx (>= 0.0.0)", "nginx (3.0.0)"}},
		{name: "berksfile-lock-multiple", fragments: []string{"jenkins-config", "path: ../cookbook-path/jenkins-config", "build-essential (>= 0.0.0)"}},
		{name: "berksfile-lock-path-metadata", fragments: []string{"path: .", "metadata: true", "path: test/fixtures/test-helper"}},
		{name: "berksfile-lock-git-ref", fragments: []string{"git: https://github.com/RiotGames/berkshelf-cookbook-fixture.git", "revision: 919afa0c402089df23ebdf36637f12271b8a96b4", "ref: 919afa0"}},
		{name: "berksfile-lock-git-branch-tag", fragments: []string{"branch: master", "tag: v0.2.0", "revision: a97b9447cbd41a5fe58eee2026e48ccb503bd3bc"}},
		{name: "berksfile-lock-github-rel", fragments: []string{"branch: rel", "rel: cookbooks/berkshelf-cookbook-fixture"}},
		{name: "berksfile-lock-git-all-options", fragments: []string{"git: https://repo.example.test/cookbooks/bacon.git", "revision: defjkl123456", "ref: abc123", "branch: ham", "tag: v1.2.3", "rel: cookbooks/bacon"}},
		{name: "berksfile-lock-legacy-json", legacy: true, fragments: []string{`"dependencies"`, `"locked_version": "2.0.3"`, `"path": "."`}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "chef", fixture.name, "Berksfile.lock"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if fixture.legacy {
				var lock struct {
					Dependencies map[string]struct {
						LockedVersion string `json:"locked_version"`
						Path          string `json:"path"`
					} `json:"dependencies"`
				}
				if err := json.Unmarshal(content, &lock); err != nil || len(lock.Dependencies) != 4 {
					t.Fatalf("valid legacy JSON lock: %v, %#v", err, lock)
				}
				for name, dependency := range lock.Dependencies {
					if dependency.LockedVersion == "" {
						t.Fatalf("missing locked version for legacy dependency %q: %#v", name, dependency)
					}
				}
				if lock.Dependencies["jenkins"].Path != "." {
					t.Fatalf("expected legacy Jenkins path to survive: %#v", lock.Dependencies["jenkins"])
				}
			} else {
				direct, graph := parseModernBerksfileLock(t, content)
				if fixture.name == "berksfile-lock-multiple" {
					if graph["jenkins"]["runit"] != "~> 1.5" || graph["runit"]["build-essential"] != ">= 0.0.0" {
						t.Fatalf("expected preserved multi-hop graph, got %#v", graph)
					}
				}
				if fixture.name == "berksfile-lock-git-all-options" {
					wantOrder := []string{"git:", "revision:", "ref:", "branch:", "tag:", "rel:"}
					last := -1
					for _, field := range wantOrder {
						index := strings.Index(string(content), field)
						if index <= last {
							t.Fatalf("expected canonical Git lock field order %v: %s", wantOrder, content)
						}
						last = index
					}
				}
				if len(direct) == 0 || len(graph) == 0 {
					t.Fatalf("empty modern lock graph: direct=%#v graph=%#v", direct, graph)
				}
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", "chef", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "chef-berksfile-lock" || source.Path != "Berksfile.lock" {
				t.Fatalf("unexpected source %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestChefMetadataFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name      string
		depends   int
		fragments []string
	}{
		{name: "metadata", depends: 1, fragments: []string{"depends 'apt', '~> 7.0'"}},
		{name: "metadata-unconstrained", depends: 2, fragments: []string{"depends 'apt'", "depends 'nginx'"}},
		{name: "metadata-pessimistic", depends: 2, fragments: []string{"~> 1.2.3", "~> 2.8"}},
		{name: "metadata-comparisons", depends: 5, fragments: []string{"< 1.0", "<= 2.0.0", "= 3.1.4", ">= 4.0.0", "> 5.0.0"}},
		{name: "metadata-static-bounds", depends: 2, fragments: []string{"depends 'mysql', '>= 8.0'", "depends 'postgresql', '<= 16.0'"}},
		{name: "metadata-ruby-forms", depends: 0, fragments: []string{"depends(\"apt\", \"~> 7.0\")", "%w[nginx redis].each { |cookbook| depends cookbook }"}},
		{name: "metadata-gem", depends: 1, fragments: []string{"gem 'loofah'", "gem 'chef-sugar'"}},
		{name: "metadata-platforms", depends: 1, fragments: []string{"chef_version", "ohai_version", "supports 'ubuntu'", "supports 'windows'"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "chef", fixture.name, "metadata.rb"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if strings.Count(string(content), "depends '") != fixture.depends {
				t.Fatalf("unexpected dependency count in %s", content)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected fixture to contain %q", fragment)
				}
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", "chef", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected 1 source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "chef-metadata" || source.Path != "metadata.rb" {
				t.Fatalf("unexpected source %+v", source)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", source.Analysis)
			}
		})
	}
}

func TestChefPolicyfileFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name      string
		fragments []string
	}{
		{name: "policyfile", fragments: []string{"name \"myapp\"", "run_list", "cookbook \"apt\""}},
		{name: "policyfile-sources", fragments: []string{":supermarket", "preferred_for 'chef-client', 'nginx', 'mysql'"}},
		{name: "policyfile-cookbook-locations", fragments: []string{"path: 'cookbooks/my_app'", "github:", "git:", "tag: 'v0.12.0'"}},
		{name: "policyfile-chef-repo-artifactory", fragments: []string{":chef_repo", ":artifactory", ":chef_server"}},
		{name: "policyfile-named-run-list", fragments: []string{"named_run_list :update_app", "application::update"}},
		{name: "policyfile-attributes", fragments: []string{"default['stage']", "default['prod']", "override['mysql']"}},
		{name: "policyfile-include-policy", fragments: []string{"include_policy 'base'", "include_policy 'directory-base', path: '.'", "include_policy 'shared'", "include_policy 'remote'", "include_policy 'server-base'", "policy_name: 'base'", "server:"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "chef", fixture.name, "Policyfile.rb"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			for _, fragment := range fixture.fragments {
				if !strings.Contains(string(content), fragment) {
					t.Fatalf("expected %q", fragment)
				}
			}
			if (!strings.Contains(string(content), "name '") && !strings.Contains(string(content), "name \"")) || (!strings.Contains(string(content), "run_list '") && !strings.Contains(string(content), "run_list \"")) {
				t.Fatalf("missing required Policyfile name or run_list: %s", content)
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", "chef", fixture.name), nil, ruleset)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			s := result.Sources[0]
			if s.Detector != "chef-policyfile" || s.Path != "Policyfile.rb" || s.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source %+v", s)
			}
		})
	}
}

func TestChefPolicyfileLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, name := range []string{"policyfile-lock", "policyfile-lock-local", "policyfile-lock-supermarket", "policyfile-lock-git", "policyfile-lock-solution", "policyfile-lock-multiple"} {
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "chef", name, "Policyfile.lock.json"))
			if err != nil {
				t.Fatal(err)
			}
			var lock struct {
				RevisionID    string   `json:"revision_id"`
				Name          string   `json:"name"`
				RunList       []string `json:"run_list"`
				CookbookLocks map[string]struct {
					Version    string `json:"version"`
					Identifier string `json:"identifier"`
				} `json:"cookbook_locks"`
				DefaultAttributes  map[string]any `json:"default_attributes"`
				OverrideAttributes map[string]any `json:"override_attributes"`
			}
			if err := json.Unmarshal(content, &lock); err != nil || lock.RevisionID == "" || lock.Name == "" || len(lock.RunList) == 0 || len(lock.CookbookLocks) == 0 {
				t.Fatalf("invalid Policyfile lock: %v %#v", err, lock)
			}
			if lock.DefaultAttributes == nil || lock.OverrideAttributes == nil {
				t.Fatalf("missing attribute maps: %#v", lock)
			}
			for cookbook, entry := range lock.CookbookLocks {
				if entry.Version == "" || len(entry.Identifier) != 40 || !isLowerHex(entry.Identifier) {
					t.Fatalf("invalid cookbook lock %q: %#v", cookbook, entry)
				}
			}
			result, err := Scan(filepath.Join("..", "..", "testdata", "chef", name), nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "chef-policyfile-lock" || result.Sources[0].Path != "Policyfile.lock.json" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected sources %+v", result.Sources)
			}
		})
	}
}

func parseModernBerksfileLock(t *testing.T, content []byte) (map[string]bool, map[string]map[string]string) {
	t.Helper()
	const (
		before = iota
		dependencies
		graph
	)
	section := before
	sections := map[int]int{}
	direct := map[string]bool{}
	graphNodes := map[string]map[string]string{}
	var currentDirect, currentNode string
	for _, line := range strings.Split(strings.TrimSuffix(string(content), "\n"), "\n") {
		switch line {
		case "DEPENDENCIES":
			section = dependencies
			sections[dependencies]++
			continue
		case "GRAPH":
			section = graph
			sections[graph]++
			continue
		}
		if line == "" {
			continue
		}
		if section == before {
			t.Fatalf("record before lock section: %q", line)
		}
		if strings.HasPrefix(line, "    ") {
			value := strings.TrimSpace(line)
			if section == dependencies {
				if currentDirect == "" || !strings.Contains(value, ":") {
					t.Fatalf("invalid source location record %q", line)
				}
			} else {
				name, constraint := berksLockNameAndConstraint(t, value)
				if currentNode == "" || name == "" || constraint == "" {
					t.Fatalf("invalid graph edge %q", line)
				}
				graphNodes[currentNode][name] = constraint
			}
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			t.Fatalf("invalid lock record indentation: %q", line)
		}
		if section == dependencies {
			record := strings.TrimSpace(line)
			name := record
			if open := strings.LastIndex(record, " ("); open >= 1 && strings.HasSuffix(record, ")") {
				name = record[:open]
			}
			if name == "" || direct[name] {
				t.Fatalf("invalid or duplicate direct dependency %q", line)
			}
			direct[name] = true
			currentDirect = name
		} else {
			name, value := berksLockNameAndConstraint(t, strings.TrimSpace(line))
			if value == "" {
				t.Fatalf("graph node %q has no resolved version", name)
			}
			if _, exists := graphNodes[name]; exists {
				t.Fatalf("duplicate graph node %q", name)
			}
			graphNodes[name] = map[string]string{}
			currentNode = name
		}
	}
	if sections[dependencies] != 1 || sections[graph] != 1 || section != graph {
		t.Fatalf("expected exactly one ordered DEPENDENCIES and GRAPH section, got %#v", sections)
	}
	for name := range direct {
		if _, ok := graphNodes[name]; !ok {
			t.Fatalf("direct dependency %q has no graph node: %#v", name, graphNodes)
		}
	}
	for node, edges := range graphNodes {
		for target := range edges {
			if _, ok := graphNodes[target]; !ok {
				t.Fatalf("graph edge %q -> %q has no target node: %#v", node, target, graphNodes)
			}
		}
	}
	return direct, graphNodes
}

func berksLockNameAndConstraint(t *testing.T, record string) (string, string) {
	t.Helper()
	open := strings.LastIndex(record, " (")
	if open < 1 || !strings.HasSuffix(record, ")") {
		t.Fatalf("invalid Berksfile.lock dependency record %q", record)
	}
	return record[:open], record[open+2 : len(record)-1]
}

func isBufLockDigest(digest string) bool {
	prefix, payload, ok := strings.Cut(digest, ":")
	if !ok || len(payload) != 128 || !isLowerHex(payload) {
		return false
	}
	return prefix == "b4" || prefix == "b5" || prefix == "shake256"
}

func isLowerHex(value string) bool {
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
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
		"lerna.json.bak",
		"my-lerna.json",
		"rush.json.bak",
		"Rush.json",
		"pants.toml.bak",
		"Pants.toml",
		"my-pants.toml",
		"gitmodules",
		".gitmodule",
		"turbo.json.bak",
		"turbo.jsonc",
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
		"shell.nix.bak",
		"myshell.nix",
		"Shell.nix",
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
		"jsonnetfile.json",
		"package.yaml",
		"vcpkg.json",
		"Manifest.toml",
		"Package.resolved",
		"index.html",
		"job.tf",
		"app.js",
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
			name: "crystal shard",
			root: filepath.Join("..", "..", "testdata", "crystal", "shard"),
			path: "shard.yml",
			typ:  DetectorID("crystal-shard"),
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
			name: "pubspec lock",
			root: filepath.Join("..", "..", "testdata", "yaml", "pubspec-lock-with-deps"),
			path: "pubspec.lock",
			typ:  DetectorID("dart-pubspec-lock"),
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
	if result.Sources[0].Dependencies != nil {
		t.Fatalf("expected no extracted dependencies, got %+v", result.Sources[0].Dependencies)
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
	if result.Sources[0].Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Sources[0].Analysis)
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

func TestJsonnetLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	type gitSource struct {
		Remote string `json:"remote"`
		Subdir string `json:"subdir"`
	}
	type lockDependency struct {
		Name   string `json:"name"`
		Source struct {
			Git   *gitSource `json:"git"`
			Local *struct {
				Directory string `json:"directory"`
			} `json:"local"`
		} `json:"source"`
		Version string `json:"version"`
		Sum     string `json:"sum"`
		Single  bool   `json:"single"`
	}
	type lockFile struct {
		Version       int              `json:"version"`
		Dependencies  []lockDependency `json:"dependencies"`
		LegacyImports *bool            `json:"legacyImports"`
	}

	type expectedDependency struct {
		name, remote, subdir, directory, version, sum string
		single                                        bool
	}
	for _, fixture := range []struct {
		name          string
		legacyImports bool
		dependencies  []expectedDependency
	}{
		{name: "lock", dependencies: []expectedDependency{{remote: "https://github.com/jsonnet-bundler/jsonnet-bundler.git", version: "080f157c7fb85ad0281ea78f6c641eaa570a582f", sum: "W1uI550rQ66axRpPXA2EZDquyPg/5PHZlvUz1NEzefg="}}},
		{name: "lock-root", dependencies: []expectedDependency{{remote: "https://github.com/jsonnet-libs/xtd.git", version: "803739029925cf31b0e3c6db2f4aae09b0378a6e", sum: "d/c+3om56mfddeYWrsxOwsrlH008BmX/5NoquXMj0+g="}}},
		{name: "lock-multiple", dependencies: []expectedDependency{
			{remote: "https://github.com/grafana/jsonnet-libs.git", subdir: "ksonnet-util", version: "610b00d219d0a6f3d833dd44e4bb0deda2429da0", sum: "XdIrw3m7I8fJ3CL9eR8LtuYcanf2QK78n4H4OBBOADc="},
			{remote: "https://github.com/jsonnet-bundler/frozen-lib.git", version: "9f40207f668e382b706e1822f2d46ce2cd0a57cc", sum: "qUJDskVRtmkTms2udvFpLi1t5YKVbGmMSyiZnPjXsMo="},
		}},
		{name: "lock-named-dependency", dependencies: []expectedDependency{{name: "prometheus", remote: "https://github.com/prometheus/prometheus.git", subdir: "documentation/prometheus-mixin", version: "7c039a6b3b4b2a9d7c613ac8bd3fc16e8ca79684", sum: "bVGOsq3hLOw2irNPAS91a5dZJqQlBUNWy3pVwM4+kIY="}}},
		{name: "lock-kube-prometheus", dependencies: []expectedDependency{{remote: "https://github.com/grafana/loki.git", subdir: "production/ksonnet/loki", version: "bd4d516262c107a0bde7a962fa2b1e567a2c21e5", sum: "ExovUKXmZ4KwJAv/q8ZwNW9BdIZlrxmoGrne7aR64wo=", single: true}}},
		{name: "lock-legacy-imports", legacyImports: true, dependencies: []expectedDependency{{remote: "https://github.com/jsonnet-bundler/frozen-lib.git", version: "9f40207f668e382b706e1822f2d46ce2cd0a57cc", sum: "qUJDskVRtmkTms2udvFpLi1t5YKVbGmMSyiZnPjXsMo="}}},
		{name: "lock-local", dependencies: []expectedDependency{{directory: "jsonnet/overlays"}}},
		{name: "lock-ssh", dependencies: []expectedDependency{{remote: "ssh://git@github.com/jsonnet-bundler/frozen-lib.git", version: "9f40207f668e382b706e1822f2d46ce2cd0a57cc", sum: "qUJDskVRtmkTms2udvFpLi1t5YKVbGmMSyiZnPjXsMo="}}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "jsonnet", fixture.name, "jsonnetfile.lock.json")
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			var lock lockFile
			if err := json.Unmarshal(contents, &lock); err != nil {
				t.Fatalf("parse lockfile: %v", err)
			}
			if lock.Version != 1 || len(lock.Dependencies) != len(fixture.dependencies) {
				t.Fatalf("unexpected lockfile metadata: %+v", lock)
			}
			if lock.LegacyImports == nil || *lock.LegacyImports != fixture.legacyImports {
				t.Fatalf("unexpected legacyImports value: %+v", lock.LegacyImports)
			}

			for index, dependency := range lock.Dependencies {
				expected := fixture.dependencies[index]
				if dependency.Name != expected.name || dependency.Version != expected.version || dependency.Sum != expected.sum || dependency.Single != expected.single {
					t.Fatalf("unexpected dependency[%d]: %+v", index, dependency)
				}
				switch {
				case dependency.Source.Git != nil && dependency.Source.Local == nil:
					if dependency.Source.Git.Remote != expected.remote || dependency.Source.Git.Subdir != expected.subdir {
						t.Fatalf("invalid Git source: %+v", dependency.Source.Git)
					}
					if !(strings.HasPrefix(dependency.Source.Git.Remote, "https://") || strings.HasPrefix(dependency.Source.Git.Remote, "ssh://git@")) || !strings.HasSuffix(dependency.Source.Git.Remote, ".git") {
						t.Fatalf("unexpected Git remote format: %q", dependency.Source.Git.Remote)
					}
					if len(dependency.Version) != 40 || !isLowerHex(dependency.Version) {
						t.Fatalf("expected resolved Git commit, got %q", dependency.Version)
					}
					sum, err := base64.StdEncoding.DecodeString(dependency.Sum)
					if err != nil || len(sum) != 32 {
						t.Fatalf("expected SHA-256 integrity sum, got %q: %v", dependency.Sum, err)
					}
				case dependency.Source.Git == nil && dependency.Source.Local != nil:
					if dependency.Source.Local.Directory != expected.directory || dependency.Version != "" || dependency.Sum != "" {
						t.Fatalf("invalid local dependency: %+v", dependency)
					}
				default:
					t.Fatalf("expected exactly one dependency source: %+v", dependency.Source)
				}
			}
			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "jsonnet-lock" || result.Sources[0].Path != "jsonnetfile.lock.json" {
				t.Fatalf("expected jsonnet-lock source, got %+v", result.Sources)
			}
			if result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", result.Sources[0].Analysis)
			}
		})
	}
}

func TestEmacsCaskFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name         string
		forms        []string
		block        string
		relatedFile  string
		relatedForms []string
	}{
		{name: "cask", forms: []string{"(source melpa)", "(source gnu)", "(depends-on \"magit\")", "(depends-on \"company\")"}},
		{name: "cask-archive-versions", forms: []string{"(package \"fixture-archive\" \"0.1.0\" \"Archive dependency fixture.\")", "(source gnu)", "(source melpa-stable)", "(depends-on \"dash\" \"2.19.1\")", "(depends-on \"s\" \"1.12.0\")", "(depends-on \"compat\")"}},
		{name: "cask-git-options", forms: []string{"(source melpa)", "(depends-on \"consult\" :git \"https://github.com/minad/consult.git\" :ref \"e3418f7a7f111a8aa2d3c967e9a5d1c60795b175\")", "(depends-on \"transient\" :git \"https://github.com/magit/transient.git\" :branch \"main\")", "(depends-on \"package-build\" :git \"https://github.com/melpa/package-build.git\" :files (\"*.el\" (:exclude \"test\")))"}},
		{name: "cask-vcs-fetchers", forms: []string{"(depends-on \"emacs-bzr\" :bzr \"https://bazaar.launchpad.net/~emacs-variants/emacs/trunk/\")", "(depends-on \"mercurial\" :hg \"https://www.mercurial-scm.org/repo/hg\" :branch \"stable\")", "(depends-on \"darcs\" :darcs \"http://darcs.net/\" :ref \"release-2\")", "(depends-on \"subversion\" :svn \"https://svn.apache.org/repos/asf/subversion/trunk\")", "(depends-on \"emacs-cvs\" :cvs \":pserver:anonymous@cvs.savannah.gnu.org:/sources/emacs\")"}},
		{name: "cask-development", forms: []string{"(source melpa)", "(depends-on \"use-package\" \"2.4.6\")"}, block: "(development\n (depends-on \"buttercup\" \"1.36.0\")\n (depends-on \"ecukes\")\n (depends-on \"ert-async\" \"0.1.2\"))"},
		{name: "cask-custom-source", forms: []string{"(source \"gnu-mirror\" \"https://elpa.gnu.org/packages/\")", "(source melpa)", "(depends-on \"dash\" \"2.19.1\")", "(depends-on \"company\" \"1.0.2\")"}},
		{name: "cask-vcs-default", forms: []string{"(package \"fixture-vcs-default\" \"0.1.0\" \"Default VCS branch fixture.\")", "(depends-on \"magit\" :git \"https://github.com/magit/magit.git\")", "(depends-on \"diff-hl\" :git \"ssh://git@github.com/dgutov/diff-hl.git\" :files (\"*.el\" \"README.md\"))"}},
		{name: "cask-package-file", forms: []string{"(source gnu)", "(package-file \"fixture.el\")"}, relatedFile: "fixture.el", relatedForms: []string{"Package-Requires: ((dash \"2.19.1\") (s \"1.12.0\"))"}},
		{name: "cask-package-descriptor", forms: []string{"(source gnu)", "(package-descriptor \"descriptor-pkg-pkg.el\")"}, relatedFile: "descriptor-pkg-pkg.el", relatedForms: []string{"(define-package \"descriptor-pkg\" \"0.1.0\"", "'((dash \"2.19.1\") (compat \"29.1.4.4\"))"}},
		{name: "cask-files-layout", forms: []string{"(depends-on \"package-build\" :git \"https://github.com/melpa/package-build.git\" :files (:defaults \"*.el\" (\"resources\" (\"snippets\" \"*.snippet\"))))"}},
		{name: "cask-all-built-in-sources", forms: []string{"(source marmalade)", "(source SC)", "(source org)", "(depends-on \"org-plus-contrib\")"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "emacs", fixture.name, "Cask")
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			text := string(contents)
			position := 0
			for _, form := range fixture.forms {
				index := strings.Index(text[position:], form)
				if index < 0 {
					t.Fatalf("fixture missing ordered form %q", form)
				}
				position += index + len(form)
			}
			if fixture.block != "" && !strings.Contains(text, fixture.block) {
				t.Fatalf("fixture missing nested form block %q", fixture.block)
			}
			if strings.Count(text, "(") != strings.Count(text, ")") {
				t.Fatalf("unbalanced Lisp forms in %s", path)
			}
			if fixture.relatedFile != "" {
				related, err := os.ReadFile(filepath.Join(filepath.Dir(path), fixture.relatedFile))
				if err != nil {
					t.Fatalf("read indirect package metadata: %v", err)
				}
				for _, form := range fixture.relatedForms {
					if !strings.Contains(string(related), form) {
						t.Fatalf("indirect metadata missing %q", form)
					}
				}
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "emacs-cask" || result.Sources[0].Path != "Cask" {
				t.Fatalf("expected emacs-cask source, got %+v", result.Sources)
			}
			if result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", result.Sources[0].Analysis)
			}
		})
	}
}

func TestIOSPodspecFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name         string
		file         string
		dependencies []string
		contexts     []string
	}{
		{name: "podspec", file: "demo.podspec", dependencies: []string{"s.dependency \"Alamofire\", \"~> 5.0\""}},
		{name: "podspec-constraints", file: "constraints.podspec", dependencies: []string{"s.dependency 'Alamofire'", "s.dependency 'AFNetworking', '= 4.0.1'", "s.dependency 'RxSwift', '~> 6.5'", "s.dependency 'Quick', '>= 7.0', '< 8.0'", "s.dependency('Nimble', '~> 12.0')"}},
		{name: "podspec-platforms", file: "platforms.podspec", dependencies: []string{"s.ios.dependency 'MBProgressHUD', '~> 1.2'", "s.osx.dependency 'Sparkle', '~> 2.5'", "s.macos.dependency 'Sparkle', '~> 2.5'", "s.tvos.dependency 'TVVLCKit', '~> 3.5'", "s.watchos.dependency 'KeychainAccess', '~> 4.2'", "s.visionos.dependency 'Kingfisher', '~> 7.0'"}},
		{name: "podspec-subspecs", file: "subspecs.podspec", dependencies: []string{"core.dependency 'FixtureSubspecs/Support'", "core.dependency 'Alamofire/NetworkActivityIndicator', '~> 5.9'", "support.dependency 'Reachability', '~> 3.2'"}, contexts: []string{"s.subspec 'Core' do |core|", "s.subspec 'Support' do |support|"}},
		{name: "podspec-test-app-specs", file: "test-app-specs.podspec", dependencies: []string{"test_spec.dependency 'Expecta', '~> 1.0'", "test_spec.dependency 'OCMock', '>= 3.9'", "app_spec.dependency 'AFNetworking', '~> 4.0'"}, contexts: []string{"s.test_spec 'UnitTests' do |test_spec|", "s.app_spec 'DemoApp' do |app_spec|"}},
		{name: "podspec-configurations", file: "configurations.podspec", dependencies: []string{"s.dependency 'FLEX', '~> 5.22', :configurations => ['Debug']", "s.dependency 'CocoaLumberjack/Swift', '>= 3.8', :configurations => :debug"}},
		{name: "podspec-nested-platform", file: "nested-platform.podspec", dependencies: []string{"ui.ios.dependency 'SDWebImage', '~> 5.19'", "ui.osx.dependency 'PromiseKit', '~> 6.18'"}, contexts: []string{"s.subspec 'Feature' do |feature|", "feature.subspec 'UI' do |ui|"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "ios", fixture.name, fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			text := string(contents)
			for _, rootField := range []string{"Pod::Spec.new do", "s.name", "s.version", "s.summary", "s.authors", "s.license", "s.homepage", "s.source", "https://github.com/CocoaPods/CocoaPods.git", ":tag => s.version.to_s"} {
				if !strings.Contains(text, rootField) {
					t.Fatalf("missing required podspec root field %q", rootField)
				}
			}
			if strings.Count(text, "Pod::Spec.new do") != 1 {
				t.Fatalf("expected exactly one root specification")
			}
			if strings.Count(text, ".dependency") != len(fixture.dependencies) {
				t.Fatalf("expected %d dependency calls, got %d", len(fixture.dependencies), strings.Count(text, ".dependency"))
			}
			position := 0
			for _, dependency := range fixture.dependencies {
				index := strings.Index(text[position:], dependency)
				if index < 0 {
					t.Fatalf("missing ordered dependency call %q", dependency)
				}
				position += index + len(dependency)
			}
			for _, context := range fixture.contexts {
				if !strings.Contains(text, context) {
					t.Fatalf("missing dependency scope %q", context)
				}
			}
			blocks, ends := 0, 0
			for _, line := range strings.Split(text, "\n") {
				if strings.Contains(line, " do |") {
					blocks++
				}
				if strings.TrimSpace(line) == "end" {
					ends++
				}
			}
			if blocks != ends {
				t.Fatalf("unbalanced Ruby blocks in %s", path)
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "ios-podspec" || result.Sources[0].Path != fixture.file {
				t.Fatalf("expected ios-podspec source, got %+v", result.Sources)
			}
			if result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", result.Sources[0].Analysis)
			}
		})
	}
}

func TestUnrealUprojectFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name    string
		file    string
		plugins []map[string]any
		modules []map[string]any
	}{
		{name: "uproject", file: "Game.uproject", plugins: []map[string]any{{"Name": "OnlineSubsystem", "Enabled": true}}},
		{name: "uproject-multiple", file: "PluginMatrix.uproject", plugins: []map[string]any{{"Name": "EnhancedInput", "Enabled": true}, {"Name": "OnlineSubsystemSteam", "Enabled": true}, {"Name": "SteamVR", "Enabled": false}}},
		{name: "uproject-optional", file: "OptionalPlugin.uproject", plugins: []map[string]any{{"Name": "ModelingToolsEditorMode", "Enabled": true, "Optional": true, "Description": "Optional modeling tools dependency.", "MarketplaceURL": "com.epicgames.launcher://ue/marketplace/product/example", "RequestedVersion": float64(2)}}},
		{name: "uproject-platforms", file: "PlatformPlugins.uproject", plugins: []map[string]any{{"Name": "OculusXR", "Enabled": true, "PlatformAllowList": []any{"Win64", "Android"}, "SupportedTargetPlatforms": []any{"Win64", "Android"}}, {"Name": "AppleARKit", "Enabled": true, "PlatformDenyList": []any{"Win64", "Linux"}}, {"Name": "PlatformExtension", "Enabled": true, "HasExplicitPlatforms": true, "PlatformAllowList": []any{}, "SupportedTargetPlatforms": []any{}}}},
		{name: "uproject-targets", file: "TargetPlugins.uproject", plugins: []map[string]any{{"Name": "GameplayAbilities", "Enabled": true, "TargetAllowList": []any{"Game", "Editor"}, "TargetDenyList": []any{"Server"}, "TargetConfigurationAllowList": []any{"Development", "Shipping"}, "TargetConfigurationDenyList": []any{"Debug"}, "TargetsToExplicitlyLoad": []any{"Game"}}}},
		{name: "uproject-explicit-engine", file: "ExplicitEnginePlugins.uproject", plugins: []map[string]any{{"Name": "Niagara", "Enabled": true}, {"Name": "Paper2D", "Enabled": true, "Optional": false}}},
		{name: "uproject-modules", file: "ModuleDependencies.uproject", plugins: []map[string]any{{"Name": "EnhancedInput", "Enabled": true}}, modules: []map[string]any{{"Name": "FixtureGame", "Type": "Runtime", "LoadingPhase": "Default", "AdditionalDependencies": []any{"Core", "CoreUObject", "Engine", "EnhancedInput"}}}},
		{name: "uproject-game-feature", file: "GameFeaturePlugin.uproject", plugins: []map[string]any{{"Name": "GameFeatures", "Enabled": true, "Activate": true}}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "unreal", fixture.name, fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var project struct {
				FileVersion                   int              `json:"FileVersion"`
				EngineAssociation             string           `json:"EngineAssociation"`
				Category                      string           `json:"Category"`
				Description                   string           `json:"Description"`
				DisableEnginePluginsByDefault *bool            `json:"DisableEnginePluginsByDefault"`
				Plugins                       []map[string]any `json:"Plugins"`
				Modules                       []map[string]any `json:"Modules"`
			}
			if err := json.Unmarshal(contents, &project); err != nil {
				t.Fatalf("parse project descriptor: %v", err)
			}
			if project.FileVersion != 3 || project.EngineAssociation == "" || project.Category == "" || project.Description == "" {
				t.Fatalf("invalid project descriptor metadata: %+v", project)
			}
			if fixture.name == "uproject-explicit-engine" && (project.DisableEnginePluginsByDefault == nil || !*project.DisableEnginePluginsByDefault) {
				t.Fatalf("missing explicit engine plugin setting: %+v", project)
			}
			if !reflect.DeepEqual(project.Plugins, fixture.plugins) {
				t.Fatalf("unexpected plugin references: got %#v want %#v", project.Plugins, fixture.plugins)
			}
			if !reflect.DeepEqual(project.Modules, fixture.modules) {
				t.Fatalf("unexpected project modules: got %#v want %#v", project.Modules, fixture.modules)
			}
			validTargets := map[string]bool{"Game": true, "Editor": true, "Client": true, "Server": true, "Program": true}
			validConfigurations := map[string]bool{"Debug": true, "DebugGame": true, "Development": true, "Shipping": true, "Test": true}
			for _, plugin := range project.Plugins {
				name, hasName := plugin["Name"].(string)
				if !hasName || name == "" {
					t.Fatalf("plugin reference has no name: %#v", plugin)
				}
				if _, hasEnabled := plugin["Enabled"].(bool); !hasEnabled {
					t.Fatalf("plugin reference has no boolean Enabled field: %#v", plugin)
				}
				for _, field := range []string{"PlatformAllowList", "PlatformDenyList", "SupportedTargetPlatforms", "TargetAllowList", "TargetDenyList", "TargetConfigurationAllowList", "TargetConfigurationDenyList", "TargetsToExplicitlyLoad"} {
					value, exists := plugin[field]
					if !exists {
						continue
					}
					values, ok := value.([]any)
					if !ok {
						t.Fatalf("plugin %q field %q is not an array: %#v", name, field, value)
					}
					for _, item := range values {
						stringItem, ok := item.(string)
						if !ok {
							t.Fatalf("plugin %q field %q contains a non-string: %#v", name, field, item)
						}
						if (field == "TargetAllowList" || field == "TargetDenyList" || field == "TargetsToExplicitlyLoad") && !validTargets[stringItem] {
							t.Fatalf("plugin %q field %q has unknown target %q", name, field, stringItem)
						}
						if (field == "TargetConfigurationAllowList" || field == "TargetConfigurationDenyList") && !validConfigurations[stringItem] {
							t.Fatalf("plugin %q field %q has unknown configuration %q", name, field, stringItem)
						}
					}
				}
			}
			if fixture.name == "uproject-game-feature" {
				if activated, ok := project.Plugins[0]["Activate"].(bool); !ok || !activated {
					t.Fatalf("expected Game Feature plugin activation, got %#v", project.Plugins[0]["Activate"])
				}
			}
			for _, module := range project.Modules {
				if module["Name"] == "" || module["Type"] != "Runtime" {
					t.Fatalf("invalid project module: %#v", module)
				}
				dependencies, ok := module["AdditionalDependencies"].([]any)
				if !ok || len(dependencies) == 0 {
					t.Fatalf("module has no additional dependencies: %#v", module)
				}
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "unreal-uproject" || result.Sources[0].Path != fixture.file {
				t.Fatalf("expected unreal-uproject source, got %+v", result.Sources)
			}
			if result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", result.Sources[0].Analysis)
			}
		})
	}
}

func TestUnrealUpluginFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	for _, fixture := range []struct {
		name             string
		file             string
		pluginNames      []string
		moduleDeps       map[string][]string
		pythonPackages   map[string][]string
		pythonHashCounts map[string][]int
		extraIndexes     map[string][]string
	}{
		{name: "uplugin", file: "MyPlugin.uplugin", moduleDeps: map[string][]string{"MyPlugin": nil}},
		{name: "uplugin-plugin-references", file: "ReferencePlugin.uplugin", pluginNames: []string{"EnhancedInput", "OnlineSubsystem", "GameFeatures"}, moduleDeps: map[string][]string{"ReferencePlugin": nil}},
		{name: "uplugin-module-dependencies", file: "ModuleDependencies.uplugin", moduleDeps: map[string][]string{"ModuleDependencies": {"Core", "CoreUObject", "Engine", "EnhancedInput"}, "ModuleDependenciesEditor": {"UnrealEd", "ToolMenus"}}},
		{name: "uplugin-module-platforms", file: "PlatformModule.uplugin", moduleDeps: map[string][]string{"PlatformModule": {"Core", "Projects"}, "ExtensionModule": {"Core"}}},
		{name: "uplugin-python-requirements", file: "PythonDependencies.uplugin", moduleDeps: map[string][]string{"PythonDependencies": nil}, pythonPackages: map[string][]string{"All": {"numpy==1.24.4", "requests==2.32.3"}}, pythonHashCounts: map[string][]int{"All": {12, 1}}},
		{name: "uplugin-python-platform-index", file: "PythonPlatformDependencies.uplugin", moduleDeps: map[string][]string{"PythonPlatformDependencies": nil}, pythonPackages: map[string][]string{"Linux": {"torch==2.1.0+cu118"}, "Mac": {"torch==2.1.0"}, "Win64": {"requests==2.32.3"}}, pythonHashCounts: map[string][]int{"Linux": {3}, "Mac": {8}, "Win64": {1}}, extraIndexes: map[string][]string{"Linux": {"https://download.pytorch.org/whl/"}, "Mac": {"https://download.pytorch.org/whl/"}}},
		{name: "uplugin-plugin-targets", file: "TargetedReferences.uplugin", pluginNames: []string{"OculusXR", "PlatformExtension"}, moduleDeps: map[string][]string{"TargetedReferences": nil}},
		{name: "uplugin-root-explicit-platforms", file: "ExplicitPlatformPlugin.uplugin", moduleDeps: map[string][]string{"ExplicitPlatformPlugin": {"Core"}}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "unreal", fixture.name, fixture.file)
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var plugin struct {
				FileVersion        int              `json:"FileVersion"`
				Version            int              `json:"Version"`
				VersionName        string           `json:"VersionName"`
				FriendlyName       string           `json:"FriendlyName"`
				Description        string           `json:"Description"`
				Category           string           `json:"Category"`
				Modules            []map[string]any `json:"Modules"`
				Plugins            []map[string]any `json:"Plugins"`
				PythonRequirements []struct {
					Platform       string   `json:"Platform"`
					Requirements   []string `json:"Requirements"`
					ExtraIndexURLs []string `json:"ExtraIndexUrls"`
				} `json:"PythonRequirements"`
			}
			if err := json.Unmarshal(contents, &plugin); err != nil {
				t.Fatalf("parse plugin descriptor: %v", err)
			}
			var raw map[string]any
			if err := json.Unmarshal(contents, &raw); err != nil {
				t.Fatalf("parse raw plugin descriptor: %v", err)
			}
			if plugin.FileVersion != 3 || plugin.Version < 1 || plugin.VersionName == "" || plugin.FriendlyName == "" || plugin.Description == "" || plugin.Category == "" || len(plugin.Modules) == 0 {
				t.Fatalf("invalid plugin descriptor metadata: %+v", plugin)
			}
			var pluginNames []string
			for _, reference := range plugin.Plugins {
				name, hasName := reference["Name"].(string)
				if !hasName || name == "" {
					t.Fatalf("plugin reference has no name: %#v", reference)
				}
				if _, hasEnabled := reference["Enabled"].(bool); !hasEnabled {
					t.Fatalf("plugin reference has no boolean Enabled field: %#v", reference)
				}
				pluginNames = append(pluginNames, name)
			}
			if !slices.Equal(pluginNames, fixture.pluginNames) {
				t.Fatalf("unexpected referenced plugins: got %#v want %#v", pluginNames, fixture.pluginNames)
			}
			if fixture.name == "uplugin-plugin-references" && !reflect.DeepEqual(plugin.Plugins, []map[string]any{{"Name": "EnhancedInput", "Enabled": true}, {"Name": "OnlineSubsystem", "Enabled": true, "Optional": true, "RequestedVersion": float64(1)}, {"Name": "GameFeatures", "Enabled": true, "Activate": true}}) {
				t.Fatalf("unexpected plugin reference metadata: %#v", plugin.Plugins)
			}
			if fixture.name == "uplugin-plugin-targets" && !reflect.DeepEqual(plugin.Plugins, []map[string]any{{"Name": "OculusXR", "Enabled": true, "PlatformAllowList": []any{"Win64"}, "PlatformDenyList": []any{"Linux"}, "SupportedTargetPlatforms": []any{"Win64"}, "TargetAllowList": []any{"Game", "Editor"}, "TargetDenyList": []any{"Server"}, "TargetConfigurationAllowList": []any{"Development", "Shipping"}, "TargetConfigurationDenyList": []any{"Debug"}, "TargetsToExplicitlyLoad": []any{"Game"}}, {"Name": "PlatformExtension", "Enabled": true, "HasExplicitPlatforms": true, "PlatformAllowList": []any{}, "SupportedTargetPlatforms": []any{}}}) {
				t.Fatalf("unexpected targeted plugin reference metadata: %#v", plugin.Plugins)
			}

			moduleDeps := make(map[string][]string, len(plugin.Modules))
			for _, module := range plugin.Modules {
				name, hasName := module["Name"].(string)
				moduleType, hasType := module["Type"].(string)
				if !hasName || name == "" || !hasType || moduleType == "" {
					t.Fatalf("invalid module descriptor: %#v", module)
				}
				moduleDeps[name] = nil
				dependencies, exists := module["AdditionalDependencies"]
				if !exists {
					continue
				}
				items, ok := dependencies.([]any)
				if !ok {
					t.Fatalf("module dependencies are not an array: %#v", dependencies)
				}
				for _, item := range items {
					dependency, ok := item.(string)
					if !ok || dependency == "" {
						t.Fatalf("invalid module dependency: %#v", item)
					}
					moduleDeps[name] = append(moduleDeps[name], dependency)
				}
			}
			if !reflect.DeepEqual(moduleDeps, fixture.moduleDeps) {
				t.Fatalf("unexpected module dependencies: got %#v want %#v", moduleDeps, fixture.moduleDeps)
			}
			if fixture.name == "uplugin-module-platforms" && !reflect.DeepEqual(plugin.Modules, []map[string]any{{"Name": "PlatformModule", "Type": "Runtime", "LoadingPhase": "Default", "PlatformAllowList": []any{"Win64", "Linux"}, "AdditionalDependencies": []any{"Core", "Projects"}}, {"Name": "ExtensionModule", "Type": "Runtime", "LoadingPhase": "None", "HasExplicitPlatforms": true, "PlatformAllowList": []any{}, "AdditionalDependencies": []any{"Core"}}}) {
				t.Fatalf("unexpected module platform metadata: %#v", plugin.Modules)
			}
			if fixture.name == "uplugin-root-explicit-platforms" {
				if explicit, ok := raw["HasExplicitPlatforms"].(bool); !ok || !explicit || !reflect.DeepEqual(raw["SupportedTargetPlatforms"], []any{}) {
					t.Fatalf("unexpected root explicit-platform metadata: %#v", raw)
				}
			}

			pythonPackages, extraIndexes := map[string][]string{}, map[string][]string{}
			pythonHashCounts := map[string][]int{}
			for _, requirement := range plugin.PythonRequirements {
				if requirement.Platform != "All" && requirement.Platform != "Linux" && requirement.Platform != "Mac" && requirement.Platform != "Win64" {
					t.Fatalf("unexpected Python requirement platform %q", requirement.Platform)
				}
				if len(requirement.Requirements) == 0 {
					t.Fatalf("Python requirement group %q is empty", requirement.Platform)
				}
				for _, dependency := range requirement.Requirements {
					fields := strings.Fields(dependency)
					if len(fields) < 2 || !strings.Contains(fields[0], "==") {
						t.Fatalf("expected pinned hashed Python requirement, got %q", dependency)
					}
					hashCount := 0
					for _, field := range fields[1:] {
						if !strings.HasPrefix(field, "--hash=sha256:") {
							t.Fatalf("unexpected Python requirement option %q", field)
						}
						hash := strings.TrimPrefix(field, "--hash=sha256:")
						if len(hash) != 64 || !isLowerHex(hash) {
							t.Fatalf("invalid SHA-256 requirement hash %q", hash)
						}
						hashCount++
					}
					pythonPackages[requirement.Platform] = append(pythonPackages[requirement.Platform], fields[0])
					pythonHashCounts[requirement.Platform] = append(pythonHashCounts[requirement.Platform], hashCount)
				}
				if requirement.ExtraIndexURLs != nil {
					extraIndexes[requirement.Platform] = requirement.ExtraIndexURLs
				}
			}
			wantPythonPackages, wantHashCounts, wantExtraIndexes := fixture.pythonPackages, fixture.pythonHashCounts, fixture.extraIndexes
			if wantPythonPackages == nil {
				wantPythonPackages = map[string][]string{}
			}
			if wantHashCounts == nil {
				wantHashCounts = map[string][]int{}
			}
			if wantExtraIndexes == nil {
				wantExtraIndexes = map[string][]string{}
			}
			if !reflect.DeepEqual(pythonPackages, wantPythonPackages) || !reflect.DeepEqual(pythonHashCounts, wantHashCounts) || !reflect.DeepEqual(extraIndexes, wantExtraIndexes) {
				t.Fatalf("unexpected Python dependencies: packages=%#v hash-counts=%#v indexes=%#v", pythonPackages, pythonHashCounts, extraIndexes)
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "unreal-uplugin" || result.Sources[0].Path != fixture.file {
				t.Fatalf("expected unreal-uplugin source, got %+v", result.Sources)
			}
			if result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("expected selector-only analysis, got %+v", result.Sources[0].Analysis)
			}
		})
	}
}

func TestFoundryTomlFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name         string
		dependencies []string
		shorthand    []string
		legacy       []string
	}{
		{name: "toml"},
		{name: "toml-registry", dependencies: []string{"@openzeppelin-contracts", "@uniswap-universal-router", "forge-std"}},
		{name: "toml-url", dependencies: []string{"solady", "openzeppelin"}},
		{name: "toml-legacy-sdependencies", legacy: []string{"@openzeppelin~v4.9.3", "@solady~v0.0.47"}},
		{name: "toml-soldeer-config", dependencies: []string{"forge-std"}},
		{name: "toml-empty-dependencies"},
		{name: "toml-registry-shorthand", shorthand: []string{"forge-std", "@openzeppelin-contracts"}},
		{name: "toml-git", dependencies: []string{"solmate", "solady", "forge-std"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "foundry", fixture.name, "foundry.toml")
			var config map[string]any
			if _, err := toml.DecodeFile(path, &config); err != nil {
				t.Fatalf("parse foundry config: %v", err)
			}
			profile, ok := config["profile"].(map[string]any)
			if !ok || profile["default"] == nil {
				t.Fatalf("missing default profile: %#v", config)
			}
			if len(fixture.dependencies) > 0 {
				dependencies, ok := config["dependencies"].(map[string]any)
				if !ok {
					t.Fatalf("missing dependencies table: %#v", config)
				}
				for _, name := range fixture.dependencies {
					entry, ok := dependencies[name].(map[string]any)
					if !ok || entry["version"] == "" {
						t.Fatalf("invalid dependency %q: %#v", name, dependencies[name])
					}
				}
			}
			if len(fixture.dependencies) > 0 || len(fixture.shorthand) > 0 || fixture.name == "toml-empty-dependencies" {
				libs := profile["default"].(map[string]any)["libs"].([]any)
				if !slices.ContainsFunc(libs, func(value any) bool { return value == "dependencies" }) {
					t.Fatalf("Soldeer config must include dependencies lib: %#v", libs)
				}
			}
			if len(fixture.shorthand) > 0 {
				dependencies := config["dependencies"].(map[string]any)
				for _, name := range fixture.shorthand {
					if version, ok := dependencies[name].(string); !ok || version == "" {
						t.Fatalf("invalid shorthand dependency %q: %#v", name, dependencies[name])
					}
				}
			}
			if fixture.name == "toml-git" {
				for _, entry := range config["dependencies"].(map[string]any) {
					dependency := entry.(map[string]any)
					git, _ := dependency["git"].(string)
					if !strings.HasPrefix(git, "https://") || !strings.HasSuffix(git, ".git") {
						t.Fatalf("invalid Git dependency: %#v", dependency)
					}
					refs := 0
					for _, key := range []string{"rev", "branch", "tag"} {
						if dependency[key] != nil {
							refs++
						}
					}
					if refs != 1 {
						t.Fatalf("expected exactly one Git ref: %#v", dependency)
					}
				}
			}
			if fixture.name == "toml-empty-dependencies" {
				if dependencies, ok := config["dependencies"].(map[string]any); !ok || len(dependencies) != 0 {
					t.Fatalf("expected explicit empty dependencies table: %#v", config["dependencies"])
				}
			}
			if len(fixture.legacy) > 0 {
				dependencies, ok := config["sdependencies"].(map[string]any)
				if !ok {
					t.Fatalf("missing legacy dependencies: %#v", config)
				}
				for _, name := range fixture.legacy {
					url, ok := dependencies[name].(string)
					if !ok || !strings.HasPrefix(url, "https://") || !strings.HasSuffix(url, ".zip") {
						t.Fatalf("invalid legacy dependency %q: %#v", name, dependencies[name])
					}
				}
			}
			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "foundry-toml" || result.Sources[0].Path != "foundry.toml" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestSoldeerLockFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name         string
		dependencies []string
		entryKinds   []string
	}{
		{name: "soldeer-lock-single", dependencies: []string{"forge-std@1.11.0"}, entryKinds: []string{"registry"}},
		{name: "soldeer-lock-scoped", dependencies: []string{"@openzeppelin-contracts@5.5.0"}, entryKinds: []string{"registry"}},
		{name: "soldeer-lock-multiple", dependencies: []string{"@openzeppelin-contracts-upgradeable@5.5.0", "forge-std@1.11.0"}, entryKinds: []string{"registry", "registry"}},
		{name: "soldeer-lock-major-version", dependencies: []string{"uniswap-v4-core@4"}, entryKinds: []string{"registry"}},
		{name: "soldeer-lock-custom-url", dependencies: []string{"forge-std-custom@1.9.2"}, entryKinds: []string{"custom-url"}},
		{name: "soldeer-lock-git-https", dependencies: []string{"solmate@6.8.0"}, entryKinds: []string{"git"}},
		{name: "soldeer-lock-git-ssh", dependencies: []string{"forge-std@1.9.2"}, entryKinds: []string{"git"}},
		{name: "soldeer-lock-private", dependencies: []string{"private-contracts@1.0.0"}, entryKinds: []string{"private"}},
		{name: "soldeer-lock-empty"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "foundry", fixture.name, "soldeer.lock")
			var lock struct {
				Dependencies []struct {
					Name      string `toml:"name"`
					Version   string `toml:"version"`
					URL       string `toml:"url"`
					Checksum  string `toml:"checksum"`
					Integrity string `toml:"integrity"`
					Git       string `toml:"git"`
					Rev       string `toml:"rev"`
				} `toml:"dependencies"`
			}
			if _, err := toml.DecodeFile(path, &lock); err != nil {
				t.Fatalf("parse Soldeer lockfile: %v", err)
			}
			var got, kinds []string
			for _, dependency := range lock.Dependencies {
				if dependency.Name == "" || dependency.Version == "" {
					t.Fatalf("invalid resolved dependency: %#v", dependency)
				}
				switch {
				case dependency.URL != "":
					if !strings.HasPrefix(dependency.URL, "https://") || !strings.HasSuffix(dependency.URL, ".zip") || dependency.Git != "" || dependency.Rev != "" {
						t.Fatalf("invalid HTTP lock entry: %#v", dependency)
					}
					if strings.Contains(dependency.URL, "soldeer-revisions.s3.amazonaws.com") {
						kinds = append(kinds, "registry")
					} else {
						kinds = append(kinds, "custom-url")
					}
				case dependency.Git != "":
					if !(strings.HasPrefix(dependency.Git, "https://") || strings.HasPrefix(dependency.Git, "git@")) || !strings.HasSuffix(dependency.Git, ".git") || len(dependency.Rev) != 40 || strings.Trim(dependency.Rev, "0123456789abcdef") != "" || dependency.Checksum != "" || dependency.Integrity != "" {
						t.Fatalf("invalid Git lock entry: %#v", dependency)
					}
					kinds = append(kinds, "git")
				default:
					if dependency.Rev != "" {
						t.Fatalf("private lock entry must not include Git revision: %#v", dependency)
					}
					kinds = append(kinds, "private")
				}
				if dependency.Git == "" {
					for _, hash := range []string{dependency.Checksum, dependency.Integrity} {
						if len(hash) != 64 || strings.Trim(hash, "0123456789abcdef") != "" {
							t.Fatalf("expected lowercase SHA-256 hash, got %q", hash)
						}
					}
				}
				got = append(got, dependency.Name+"@"+dependency.Version)
			}
			if !slices.Equal(got, fixture.dependencies) {
				t.Fatalf("unexpected locked dependencies: got %#v want %#v", got, fixture.dependencies)
			}
			if !slices.Equal(kinds, fixture.entryKinds) {
				t.Fatalf("unexpected lock entry kinds: got %#v want %#v", kinds, fixture.entryKinds)
			}

			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "soldeer-lock" || result.Sources[0].Path != "soldeer.lock" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source: %+v", result.Sources)
			}
		})
	}
}

func TestFoundryRemappingsFixturesDetected(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []struct {
		name     string
		mappings []string
	}{
		{name: "remappings", mappings: []string{"@openzeppelin/=lib/openzeppelin-contracts/", "ds-test/=lib/forge-std/lib/ds-test/src/", "forge-std/=lib/forge-std/src/"}},
		{name: "remappings-root", mappings: []string{"forge-std/=lib/forge-std/src/", "solady/=lib/solady/src/"}},
		{name: "remappings-subdirectory", mappings: []string{"@solady-utils/=lib/solady/src/utils/", "@openzeppelin/contracts/=lib/openzeppelin-contracts/contracts/"}},
		{name: "remappings-context", mappings: []string{"lib/lib_1/:@openzeppelin/=lib/lib_1/node_modules/@openzeppelin/", "lib/lib_2/:@openzeppelin/=lib/lib_2/node_modules/@openzeppelin/"}},
		{name: "remappings-hardhat", mappings: []string{"@openzeppelin/contracts/=node_modules/@openzeppelin/contracts/", "hardhat/=node_modules/hardhat/"}},
		{name: "remappings-nested", mappings: []string{"ds-test/=lib/forge-std/lib/ds-test/src/", "openzeppelin-contracts/=lib/openzeppelin-contracts/contracts/", "solmate/=lib/solmate/src/"}},
		{name: "remappings-local", mappings: []string{"@src/=src/", "contracts/=src/"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "foundry", fixture.name, "remappings.txt")
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var lines []string
			for _, line := range strings.Split(string(contents), "\n") {
				if line = strings.TrimSpace(line); line != "" {
					lines = append(lines, line)
				}
			}
			if !slices.Equal(lines, fixture.mappings) {
				t.Fatalf("unexpected mappings: %#v", lines)
			}
			for _, line := range lines {
				prefix, target, ok := strings.Cut(line, "=")
				if !ok || strings.Count(line, "=") != 1 || prefix == "" || !strings.HasSuffix(prefix, "/") || target == "" || !strings.HasSuffix(target, "/") {
					t.Fatalf("invalid remapping %q", line)
				}
			}
			result, err := Scan(filepath.Dir(path), nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 || result.Sources[0].Detector != "foundry-remappings" || result.Sources[0].Path != "remappings.txt" || result.Sources[0].Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported}) {
				t.Fatalf("unexpected source %+v", result.Sources)
			}
		})
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
