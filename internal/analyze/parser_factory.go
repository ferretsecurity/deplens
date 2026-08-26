package analyze

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

type bannerRegexAnalyzerConfig struct {
	Pattern string `yaml:"pattern"`
}

func compileSourceAnalyzer(raw ruleConfig) (sourceAnalyzer, error) {
	if raw.Analyzer == nil {
		return nil, nil
	}
	if raw.Analyzer.Type == "" {
		return nil, fmt.Errorf("type: required")
	}

	switch raw.Analyzer.Type {
	case "banner-regex":
		var config bannerRegexAnalyzerConfig
		if err := raw.Analyzer.decodeStrict(&config); err != nil {
			return nil, err
		}
		if config.Pattern == "" {
			return nil, fmt.Errorf("pattern: required")
		}
		return newBannerRegexParser(config.Pattern)
	case "terraform":
		return decodeAndCreate(raw.Analyzer, newTerraformResourceParser)
	case "ini":
		return decodeAndCreate(raw.Analyzer, newINIQueryParser)
	case "git-submodules":
		return decodeAndCreate(raw.Analyzer, newGitSubmodulesParser)
	case "nix-default-shell":
		return decodeAndCreate(raw.Analyzer, newNixDefaultShellParser)
	case "nix-flake":
		return decodeAndCreate(raw.Analyzer, newNixFlakeParser)
	case "nix-flake-lock":
		return decodeAndCreate(raw.Analyzer, newNixFlakeLockParser)
	case "github-actions-action":
		return decodeAndCreate(raw.Analyzer, newGithubActionsActionParser)
	case "helm-chart":
		return decodeAndCreate(raw.Analyzer, newHelmChartParser)
	case "helm-chart-lock":
		return decodeAndCreate(raw.Analyzer, newHelmChartLockParser)
	case "ios-podfile":
		return decodeAndCreate(raw.Analyzer, newIOSPodfileParser)
	case "ios-podfile-lock":
		return decodeAndCreate(raw.Analyzer, newIOSPodfileLockParser)
	case "ios-podspec":
		return decodeAndCreate(raw.Analyzer, newIOSPodspecParser)
	case "ios-cartfile":
		return decodeAndCreate(raw.Analyzer, newIOSCartfileParser)
	case "ios-cartfile-resolved":
		return decodeAndCreate(raw.Analyzer, newIOSCartfileResolvedParser)
	case "homebrew-brewfile":
		return decodeAndCreate(raw.Analyzer, newHomebrewBrewfileParser)
	case "homebrew-brewfile-lock":
		return decodeAndCreate(raw.Analyzer, newHomebrewBrewfileLockParser)
	case "typescript":
		return decodeAndCreate(raw.Analyzer, newTypeScriptMatcher)
	case "python":
		return decodeAndCreate(raw.Analyzer, newPythonMatcher)
	case "py-requirements":
		return decodeAndCreate(raw.Analyzer, newPyRequirementsMatcher)
	case "conda-environment":
		return decodeAndCreate(raw.Analyzer, newCondaEnvironmentParser)
	case "pipfile-lock":
		return decodeAndCreate(raw.Analyzer, newPipfileLockParser)
	case "poetry-lock":
		return decodeAndCreate(raw.Analyzer, newPoetryLockParser)
	case "pdm-lock":
		return decodeAndCreate(raw.Analyzer, newPDMLockParser)
	case "uv-lock":
		return decodeAndCreate(raw.Analyzer, newUVLockParser)
	case "go-mod":
		return decodeAndCreate(raw.Analyzer, newGoModMatcher)
	case "go-gopkg-lock":
		return decodeAndCreate(raw.Analyzer, newGopkgLockParser)
	case "go-gopkg-toml":
		return decodeAndCreate(raw.Analyzer, newGopkgTOMLParser)
	case "go-glide-lock":
		return decodeAndCreate(raw.Analyzer, newGlideLockParser)
	case "go-glide-yaml":
		return decodeAndCreate(raw.Analyzer, newGlideYAMLParser)
	case "package-lock":
		return decodeAndCreate(raw.Analyzer, newPackageLockParser)
	case "bun-lock":
		return decodeAndCreate(raw.Analyzer, newBunLockParser)
	case "package-json":
		return decodeAndCreate(raw.Analyzer, newPackageJSONParser)
	case "ocaml-esy":
		return decodeAndCreate(raw.Analyzer, newOCamlEsyParser)
	case "ocaml-opam":
		return decodeAndCreate(raw.Analyzer, newOCamlOpamParser)
	case "bower":
		return decodeAndCreate(raw.Analyzer, newBowerParser)
	case "deno-json":
		return decodeAndCreate(raw.Analyzer, newDenoJSONParser)
	case "deno-jsonc":
		return decodeAndCreate(raw.Analyzer, newDenoJSONCParser)
	case "importmap":
		return decodeAndCreate(raw.Analyzer, newImportMapParser)
	case "jsonnet-bundler":
		return decodeAndCreate(raw.Analyzer, newJSONNetBundlerParser)
	case "jsonnet-lock":
		return decodeAndCreate(raw.Analyzer, newJSONNetLockParser)
	case "deno-lock":
		return decodeAndCreate(raw.Analyzer, newDenoLockParser)
	case "pnpm-lock":
		return decodeAndCreate(raw.Analyzer, newPNPMLockParser)
	case "composer-lock":
		return decodeAndCreate(raw.Analyzer, newComposerLockParser)
	case "cargo-lock":
		return decodeAndCreate(raw.Analyzer, newCargoLockParser)
	case "yarn-lock":
		return decodeAndCreate(raw.Analyzer, newYarnLockParser)
	case "gradle-build":
		return decodeAndCreate(raw.Analyzer, newGradleBuildParser)
	case "gradle-lock":
		return decodeAndCreate(raw.Analyzer, newGradleLockParser)
	case "gradle-version-catalog":
		return decodeAndCreate(raw.Analyzer, newGradleVersionCatalogParser)
	case "scala-mill":
		return decodeAndCreate(raw.Analyzer, newScalaMillParser)
	case "scala-sbt-build":
		return decodeAndCreate(raw.Analyzer, newScalaSBTBuildParser)
	case "gemfile":
		return decodeAndCreate(raw.Analyzer, newGemfileParser)
	case "gemfile-lock":
		return decodeAndCreate(raw.Analyzer, newGemfileLockParser)
	case "gemspec":
		return decodeAndCreate(raw.Analyzer, newGemspecParser)
	case "chef-berksfile":
		return decodeAndCreate(raw.Analyzer, newChefBerksfileParser)
	case "chef-berksfile-lock":
		return decodeAndCreate(raw.Analyzer, newChefBerksfileLockParser)
	case "chef-metadata":
		return decodeAndCreate(raw.Analyzer, newChefMetadataParser)
	case "chef-policyfile":
		return decodeAndCreate(raw.Analyzer, newChefPolicyfileParser)
	case "chef-policyfile-lock":
		return decodeAndCreate(raw.Analyzer, newChefPolicyfileLockParser)
	case "dockerfile":
		return decodeAndCreate(raw.Analyzer, newDockerfileParser)
	case "docker-compose":
		return decodeAndCreate(raw.Analyzer, newDockerComposeParser)
	case "maven-pom":
		return decodeAndCreate(raw.Analyzer, newMavenPOMParser)
	case "java-ivy":
		return decodeAndCreate(raw.Analyzer, newIvyParser)
	case "cargo-manifest":
		return decodeAndCreate(raw.Analyzer, newCargoManifestParser)
	case "fortran-fpm":
		return decodeAndCreate(raw.Analyzer, newFortranFPMParser)
	case "foundry-toml":
		return decodeAndCreate(raw.Analyzer, newFoundryTOMLParser)
	case "pants-config":
		return decodeAndCreate(raw.Analyzer, newPantsConfigParser)
	case "composer-manifest":
		return decodeAndCreate(raw.Analyzer, newComposerManifestParser)
	case "dotnet-project":
		return decodeAndCreate(raw.Analyzer, newDotnetProjectParser)
	case "dotnet-central-packages":
		return decodeAndCreate(raw.Analyzer, newDotnetCentralPackagesParser)
	case "dotnet-packages-config":
		return decodeAndCreate(raw.Analyzer, newDotnetPackagesConfigParser)
	case "dotnet-packages-lock":
		return decodeAndCreate(raw.Analyzer, newDotnetPackagesLockParser)
	case "dotnet-paket-dependencies":
		return decodeAndCreate(raw.Analyzer, newDotnetPaketDependenciesParser)
	case "dotnet-paket-lock":
		return decodeAndCreate(raw.Analyzer, newDotnetPaketLockParser)
	case "dotnet-paket-references":
		return decodeAndCreate(raw.Analyzer, newDotnetPaketReferencesParser)
	case "buf":
		return decodeAndCreate(raw.Analyzer, newBufManifestParser)
	case "buf-lock":
		return decodeAndCreate(raw.Analyzer, newBufLockParser)
	case "conanfile":
		return decodeAndCreate(raw.Analyzer, newConanfileParser)
	case "conanfile-py":
		return decodeAndCreate(raw.Analyzer, newConanfilePyParser)
	case "conan-lock":
		return decodeAndCreate(raw.Analyzer, newConanLockParser)
	case "meson":
		return decodeAndCreate(raw.Analyzer, newMesonParser)
	case "vcpkg":
		return decodeAndCreate(raw.Analyzer, newVcpkgParser)
	case "vcpkg-configuration":
		return decodeAndCreate(raw.Analyzer, newVcpkgConfigurationParser)
	case "dart-pubspec":
		return decodeAndCreate(raw.Analyzer, newDartPubspecParser)
	case "dart-pubspec-lock":
		return decodeAndCreate(raw.Analyzer, newDartPubspecLockParser)
	case "erlang-rebar-config":
		return decodeAndCreate(raw.Analyzer, newErlangRebarConfigParser)
	case "erlang-rebar-lock":
		return decodeAndCreate(raw.Analyzer, newErlangRebarLockParser)
	case "elixir-mix":
		return decodeAndCreate(raw.Analyzer, newElixirMixParser)
	case "julia-project":
		return decodeAndCreate(raw.Analyzer, newJuliaProjectParser)
	case "clojure-boot":
		return decodeAndCreate(raw.Analyzer, newClojureBootParser)
	case "haskell-cabal":
		return decodeAndCreate(raw.Analyzer, newHaskellCabalParser)
	case "haskell-stack":
		return decodeAndCreate(raw.Analyzer, newHaskellStackParser)
	case "haskell-stack-lock":
		return decodeAndCreate(raw.Analyzer, newHaskellStackLockParser)
	case "haskell-package-yaml":
		return decodeAndCreate(raw.Analyzer, newHaskellPackageYAMLParser)
	case "haskell-cabal-project-freeze":
		return decodeAndCreate(raw.Analyzer, newHaskellCabalProjectFreezeParser)
	case "ocaml-dune-project":
		return decodeAndCreate(raw.Analyzer, newOCamlDuneProjectParser)
	case "lua-rockspec":
		return decodeAndCreate(raw.Analyzer, newLuaRocksParser)
	case "emacs-cask":
		return decodeAndCreate(raw.Analyzer, newEmacsCaskParser)
	case "perl-build-pl":
		return decodeAndCreate(raw.Analyzer, newPerlBuildPLParser)
	case "perl-cpanfile":
		return decodeAndCreate(raw.Analyzer, newPerlCpanfileParser)
	case "perl-cpanfile-snapshot":
		return decodeAndCreate(raw.Analyzer, newPerlCpanfileSnapshotParser)
	case "r-renv-lock":
		return decodeAndCreate(raw.Analyzer, newRRenvLockParser)
	case "raku-meta":
		return decodeAndCreate(raw.Analyzer, newRakuMetaParser)
	case "perl-makefile-pl":
		return decodeAndCreate(raw.Analyzer, newPerlMakefilePLParser)
	case "clojure-deps-edn":
		return decodeAndCreate(raw.Analyzer, newClojureDepsEDNParser)
	case "clojure-project-clj":
		return decodeAndCreate(raw.Analyzer, newClojureProjectCLJParser)
	case "crystal-shard":
		return decodeAndCreate(raw.Analyzer, newCrystalShardParser)
	case "crystal-shard-lock":
		return decodeAndCreate(raw.Analyzer, newCrystalShardLockParser)
	case "gleam":
		return decodeAndCreate(raw.Analyzer, newGleamParser)
	case "yaml":
		return decodeAndCreate(raw.Analyzer, newYAMLQueryParser)
	case "toml":
		return decodeAndCreate(raw.Analyzer, newTOMLQueryParser)
	case "json":
		return decodeAndCreate(raw.Analyzer, newJSONMatcher)
	case "xml":
		return decodeAndCreate(raw.Analyzer, newXMLMatcher)
	case "html":
		return decodeAndCreate(raw.Analyzer, newHTMLMatcher)
	default:
		return nil, fmt.Errorf("type: unsupported value %q", raw.Analyzer.Type)
	}
}

func (c analyzerConfig) decodeStrict(target any) error {
	data, err := yaml.Marshal(&c.config)
	if err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%s configuration: %w", c.Type, err)
	}
	return nil
}

func decodeAndCreate[T any](config *analyzerConfig, create func(T) (sourceAnalyzer, error)) (sourceAnalyzer, error) {
	var value T
	if err := config.decodeStrict(&value); err != nil {
		return nil, err
	}
	return create(value)
}
