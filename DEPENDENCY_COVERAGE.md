# Dependency coverage

This inventory is derived from `internal/analyze/default_rules.yaml`. It describes the 185 built-in dependency-source detectors using source forms, roles, analyzers, and derived capabilities. Of these, 134 have content analyzers and 51 are selector-only. The YAML file remains the source of truth for selectors and analyzer configuration.

Capabilities are `select`, `recognize`, `assess-presence`, `extract`, `normalize`, and `relate`. Relationship fields are available in the shared model but are only populated when an analyzer has that information.

| Detector ID | Form | Roles | Analyzer | Capabilities |
| --- | --- | --- | --- | --- |
| `python-requirements` | `requirements` | `declaration, constraint` | `py-requirements` | select, recognize, extract, normalize |
| `python-requirements-dir` | `requirements` | `declaration, constraint` | `py-requirements` | select, recognize, extract, normalize |
| `python-uv` | `lockfile` | `resolution, integrity` | `uv-lock` | select, recognize, extract, normalize |
| `python-poetry-lock` | `lockfile` | `resolution, integrity` | `poetry-lock` | select, recognize, extract, normalize |
| `python-pipfile-lock` | `lockfile` | `resolution, integrity` | `pipfile-lock` | select, recognize, extract, normalize |
| `python-pdm-lock` | `lockfile` | `resolution, integrity` | `pdm-lock` | select, recognize, extract, normalize |
| `python-conda-lock` | `lockfile` | `resolution, integrity` | `yaml` | select, recognize, assess-presence |
| `python-conda-env-alt` | `manifest` | `declaration, constraint` | `conda-environment` | select, recognize, extract, normalize, relate |
| `python-pyproject` | `manifest` | `declaration, constraint` | `toml` | select, recognize, extract, normalize |
| `python-conda-environment` | `manifest` | `declaration, constraint` | `conda-environment` | select, recognize, extract, normalize, relate |
| `python-pipfile` | `manifest` | `declaration, constraint` | `toml` | select, recognize, extract, normalize |
| `python-setup-py` | `manifest` | `declaration, constraint` | `python` | select, recognize, extract, normalize |
| `python-setup-cfg` | `manifest` | `declaration, constraint` | `ini` | select, recognize, extract, normalize |
| `python-constraints` | `constraint-file` | `constraint` | `py-requirements` | select, recognize, extract, normalize |
| `js` | `manifest` | `declaration, constraint` | `package-json` | select, recognize, extract, normalize, relate |
| `js-bower` | `manifest` | `declaration, constraint` | `bower` | select, recognize, extract, normalize, relate |
| `js-npm-shrinkwrap` | `lockfile` | `resolution, integrity` | `package-lock` | select, recognize, extract, normalize |
| `js-npm-lock` | `lockfile` | `resolution, integrity` | `package-lock` | select, recognize, extract, normalize |
| `js-yarn` | `lockfile` | `resolution, integrity` | `yarn-lock` | select, recognize, extract, normalize |
| `js-pnpm-lock` | `lockfile` | `resolution, integrity` | `pnpm-lock` | select, recognize, extract, normalize |
| `js-bun-lock` | `lockfile` | `resolution, integrity` | `bun-lock` | select, recognize, extract, normalize |
| `js-bun-lockb` | `lockfile` | `resolution, integrity` | — | select |
| `deno-lock` | `lockfile` | `resolution, integrity` | `deno-lock` | select, recognize, extract, normalize, relate |
| `deno-json` | `manifest` | `declaration, constraint` | `deno-json` | select, recognize, assess-presence, extract, normalize, relate |
| `deno-jsonc` | `manifest` | `declaration, constraint` | `deno-jsonc` | select, recognize, assess-presence, extract, normalize, relate |
| `js-pnp` | `vendored-file` | `inventory, resolution` | — | select |
| `js-pnpm-workspace` | `workspace-definition` | `workspace, configuration` | — | select |
| `js-npmrc` | `tool-config` | `configuration` | — | select |
| `js-yarnrc` | `tool-config` | `configuration` | — | select |
| `js-importmap` | `manifest` | `declaration, constraint` | `importmap` | select, recognize, extract, normalize, relate |
| `java` | `manifest` | `declaration, constraint` | `maven-pom` | select, recognize, extract, normalize, relate |
| `java-gradle-lockfile` | `lockfile` | `resolution, integrity` | `gradle-lock` | select, recognize, extract, normalize, relate |
| `java-gradle` | `build-definition` | `declaration, constraint, configuration` | `gradle-build` | select, recognize, extract, normalize, relate |
| `java-gradle-kts` | `build-definition` | `declaration, constraint, configuration` | `gradle-build` | select, recognize, extract, normalize, relate |
| `java-gradle-settings` | `workspace-definition` | `workspace, configuration` | — | select |
| `java-gradle-settings-kts` | `workspace-definition` | `workspace, configuration` | — | select |
| `java-gradle-version-catalog` | `version-catalog` | `declaration, constraint` | `gradle-version-catalog` | select, recognize, extract, normalize, relate |
| `java-gradle-wrapper` | `tool-config` | `configuration` | — | select |
| `scala-sbt-build` | `build-definition` | `declaration, constraint, configuration` | `scala-sbt-build` | select, recognize, extract, normalize, relate |
| `scala-sbt-plugins` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `scala-sbt-dependencies` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `scala-sbt-build-props` | `tool-config` | `configuration` | — | select |
| `scala-mill` | `build-definition` | `declaration, constraint, configuration` | `scala-mill` | select, recognize, extract, normalize, relate |
| `java-ant-build` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `java-ivy` | `manifest` | `declaration, constraint` | `java-ivy` | select, recognize, extract, normalize, relate |
| `java-ivy-settings` | `tool-config` | `configuration` | — | select |
| `ruby-gemfile` | `manifest` | `declaration, constraint` | `gemfile` | select, recognize, extract, normalize, relate |
| `ruby-gemfile-lock` | `lockfile` | `resolution, integrity` | `gemfile-lock` | select, recognize, extract, normalize, relate |
| `ruby-gemspec` | `manifest` | `declaration, constraint` | `gemspec` | select, recognize, extract, normalize, relate |
| `ruby-appraisal` | `manifest` | `declaration, constraint` | — | select |
| `swift-package` | `manifest` | `declaration, constraint` | `swift-package` | select, recognize, assess-presence, extract, normalize, relate |
| `ios-podfile` | `manifest` | `declaration, constraint` | `ios-podfile` | select, recognize, extract, normalize, relate |
| `ios-cartfile` | `manifest` | `declaration, constraint` | `ios-cartfile` | select, recognize, assess-presence, extract, normalize, relate |
| `ios-podspec` | `manifest` | `declaration, constraint` | `ios-podspec` | select, recognize, extract, normalize, relate |
| `ios-cartfile-resolved` | `lockfile` | `resolution, integrity` | `ios-cartfile-resolved` | select, recognize, assess-presence, extract, normalize, relate |
| `php-composer` | `manifest` | `declaration, constraint` | `composer-manifest` | select, recognize, extract, normalize, relate |
| `php-composer-lock` | `lockfile` | `resolution, integrity` | `composer-lock` | select, recognize, extract, normalize |
| `dart-pubspec` | `manifest` | `declaration, constraint` | `dart-pubspec` | select, recognize, assess-presence, extract, normalize, relate |
| `dart-pubspec-lock` | `lockfile` | `resolution, integrity` | `dart-pubspec-lock` | select, recognize, assess-presence, extract, normalize, relate |
| `erlang-rebar-config` | `manifest` | `declaration, constraint` | `erlang-rebar-config` | select, recognize, extract, normalize, relate |
| `erlang-rebar-lock` | `lockfile` | `resolution, integrity` | `erlang-rebar-lock` | select, recognize, extract, normalize, relate |
| `clojure-deps-edn` | `manifest` | `declaration, constraint` | `clojure-deps-edn` | select, recognize, extract, normalize, relate |
| `clojure-project-clj` | `manifest` | `declaration, constraint` | `clojure-project-clj` | select, recognize, extract, normalize, relate |
| `clojure-boot` | `manifest` | `declaration, constraint` | `clojure-boot` | select, recognize, extract, normalize, relate |
| `haskell-stack` | `manifest` | `declaration, constraint` | `haskell-stack` | select, recognize, extract, normalize, relate |
| `haskell-stack-lock` | `lockfile` | `resolution, integrity` | `haskell-stack-lock` | select, recognize, extract, normalize, relate |
| `haskell-cabal-project` | `manifest` | `declaration, constraint` | — | select |
| `haskell-cabal-project-freeze` | `lockfile` | `resolution, integrity` | `haskell-cabal-project-freeze` | select, recognize, extract, normalize, relate |
| `haskell-cabal` | `manifest` | `declaration, constraint` | `haskell-cabal` | select, recognize, extract, normalize, relate |
| `haskell-package-yaml` | `manifest` | `declaration, constraint` | `haskell-package-yaml` | select, recognize, extract, normalize, relate |
| `dotnet-packages-config` | `manifest` | `declaration, constraint` | `dotnet-packages-config` | select, recognize, extract, normalize, relate |
| `dotnet-packages-lock` | `lockfile` | `resolution, integrity` | `dotnet-packages-lock` | select, recognize, extract, normalize, relate |
| `dotnet-directory-packages-props` | `version-catalog` | `declaration, constraint` | `dotnet-central-packages` | select, recognize, extract, normalize, relate |
| `dotnet-paket-dependencies` | `requirements` | `declaration, constraint` | `dotnet-paket-dependencies` | select, recognize, extract, normalize, relate |
| `dotnet-paket-lock` | `lockfile` | `resolution, integrity` | `dotnet-paket-lock` | select, recognize, extract, normalize, relate |
| `dotnet-fsproj` | `build-definition` | `declaration, constraint, configuration` | `dotnet-project` | select, recognize, extract, normalize, relate |
| `dotnet-vbproj` | `build-definition` | `declaration, constraint, configuration` | `dotnet-project` | select, recognize, extract, normalize, relate |
| `dotnet-directory-build` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `dotnet-paket-references` | `requirements` | `declaration, constraint` | `dotnet-paket-references` | select, recognize, extract, normalize, relate |
| `dotnet-tools-manifest` | `manifest` | `declaration, constraint` | — | select |
| `go-mod` | `manifest` | `declaration, constraint, resolution` | `go-mod` | select, recognize, extract, normalize |
| `go-sum` | `checksum-file` | `integrity` | — | select |
| `go-work` | `workspace-definition` | `workspace, configuration` | — | select |
| `go-gopkg-toml` | `manifest` | `declaration, constraint` | `go-gopkg-toml` | select, recognize, extract, normalize, relate |
| `go-glide-yaml` | `manifest` | `declaration, constraint` | `go-glide-yaml` | select, recognize, assess-presence, extract, normalize, relate |
| `go-godep` | `manifest` | `declaration, constraint` | — | select |
| `rust-cargo` | `manifest` | `declaration, constraint` | `cargo-manifest` | select, recognize, extract, normalize, relate |
| `rust-cargo-lock` | `lockfile` | `resolution, integrity` | `cargo-lock` | select, recognize, extract, normalize |
| `rust-cargo-config` | `tool-config` | `configuration` | — | select |
| `go-gopkg-lock` | `lockfile` | `resolution, integrity` | `go-gopkg-lock` | select, recognize, assess-presence, extract, normalize, relate |
| `go-glide-lock` | `lockfile` | `resolution, integrity` | `go-glide-lock` | select, recognize, extract, normalize, relate |
| `dotnet-csproj` | `build-definition` | `declaration, constraint, configuration` | `dotnet-project` | select, recognize, extract, normalize, relate |
| `cpp-conanfile` | `manifest` | `declaration, constraint` | `conanfile` | select, recognize, extract, normalize, relate |
| `cpp-conan-lock` | `lockfile` | `resolution, integrity` | `conan-lock` | select, recognize, extract, normalize, relate |
| `cpp-vcpkg` | `manifest` | `declaration, constraint` | `vcpkg` | select, recognize, extract, normalize, relate |
| `cpp-cmake` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `cpp-conanfile-py` | `manifest` | `declaration, constraint` | `conanfile-py` | select, recognize, extract, normalize, relate |
| `cpp-vcpkg-config` | `tool-config` | `configuration` | `vcpkg-configuration` | select, recognize, extract, normalize, relate |
| `cpp-meson` | `build-definition` | `declaration, constraint, configuration` | `meson` | select, recognize, extract, normalize, relate |
| `cpp-autotools` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `cpp-cmake-modules` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `cpp-meson-wrap` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `swift-package-resolved` | `lockfile` | `resolution, integrity` | `swift-package-resolved` | select, recognize, assess-presence, extract, normalize, relate |
| `ios-podfile-lock` | `lockfile` | `resolution, integrity` | `ios-podfile-lock` | select, recognize, assess-presence, extract, normalize, relate |
| `elixir-mix` | `manifest` | `declaration, constraint` | `elixir-mix` | select, recognize, extract, normalize, relate |
| `elixir-mix-lock` | `lockfile` | `resolution, integrity` | — | select |
| `julia-project` | `manifest` | `declaration, constraint` | `julia-project` | select, recognize, assess-presence, extract, normalize, relate |
| `julia-manifest` | `lockfile` | `resolution, integrity` | `toml` | select, recognize, assess-presence |
| `perl-cpanfile` | `manifest` | `declaration, constraint` | `perl-cpanfile` | select, recognize, extract, normalize, relate |
| `perl-cpanfile-snapshot` | `lockfile` | `resolution, integrity` | `perl-cpanfile-snapshot` | select, recognize, assess-presence, extract, normalize, relate |
| `perl-makefile-pl` | `build-definition` | `declaration, constraint, configuration` | `perl-makefile-pl` | select, recognize, assess-presence, extract, normalize, relate |
| `perl-build-pl` | `build-definition` | `declaration, constraint, configuration` | `perl-build-pl` | select, recognize, assess-presence, extract, normalize, relate |
| `perl-meta` | `manifest` | `declaration, constraint` | — | select |
| `perl-dist-ini` | `tool-config` | `configuration` | — | select |
| `raku-meta` | `manifest` | `declaration, constraint` | `raku-meta` | select, recognize, extract, normalize, relate |
| `r-renv-lock` | `lockfile` | `resolution, integrity` | `r-renv-lock` | select, recognize, extract, normalize |
| `r-packrat-lock` | `lockfile` | `resolution, integrity` | — | select |
| `lua-rockspec` | `manifest` | `declaration, constraint` | `lua-rockspec` | select, recognize, extract, normalize, relate |
| `zig-build-zon` | `manifest` | `declaration, constraint` | `zig-build-zon` | select, recognize, extract, normalize, relate |
| `zig-build` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `nim-nimble` | `manifest` | `declaration, constraint` | — | select |
| `ocaml-opam` | `manifest` | `declaration, constraint` | `ocaml-opam` | select, recognize, extract, normalize, relate |
| `ocaml-opam-locked` | `lockfile` | `resolution` | — | select |
| `ocaml-dune-project` | `workspace-definition` | `workspace, configuration` | `ocaml-dune-project` | select, recognize, extract, normalize, relate |
| `ocaml-esy` | `manifest` | `declaration, constraint` | `ocaml-esy` | select, recognize, extract, normalize, relate |
| `crystal-shard` | `manifest` | `declaration, constraint` | `crystal-shard` | select, recognize, assess-presence, extract, normalize, relate |
| `crystal-shard-lock` | `lockfile` | `resolution, integrity` | `crystal-shard-lock` | select, recognize, extract, normalize, relate |
| `gleam` | `manifest` | `declaration, constraint` | `gleam` | select, recognize, assess-presence, extract, normalize, relate |
| `gleam-manifest` | `lockfile` | `resolution, integrity` | — | select |
| `fortran-fpm` | `manifest` | `declaration, constraint` | `fortran-fpm` | select, recognize, extract, normalize, relate |
| `vlang` | `manifest` | `declaration, constraint` | `vlang` | select, recognize, extract, normalize, relate |
| `helm-chart` | `deployment-definition` | `declaration, configuration` | `helm-chart` | select, recognize, extract, normalize, relate |
| `ansible-requirements` | `requirements` | `declaration, constraint` | `yaml` | select, recognize, assess-presence |
| `buf` | `manifest` | `declaration, constraint` | `buf` | select, recognize, extract, normalize, relate |
| `homebrew-brewfile` | `manifest` | `declaration, constraint` | `homebrew-brewfile` | select, recognize, extract, normalize, relate |
| `jsonnet-bundler` | `manifest` | `declaration, constraint` | `jsonnet-bundler` | select, recognize, extract, normalize, relate |
| `terraform-lock` | `lockfile` | `resolution, integrity` | `terraform-lock` | select, recognize, extract, normalize, relate |
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
| `pants-config` | `tool-config` | `configuration` | `pants-config` | select, recognize, assess-presence, extract, normalize, relate |
| `pants-jvm-build` | `build-definition` | `declaration, constraint, configuration` | — | select |
| `git-submodules` | `manifest` | `declaration, constraint` | `git-submodules` | select, recognize, extract, normalize, relate |
| `nix-default-shell` | `manifest` | `declaration, constraint` | `nix-default-shell` | select, recognize, extract, normalize, relate |
| `nix-flake` | `manifest` | `declaration, constraint` | `nix-flake` | select, recognize, extract, normalize, relate |
| `nix-flake-lock` | `lockfile` | `resolution, integrity` | `nix-flake-lock` | select, recognize, extract, normalize, relate |
| `helm-chart-lock` | `lockfile` | `resolution, integrity` | `helm-chart-lock` | select, recognize, extract, normalize, relate |
| `homebrew-brewfile-lock` | `lockfile` | `resolution, integrity` | `homebrew-brewfile-lock` | select, recognize, extract, normalize, relate |
| `buf-lock` | `lockfile` | `resolution, integrity` | `buf-lock` | select, recognize, extract, normalize, relate |
| `puppet-puppetfile` | `manifest` | `declaration, constraint` | — | select |
| `chef-berksfile` | `manifest` | `declaration, constraint` | `chef-berksfile` | select, recognize, extract, normalize, relate |
| `chef-berksfile-lock` | `lockfile` | `resolution, integrity` | `chef-berksfile-lock` | select, recognize, extract, normalize, relate |
| `chef-metadata` | `manifest` | `declaration, constraint` | `chef-metadata` | select, recognize, extract, normalize, relate |
| `chef-policyfile` | `manifest` | `declaration, constraint` | `chef-policyfile` | select, recognize, extract, normalize, relate |
| `chef-policyfile-lock` | `lockfile` | `resolution, integrity` | `chef-policyfile-lock` | select, recognize, extract, normalize, relate |
| `jsonnet-lock` | `lockfile` | `resolution, integrity` | `jsonnet-lock` | select, recognize, extract, normalize, relate |
| `emacs-cask` | `manifest` | `declaration, constraint` | `emacs-cask` | select, recognize, extract, normalize, relate |
| `unreal-uproject` | `manifest` | `declaration, constraint` | — | select |
| `unreal-uplugin` | `manifest` | `declaration, constraint` | — | select |
| `godot-plugin-cfg` | `tool-config` | `configuration` | — | select |
| `foundry-toml` | `tool-config` | `configuration` | `foundry-toml` | select, recognize, extract, normalize, relate |
| `foundry-remappings` | `constraint-file` | `constraint` | — | select |
| `soldeer-lock` | `lockfile` | `resolution, integrity` | `soldeer-lock` | select, recognize, extract, normalize, relate |
| `js-banner-block-start` | `source-code` | `usage, inventory` | `banner-regex` | select, recognize, extract, normalize |
| `js-banner-plain-block-start` | `source-code` | `usage, inventory` | `banner-regex` | select, recognize, extract, normalize |
| `js-banner-multiline-preserved` | `source-code` | `usage, inventory` | `banner-regex` | select, recognize, extract, normalize |
| `js-banner-line-comment` | `source-code` | `usage, inventory` | `banner-regex` | select, recognize, extract, normalize |
| `js-banner-version-tagged` | `source-code` | `usage, inventory` | `banner-regex` | select, recognize, extract, normalize |
| `html-external-scripts` | `markup` | `usage` | `html` | select, recognize, extract, normalize |
| `terraform.aws_glue_job.python` | `source-code` | `usage, inventory` | `terraform` | select, recognize |
| `typescript.cdk.aws_glue_job.python` | `source-code` | `usage, inventory` | `typescript` | select, recognize, extract, normalize |
| `python.cdk.aws_glue_job.python` | `source-code` | `usage, inventory` | `python` | select, recognize, extract, normalize |
