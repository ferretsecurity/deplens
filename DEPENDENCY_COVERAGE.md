# Dependency coverage

This inventory is generated from `internal/analyze/default_rules.yaml`. It describes the 185 built-in dependency-source detectors using source forms, roles, analyzers, and derived capabilities. Of these, 81 have semantic analyzers and 108 are selector-only. The YAML file remains the source of truth for selectors and analyzer configuration.

Capabilities are `select`, `recognize`, `assess-presence`, `extract`, `normalize`, and `relate`. Relationship fields are available in the shared model but are only populated when an analyzer has that information.

| Detector ID | Form | Roles | Analyzer | Capabilities |
| --- | --- | --- | --- | --- |
| `python-requirements` | `requirements` | `declaration, constraint` | `py-requirements` | select, recognize, extract, normalize |
| `python-requirements-dir` | `requirements` | `declaration, constraint` | `py-requirements` | select, recognize, extract, normalize |
| `python-uv` | `lockfile` | `resolution, integrity` | `uv-lock` | select, recognize, extract, normalize |
| `python-poetry-lock` | `lockfile` | `resolution, integrity` | `poetry-lock` | select, recognize, extract, normalize |
| `python-pipfile-lock` | `lockfile` | `resolution, integrity` | `pipfile-lock` | select, recognize, extract, normalize |
| `python-pdm-lock` | `lockfile` | `resolution, integrity` | `toml` | select, recognize, assess-presence |
| `python-conda-lock` | `lockfile` | `resolution, integrity` | `yaml` | select, recognize, assess-presence |
| `python-conda-env-alt` | `manifest` | `declaration, constraint` | — | select |
| `python-pyproject` | `manifest` | `declaration, constraint` | `toml` | select, recognize, extract, normalize |
| `python-conda-environment` | `manifest` | `declaration, constraint` | `yaml` | select, recognize |
| `python-pipfile` | `manifest` | `declaration, constraint` | `toml` | select, recognize, extract, normalize |
| `python-setup-py` | `manifest` | `declaration, constraint` | `python` | select, recognize, extract, normalize |
| `python-setup-cfg` | `manifest` | `declaration, constraint` | `ini` | select, recognize, extract, normalize |
| `python-constraints` | `constraint-file` | `constraint` | — | select |
| `js` | `manifest` | `declaration, constraint` | `package-json` | select, recognize, extract, normalize, relate |
| `js-bower` | `manifest` | `declaration, constraint` | `bower` | select, recognize, extract, normalize, relate |
| `js-npm-shrinkwrap` | `lockfile` | `resolution, integrity` | `package-lock` | select, recognize, extract, normalize |
| `js-npm-lock` | `lockfile` | `resolution, integrity` | `package-lock` | select, recognize, extract, normalize |
| `js-yarn` | `lockfile` | `resolution, integrity` | `yarn-lock` | select, recognize, extract, normalize |
| `js-pnpm-lock` | `lockfile` | `resolution, integrity` | `pnpm-lock` | select, recognize, extract, normalize |
| `js-bun-lock` | `lockfile` | `resolution, integrity` | — | select |
| `js-bun-lockb` | `lockfile` | `resolution, integrity` | — | select |
| `deno-lock` | `lockfile` | `resolution, integrity` | — | select |
| `deno-json` | `manifest` | `declaration, constraint` | `json` | select, recognize, assess-presence |
| `deno-jsonc` | `manifest` | `declaration, constraint` | `json` | select, recognize, assess-presence |
| `js-pnp` | `vendored-file` | `inventory, resolution` | — | select |
| `js-pnpm-workspace` | `workspace-definition` | `workspace, configuration` | — | select |
| `js-npmrc` | `tool-config` | `configuration` | — | select |
| `js-yarnrc` | `tool-config` | `configuration` | — | select |
| `js-importmap` | `manifest` | `declaration, constraint` | — | select |
| `java` | `manifest` | `declaration, constraint` | `maven-pom` | select, recognize, extract, normalize, relate |
| `java-gradle-lockfile` | `lockfile` | `resolution, integrity` | `gradle-lock` | select, recognize, extract, normalize, relate |
| `java-gradle` | `build-definition` | `declaration, constraint, configuration` | `gradle-build` | select, recognize, extract, normalize, relate |
| `java-gradle-kts` | `build-definition` | `declaration, constraint, configuration` | `gradle-build` | select, recognize, extract, normalize, relate |
| `java-gradle-settings` | `workspace-definition` | `workspace, configuration` | — | select |
| `java-gradle-settings-kts` | `workspace-definition` | `workspace, configuration` | — | select |
| `java-gradle-version-catalog` | `version-catalog` | `declaration, constraint` | `gradle-version-catalog` | select, recognize, extract, normalize, relate |
| `java-gradle-wrapper` | `tool-config` | `configuration` | — | select |
| `scala-sbt-build` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `scala-sbt-plugins` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `scala-sbt-dependencies` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `scala-sbt-build-props` | `tool-config` | `configuration` | — | select |
| `scala-mill` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `java-ant-build` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `java-ivy` | `manifest` | `declaration, constraint` | — | select |
| `java-ivy-settings` | `tool-config` | `configuration` | — | select |
| `ruby-gemfile` | `manifest` | `declaration, constraint` | `gemfile` | select, recognize, extract, normalize, relate |
| `ruby-gemfile-lock` | `lockfile` | `resolution, integrity` | `gemfile-lock` | select, recognize, extract, normalize, relate |
| `ruby-gemspec` | `manifest` | `declaration, constraint` | — | select |
| `ruby-appraisal` | `manifest` | `declaration, constraint` | — | select |
| `swift-package` | `manifest` | `declaration, constraint` | — | select |
| `ios-podfile` | `manifest` | `declaration, constraint` | — | select |
| `ios-cartfile` | `manifest` | `declaration, constraint` | `ios-cartfile` | select, recognize, assess-presence, extract, normalize, relate |
| `ios-podspec` | `manifest` | `declaration, constraint` | — | select |
| `ios-cartfile-resolved` | `lockfile` | `resolution, integrity` | `ios-cartfile-resolved` | select, recognize, assess-presence, extract, normalize, relate |
| `php-composer` | `manifest` | `declaration, constraint` | `composer-manifest` | select, recognize, extract, normalize, relate |
| `php-composer-lock` | `lockfile` | `resolution, integrity` | `composer-lock` | select, recognize, extract, normalize |
| `dart-pubspec` | `manifest` | `declaration, constraint` | `yaml` | select, recognize, assess-presence |
| `dart-pubspec-lock` | `lockfile` | `resolution, integrity` | `yaml` | select, recognize, assess-presence |
| `erlang-rebar-config` | `manifest` | `declaration, constraint` | `erlang-rebar-config` | select, recognize, extract, normalize, relate |
| `erlang-rebar-lock` | `lockfile` | `resolution, integrity` | — | select |
| `clojure-deps-edn` | `manifest` | `declaration, constraint` | — | select |
| `clojure-project-clj` | `manifest` | `declaration, constraint` | `clojure-project-clj` | select, recognize, extract, normalize, relate |
| `clojure-boot` | `manifest` | `declaration, constraint` | `clojure-boot` | select, recognize, extract, normalize, relate |
| `haskell-stack` | `manifest` | `declaration, constraint` | `haskell-stack` | select, recognize, extract, normalize, relate |
| `haskell-stack-lock` | `lockfile` | `resolution, integrity` | — | select |
| `haskell-cabal-project` | `manifest` | `declaration, constraint` | — | select |
| `haskell-cabal-project-freeze` | `lockfile` | `resolution, integrity` | `haskell-cabal-project-freeze` | select, recognize, extract, normalize, relate |
| `haskell-cabal` | `manifest` | `declaration, constraint` | `haskell-cabal` | select, recognize, extract, normalize, relate |
| `haskell-package-yaml` | `manifest` | `declaration, constraint` | `haskell-package-yaml` | select, recognize, extract, normalize, relate |
| `dotnet-packages-config` | `manifest` | `declaration, constraint` | `dotnet-packages-config` | select, recognize, extract, normalize, relate |
| `dotnet-packages-lock` | `lockfile` | `resolution, integrity` | `dotnet-packages-lock` | select, recognize, extract, normalize, relate |
| `dotnet-directory-packages-props` | `version-catalog` | `declaration, constraint` | `dotnet-central-packages` | select, recognize, extract, normalize, relate |
| `dotnet-paket-dependencies` | `requirements` | `declaration, constraint` | — | select |
| `dotnet-paket-lock` | `lockfile` | `resolution, integrity` | — | select |
| `dotnet-fsproj` | `build-definition` | `declaration, constraint, configuration` | `dotnet-project` | select, recognize, extract, normalize, relate |
| `dotnet-vbproj` | `build-definition` | `declaration, constraint, configuration` | `dotnet-project` | select, recognize, extract, normalize, relate |
| `dotnet-directory-build` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `dotnet-paket-references` | `requirements` | `declaration, constraint` | — | select |
| `dotnet-tools-manifest` | `manifest` | `declaration, constraint` | — | select |
| `go-mod` | `manifest` | `declaration, constraint, resolution` | `go-mod` | select, recognize, extract, normalize |
| `go-sum` | `checksum-file` | `integrity` | — | select |
| `go-work` | `workspace-definition` | `workspace, configuration` | — | select |
| `go-gopkg-toml` | `manifest` | `declaration, constraint` | — | select |
| `go-glide-yaml` | `manifest` | `declaration, constraint` | `yaml` | select, recognize, assess-presence |
| `go-godep` | `manifest` | `declaration, constraint` | — | select |
| `rust-cargo` | `manifest` | `declaration, constraint` | `cargo-manifest` | select, recognize, extract, normalize, relate |
| `rust-cargo-lock` | `lockfile` | `resolution, integrity` | `cargo-lock` | select, recognize, extract, normalize |
| `rust-cargo-config` | `tool-config` | `configuration` | — | select |
| `go-gopkg-lock` | `lockfile` | `resolution, integrity` | `toml` | select, recognize, assess-presence |
| `go-glide-lock` | `lockfile` | `resolution, integrity` | — | select |
| `dotnet-csproj` | `build-definition` | `declaration, constraint, configuration` | `dotnet-project` | select, recognize, extract, normalize, relate |
| `cpp-conanfile` | `manifest` | `declaration, constraint` | — | select |
| `cpp-conan-lock` | `lockfile` | `resolution, integrity` | `conan-lock` | select, recognize, extract, normalize, relate |
| `cpp-vcpkg` | `manifest` | `declaration, constraint` | `vcpkg` | select, recognize, extract, normalize, relate |
| `cpp-cmake` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `cpp-conanfile-py` | `manifest` | `declaration, constraint` | `conanfile-py` | select, recognize, extract, normalize, relate |
| `cpp-vcpkg-config` | `tool-config` | `configuration` | — | select |
| `cpp-meson` | `build-definition` | `declaration, constraint, configuration` | `meson` | select, recognize, extract, normalize, relate |
| `cpp-autotools` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `cpp-cmake-modules` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `cpp-meson-wrap` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `swift-package-resolved` | `lockfile` | `resolution, integrity` | `json` | select, recognize, assess-presence |
| `ios-podfile-lock` | `lockfile` | `resolution, integrity` | `yaml` | select, recognize, assess-presence |
| `elixir-mix` | `manifest` | `declaration, constraint` | — | select |
| `elixir-mix-lock` | `lockfile` | `resolution, integrity` | — | select |
| `julia-project` | `manifest` | `declaration, constraint` | `toml` | select, recognize, assess-presence |
| `julia-manifest` | `lockfile` | `resolution, integrity` | `toml` | select, recognize, assess-presence |
| `perl-cpanfile` | `manifest` | `declaration, constraint` | — | select |
| `perl-cpanfile-snapshot` | `lockfile` | `resolution, integrity` | — | select |
| `perl-makefile-pl` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `perl-build-pl` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `perl-meta` | `manifest` | `declaration, constraint` | — | select |
| `perl-dist-ini` | `tool-config` | `configuration` | — | select |
| `raku-meta` | `manifest` | `declaration, constraint` | — | select |
| `r-renv-lock` | `lockfile` | `resolution, integrity` | — | select |
| `r-packrat-lock` | `lockfile` | `resolution, integrity` | — | select |
| `lua-rockspec` | `manifest` | `declaration, constraint` | — | select |
| `zig-build-zon` | `manifest` | `declaration, constraint` | — | select |
| `zig-build` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `nim-nimble` | `manifest` | `declaration, constraint` | — | select |
| `ocaml-opam` | `manifest` | `declaration, constraint` | — | select |
| `ocaml-opam-locked` | `lockfile` | `resolution` | — | select |
| `ocaml-dune-project` | `workspace-definition` | `workspace, configuration` | — | select |
| `ocaml-esy` | `manifest` | `declaration, constraint` | — | select |
| `crystal-shard` | `manifest` | `declaration, constraint` | `yaml` | select, recognize, assess-presence |
| `crystal-shard-lock` | `lockfile` | `resolution, integrity` | — | select |
| `gleam` | `manifest` | `declaration, constraint` | `toml` | select, recognize, assess-presence |
| `gleam-manifest` | `lockfile` | `resolution, integrity` | — | select |
| `fortran-fpm` | `manifest` | `declaration, constraint` | `fortran-fpm` | select, recognize, extract, normalize |
| `vlang` | `manifest` | `declaration, constraint` | — | select |
| `helm-chart` | `deployment-definition` | `declaration, configuration` | `helm-chart` | select, recognize, extract, normalize, relate |
| `ansible-requirements` | `requirements` | `declaration, constraint` | `yaml` | select, recognize, assess-presence |
| `buf` | `manifest` | `declaration, constraint` | `buf` | select, recognize, extract, normalize, relate |
| `homebrew-brewfile` | `manifest` | `declaration, constraint` | `homebrew-brewfile` | select, recognize, extract, normalize, relate |
| `jsonnet-bundler` | `manifest` | `declaration, constraint` | `jsonnet-bundler` | select, recognize, extract, normalize, relate |
| `terraform-lock` | `lockfile` | `resolution, integrity` | — | select |
| `unity-packages-manifest` | `manifest` | `declaration, constraint` | `json` | select, recognize, assess-presence |
| `unity-packages-lock` | `lockfile` | `resolution, integrity` | — | select |
| `docker-dockerfile` | `deployment-definition` | `declaration, configuration` | `dockerfile` | select, recognize, extract, normalize, relate |
| `docker-compose` | `deployment-definition` | `declaration, configuration` | `docker-compose` | select, recognize, extract, normalize, relate |
| `github-actions-action` | `automation-definition` | `configuration, usage` | `github-actions-action` | select, recognize, extract, normalize, relate |
| `github-actions-workflow` | `automation-definition` | `configuration, usage` | — | select |
| `bazel-workspace` | `workspace-definition` | `workspace, configuration` | — | select |
| `bazel-module` | `manifest` | `declaration, constraint` | — | select |
| `bazel-module-lock` | `lockfile` | `resolution, integrity` | — | select |
| `bazel-build-file` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `bazel-third-party-bzl` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `js-nx` | `tool-config` | `configuration` | — | select |
| `js-lerna` | `tool-config` | `configuration` | — | select |
| `js-rush` | `tool-config` | `configuration` | — | select |
| `rush-common-versions` | `constraint-file` | `constraint` | — | select |
| `js-turbo` | `tool-config` | `configuration` | — | select |
| `pants-config` | `tool-config` | `configuration` | — | select |
| `pants-jvm-build` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `git-submodules` | `manifest` | `declaration, constraint` | — | select |
| `nix-default-shell` | `manifest` | `declaration, constraint` | — | select |
| `nix-flake` | `manifest` | `declaration, constraint` | — | select |
| `nix-flake-lock` | `lockfile` | `resolution, integrity` | — | select |
| `helm-chart-lock` | `lockfile` | `resolution, integrity` | `helm-chart-lock` | select, recognize, extract, normalize, relate |
| `homebrew-brewfile-lock` | `lockfile` | `resolution, integrity` | `homebrew-brewfile-lock` | select, recognize, extract, normalize, relate |
| `buf-lock` | `lockfile` | `resolution, integrity` | `buf-lock` | select, recognize, extract, normalize, relate |
| `puppet-puppetfile` | `manifest` | `declaration, constraint` | — | select |
| `chef-berksfile` | `manifest` | `declaration, constraint` | — | select |
| `chef-berksfile-lock` | `lockfile` | `resolution, integrity` | — | select |
| `chef-metadata` | `manifest` | `declaration, constraint` | — | select |
| `chef-policyfile` | `manifest` | `declaration, constraint` | — | select |
| `chef-policyfile-lock` | `lockfile` | `resolution, integrity` | — | select |
| `jsonnet-lock` | `lockfile` | `resolution, integrity` | — | select |
| `emacs-cask` | `manifest` | `declaration, constraint` | `emacs-cask` | select, recognize, extract, normalize, relate |
| `unreal-uproject` | `manifest` | `declaration, constraint` | — | select |
| `unreal-uplugin` | `manifest` | `declaration, constraint` | — | select |
| `godot-plugin-cfg` | `tool-config` | `configuration` | — | select |
| `foundry-toml` | `tool-config` | `configuration` | `foundry-toml` | select, recognize, extract, normalize |
| `foundry-remappings` | `constraint-file` | `constraint` | — | select |
| `soldeer-lock` | `lockfile` | `resolution, integrity` | — | select |
| `js-banner-block-start` | `source-code` | `usage, inventory` | `banner-regex` | select, recognize, extract, normalize |
| `js-banner-plain-block-start` | `source-code` | `usage, inventory` | `banner-regex` | select, recognize, extract, normalize |
| `js-banner-multiline-preserved` | `source-code` | `usage, inventory` | `banner-regex` | select, recognize, extract, normalize |
| `js-banner-line-comment` | `source-code` | `usage, inventory` | `banner-regex` | select, recognize, extract, normalize |
| `js-banner-version-tagged` | `source-code` | `usage, inventory` | `banner-regex` | select, recognize, extract, normalize |
| `html-external-scripts` | `markup` | `usage` | `html` | select, recognize, extract, normalize |
| `terraform.aws_glue_job.python` | `source-code` | `usage, inventory` | `terraform` | select, recognize |
| `typescript.cdk.aws_glue_job.python` | `source-code` | `usage, inventory` | `typescript` | select, recognize, extract, normalize |
| `python.cdk.aws_glue_job.python` | `source-code` | `usage, inventory` | `python` | select, recognize, extract, normalize |
