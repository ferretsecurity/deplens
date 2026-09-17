# Coverage presets

`--exclude-preset snyk` and `--exclude-preset socket` remove detectors with documented source-format coverage. Repeat either flag to combine their memberships. The lists contain 43 Snyk Open Source IDs and 41 Socket SCA IDs, frozen on 2026-09-17. The exact lists and full evidence appear below.

A source coverage gap is a recognized dependency source that the target tool cannot analyze. Retained sources include unverified mappings, so they are candidates for investigation, not proven gaps. A reference coverage gap means a vendor scan misses particular dependencies within a supported format. Presets address source coverage only. They do not validate your actual vendor scan or promise complete reference coverage.

Coverage includes documented extra flags, installations, builds, restoration, native adapters, and concrete vendor-recommended SBOM recipes. Generic SBOM import alone does not prove arbitrary source support. Required setup and labeled native-build inferences are preserved below. Uncertain broad selectors and separate-product mappings remain enabled.

Presets apply after the base and all extensions are validated and composed. They match detector IDs, including custom definitions with different semantics. Use new IDs for custom replacements. Members absent from a custom base and its extensions do nothing. Individual detector exclusions still require a known ID. All exclusions win regardless of flag position, and repeats and overlaps are harmless. Flags must precede the positional path.

Presets remove detectors only. Remaining checks whose prerequisites were removed report `detector-disabled` skips. Unrelated checks still run. `--disable-check` removes a check and all its output independently.

```bash
deplens --extend-rules company.yaml --exclude-preset socket --disable-rule python-poetry-lock --disable-check javascript-npm-lockfile-missing .
deplens --rules company-base.yaml --extend-rules company.yaml --extend-rules team.yaml --exclude-preset snyk --exclude-preset socket .
```

Scans use embedded lists. They make no vendor calls and need no vendor credentials, accounts, or installations.

## Frozen research evidence

The following reports are preserved from issue #136. Their references to candidate data and pre-implementation follow-up describe the original research, not additional setup required by deplens. The listed memberships are now shipped without expanding uncertain mappings. No authenticated vendor scans were performed.

<details>
<summary>Snyk Open Source research and exact preset IDs</summary>

# Snyk Open Source dependency source coverage

Research date: 2026-09-17. Product boundary: Snyk Open Source SCA, using CLI, IDE, or SCM integrations. Snyk Code, Container, and IaC are separate products and do not establish membership here. This report follows the shared preset definition and maps the current default detectors.

The preset means that Snyk has a supported way to analyze the source format with the necessary setup. It does not promise that default auto-discovery finds every file or that every dependency reference appears in a scan. Native tooling, installation, restoration, explicit file selection, and preview settings count as setup.

## Confirmed source formats

Each row's detector IDs are candidates for exclusion. Companion files and tooling are prerequisites, not reasons to call a source unsupported.

| Manager / source | Deplens detector IDs | Setup and evidence |
| --- | --- | --- |
| pip requirements, including alternative names and `.in` files | `python-requirements`, `python-requirements-dir`, `python-constraints` | Install dependencies in the selected Python environment. Use `snyk test --file=PATH --package-manager=pip` for nonstandard names, including files under `requirements/`. Requirements syntax is selected by the explicit manager, not the extension. Constraints can be supplied as requirements input to inspect their package entries; normal constraint-only files do not themselves install packages. [CLI file selection](https://docs.snyk.io/developer-tools/snyk-cli/commands/test), [Python resolver](https://github.com/snyk/snyk-python-plugin/blob/master/pysrc/pip_resolve.py). |
| Poetry | `python-poetry-lock` | `pyproject.toml` and `poetry.lock` must both exist. Poetry v1 and v2 are supported. Generate the lockfile with Poetry when missing. [Python CLI](https://docs.snyk.io/supported-languages/supported-languages-list/python/snyk-cli-for-python). |
| Pipenv | `python-pipfile`, `python-pipfile-lock` | Both files and the installed project are needed; `pipenv install` prepares resolution. [Python CLI](https://docs.snyk.io/supported-languages/supported-languages-list/python/snyk-cli-for-python). |
| setuptools `setup.py` | `python-setup-py` | Install the project and select `--file=setup.py`; it is not auto-discovered. Snyk documents `install_requires` analysis. Extras and other reference details are not guaranteed by format support. [Python CLI](https://docs.snyk.io/supported-languages/supported-languages-list/python/snyk-cli-for-python). |
| uv | `python-uv` | Native Early Access support shipped April 9, 2026, for CLI v1.1304 onward. Enable it under Snyk Preview. `test`, `monitor`, and `sbom` support uv. The CLI recognizes `uv.lock`, gated by `enableUvCLI`. This is native support, distinct from the earlier SBOM-export workaround. [Release announcement](https://updates.snyk.io/announcing-native-uv-support-for-the-snyk-cli/), [CLI detection](https://github.com/snyk/cli/blob/master/src/lib/detect.ts). |
| npm, Yarn, pnpm | `js`, `js-npm-lock`, `js-yarn`, `js-pnpm-lock` | `package.json`, npm lockfiles v1-v3, Yarn 1-4, pnpm 7-10. Generate missing lockfiles or use an installed project where supported. Include development dependencies with `--dev`. [JavaScript](https://docs.snyk.io/supported-languages/supported-languages-list/javascript). |
| Maven | `java` | `pom.xml`; working Maven environment, required artifacts installed, project profiles/settings supplied as needed. [Java and Kotlin](https://docs.snyk.io/supported-languages/supported-languages-list/java-and-kotlin), [build requirements](https://docs.snyk.io/developer-tools/snyk-cli/scan-and-maintain-projects-using-the-cli/snyk-cli-for-open-source/open-source-projects-that-must-be-built-before-testing-with-the-snyk-cli). |
| Gradle Groovy / Kotlin | `java-gradle`, `java-gradle-kts`, `java-gradle-lockfile` | Snyk invokes Gradle; CLI supports both build DSLs. Configure subprojects, configurations and attributes as needed. SCM also documents `gradle.lockfile`. [Java and Kotlin](https://docs.snyk.io/supported-languages/supported-languages-list/java-and-kotlin), [SCM Gradle](https://docs.snyk.io/supported-languages/supported-languages-list/java-and-kotlin/git-repositories-with-maven-and-gradle), [Gradle plugin](https://github.com/snyk/snyk-gradle-plugin). |
| sbt | `scala-sbt-build` | `build.sbt`; install sbt and its dependency-graph plugin using the Snyk-documented `addSbtPlugin` form. CLI execution covers computed build declarations more broadly than SCM parsing. [Scala](https://docs.snyk.io/supported-languages/supported-languages-list/scala). |
| Bundler | `ruby-gemfile`, `ruby-gemfile-lock` | Both `Gemfile` and `Gemfile.lock`; generate the lockfile with `bundle install`. Snyk scans all Bundler groups. Platform-specific package limitations remain. [Ruby](https://docs.snyk.io/supported-languages/supported-languages-list/ruby). |
| Composer | `php-composer`, `php-composer-lock` | `composer.json` plus `composer.lock`; run Composer to generate the lockfile when missing. [PHP](https://docs.snyk.io/supported-languages/supported-languages-list/php). |
| Go modules | `go-mod` | Native CLI uses Go tooling and source imports. SCM can use the complete module graph. CLI omission of test imports is a reference coverage limitation. [Go](https://docs.snyk.io/supported-languages/supported-languages-list/go). |
| Go dep | `go-gopkg-lock`, `go-gopkg-toml` | Snyk scans `Gopkg.lock`; run `dep ensure`. Including the manifest is a build-workflow inference: dep resolves it into the lockfile consumed by Snyk, just as Composer/Bundler generate their lockfiles. [Go](https://docs.snyk.io/supported-languages/supported-languages-list/go). |
| NuGet | `dotnet-packages-config`, `dotnet-csproj`, `dotnet-directory-packages-props`, `dotnet-directory-build` | Restore/build projects to produce `obj/project.assets.json` for CLI scanning. Modern SCM uses the .NET SDK and explicitly documents central package management and `Directory.Build.props`. The detector also covers `Directory.Build.targets`; its coverage is inferred from the same SDK build import path. [NET support](https://docs.snyk.io/supported-languages/supported-languages-list/.net). |
| Paket | `dotnet-paket-dependencies`, `dotnet-paket-lock` | Native CLI support; install/restore before scanning. SCM import is not supported for Paket. [NET support](https://docs.snyk.io/supported-languages/supported-languages-list/.net). |
| CocoaPods | `ios-podfile`, `ios-podfile-lock` | `Podfile` plus `Podfile.lock`; use `pod install` when the lockfile is missing. CLI and SCM supported. [Swift and Objective-C](https://docs.snyk.io/supported-languages/supported-languages-list/swift-and-objective-c). |
| Swift Package Manager | `swift-package`, `swift-package-resolved` | CLI invokes `swift package show-dependencies` from a project with `Package.swift`. `Package.resolved` coverage is inferred through Swift's native resolution, not a standalone Snyk parser. Additional post-processing dependencies are omitted. [Swift and Objective-C](https://docs.snyk.io/supported-languages/supported-languages-list/swift-and-objective-c). |
| Elixir Mix / Hex | `elixir-mix`, `elixir-mix-lock` | Install Elixir and Mix; synchronized `mix.exs` and `mix.lock` required. Umbrella projects supported. Hex can contain Erlang packages, but this does not establish Rebar file support. Git/path dependency limitations concern references. [Elixir](https://docs.snyk.io/supported-languages/supported-languages-list/elixir). |

## Native build integration mappings

These candidates rely on the build tool consuming an auxiliary source rather than Snyk parsing it as an independent entry point. A valid containing project is required.

| Source | Detector IDs | Evidence and inference |
| --- | --- | --- |
| Gradle settings and version catalogs | `java-gradle-settings`, `java-gradle-settings-kts`, `java-gradle-version-catalog` | The Snyk plugin injects a Gradle script to resolve dependencies, so settings and catalog references participate in the native build. Gradle documents automatic loading of `gradle/libs.versions.toml`. Unused catalog entries need not appear in a resolved graph. [Snyk plugin](https://github.com/snyk/snyk-gradle-plugin), [Gradle catalogs](https://docs.gradle.org/current/userguide/version_catalogs.html). |
| sbt dependency definitions | `scala-sbt-dependencies` | The CLI runs sbt with a dependency-graph plugin, so project dependencies defined in `project/Dependencies.scala` can resolve. Snyk explicitly warns that SCM parsing alone does not cover this layout. This inclusion is specific to the native CLI workflow. [Scala](https://docs.snyk.io/supported-languages/supported-languages-list/scala). |

Do not automatically extend this reasoning to build-tool distributions or plugins. Reading a wrapper configuration to launch a tool does not establish that the tool distribution becomes an analyzed dependency.

## Partial, uncertain, and separately supported sources

| Source / detector | Finding and initial preset decision |
| --- | --- |
| Generic `python-pyproject` | Poetry and uv support establish coverage for those projects. The detector also recognizes build-system requirements, generic PEP 621 projects and dependency groups independently of the manager. Snyk's Python CLI docs restrict PEP 621 support to Poetry, while the newer uv release extends the product beyond that statement. Retain this broad detector initially rather than hide unrelated PDM/setuptools/Hatch projects. Splitting by project type would allow precise exclusions. [Python CLI](https://docs.snyk.io/supported-languages/supported-languages-list/python/snyk-cli-for-python), [uv release](https://updates.snyk.io/announcing-native-uv-support-for-the-snyk-cli/). |
| `python-setup-cfg` | The Python plugin's setup path reads `setup.py` declarations; native support for standalone `setup.cfg` is not established. Retain. [Resolver source](https://github.com/snyk/snyk-python-plugin/blob/master/pysrc/pip_resolve.py). |
| `ruby-gemspec` | The CLI detection map explicitly accepts `.gemspec`, but the current user docs only promise Gemfile pairs. Recognition alone is insufficient to prove complete handling. Retain pending plugin-path verification. [Detection source](https://github.com/snyk/cli/blob/master/src/lib/detect.ts), [Ruby](https://docs.snyk.io/supported-languages/supported-languages-list/ruby). |
| `js-npm-shrinkwrap` | Include through installation. Use a compatible npm version such as npm 10, whose installer honors shrinkwrap, then scan the resulting `node_modules` with Snyk when no competing `package-lock.json` is present. This is a native installed-project path, not a verified direct shrinkwrap parser. [npm 10 install](https://docs.npmjs.com/cli/v10/commands/npm-install), [Snyk JavaScript](https://docs.snyk.io/supported-languages/supported-languages-list/javascript). |
| `js-pnpm-workspace` | Snyk explicitly supports `pnpm-workspace.yaml` with root package/lock files and `--all-projects`. The detector also matches `.yml`; that spelling is not established as supported. Retain the broad detector until its selector is narrowed or split, then exclude the `.yaml` rule. [JavaScript workspaces](https://docs.snyk.io/supported-languages/supported-languages-list/javascript#support-for-monorepos-and-workspaces). |
| Bazel `bazel-*` | Snyk documents Bazel v7 integration through the Dep Graph API, with an example manually constructing a Maven graph. This is real integration support, but not a generic parser for every Bazel source. C++ is explicitly outside the Dep Graph endpoint coverage. Keep the broad Bazel detectors enabled until there is a source-to-graph adapter mapping for their selectors. [Snyk for Bazel](https://docs.snyk.io/scan-with-snyk/snyk-open-source/snyk-for-bazel), [manual graph example](https://docs.snyk.io/scan-with-snyk/snyk-open-source/snyk-for-bazel/example-of-snyk-for-bazel). |
| `dotnet-fsproj`, `dotnet-vbproj` | Documentation says Open Source supports C# only, but its legacy SCM section also names VB project files and a likely misspelled F# extension. SDK-generated NuGet assets may be scannable regardless of language. Retain due to contradictory product documentation. [NET support](https://docs.snyk.io/supported-languages/supported-languages-list/.net). |
| `dotnet-packages-lock` | NuGet restoration can consume lockfiles, but the Snyk page does not explicitly establish `packages.lock.json` support and includes an ambiguous warning naming `package-lock.json`. Retain until the lockfile-to-assets route is verified. [NET support](https://docs.snyk.io/supported-languages/supported-languages-list/.net). |
| Dart `dart-pubspec`, `dart-pubspec-lock` | Include as conditional adapter coverage. Snyk documents installing the pub `sbom` package, creating `sbom.yaml`, running `dart pub global run sbom`, then `snyk sbom test --file=sbom-pub.json`. This concrete vendor recipe counts under the shared policy, although Snyk does not parse pub sources itself. Flutter platform dependencies use Gradle/CocoaPods after a Flutter build. [Dart and Flutter](https://docs.snyk.io/supported-languages/supported-languages-list/dart-and-flutter). |
| Rust `rust-cargo`, `rust-cargo-lock`, `rust-cargo-config` | Snyk documents third-party SBOM generation or package API queries, not a native Cargo source analyzer. Do not exclude based only on `pkg:cargo` vulnerability support. [Rust](https://docs.snyk.io/supported-languages/supported-languages-list/rust). |
| C/C++ Conan, vcpkg, CMake, Meson, Autotools | `--unmanaged` fingerprints unpacked dependency source files. That does not establish parsing of the manager's dependency sources. Conan PURL/SBOM support is not `conanfile` or `conan.lock` support. Retain the `cpp-*` detectors. [C/C++](https://docs.snyk.io/supported-languages/supported-languages-list/c-c%2B%2B), [C/C++ guidance](https://docs.snyk.io/supported-languages-package-managers-and-frameworks/c-c%2B%2B/guidance-for-snyk-for-c-c%2B%2B). |

The official [language matrix](https://docs.snyk.io/supported-languages/supported-languages-package-managers-and-frameworks) and [CLI manager registry](https://github.com/snyk/cli/blob/master/src/lib/package-managers.ts) do not establish native support for the other current detector families: Bun, Deno, Bower, import maps, Ant/Ivy, Mill, Rebar, Clojure, Haskell, Conda, PDM, Julia, Perl/Raku, R, Lua, Zig, Nim, OCaml, Crystal, Gleam, Fortran, V, Unity, Pants, Nix, Helm, Ansible, Buf, Homebrew, Jsonnet, Puppet/Chef, editor/game-engine sources, Solidity, JavaScript banners, HTML scripts, or Glue dependency declarations. Keep those rules enabled. This is lack of established coverage, not proof that every possible adapter is impossible.

Also retain auxiliary files without verified analysis: `js-pnp`, `js-npmrc`, `js-yarnrc`, `java-gradle-wrapper`, `scala-sbt-plugins`, `scala-sbt-build-props`, `ruby-appraisal`, `ios-podspec`, `dotnet-paket-references`, `dotnet-tools-manifest`, `go-sum`, `go-work`, and monorepo-tool configuration. A supported parent ecosystem does not by itself establish these files as analyzed sources.

## Candidate preset data

These 43 IDs include conditional native support, the documented Dart adapter, and the build-integration inferences identified above. They exclude the broad/uncertain cases. This is a research input, not activated configuration.

```yaml
snyk:
  - python-requirements
  - python-requirements-dir
  - python-constraints
  - python-poetry-lock
  - python-pipfile
  - python-pipfile-lock
  - python-setup-py
  - python-uv
  - js
  - js-npm-lock
  - js-npm-shrinkwrap
  - js-yarn
  - js-pnpm-lock
  - java
  - java-gradle
  - java-gradle-kts
  - java-gradle-lockfile
  - java-gradle-settings
  - java-gradle-settings-kts
  - java-gradle-version-catalog
  - scala-sbt-build
  - scala-sbt-dependencies
  - ruby-gemfile
  - ruby-gemfile-lock
  - php-composer
  - php-composer-lock
  - go-mod
  - go-gopkg-lock
  - go-gopkg-toml
  - dotnet-packages-config
  - dotnet-csproj
  - dotnet-directory-packages-props
  - dotnet-directory-build
  - dotnet-paket-dependencies
  - dotnet-paket-lock
  - ios-podfile
  - ios-podfile-lock
  - swift-package
  - swift-package-resolved
  - elixir-mix
  - elixir-mix-lock
  - dart-pubspec
  - dart-pubspec-lock
```

Before implementation, review the broad `python-pyproject` decision. Pin upstream code citations to a release commit if code-only evidence is promoted into shipped membership. No authenticated Snyk scans were run for this research.

</details>

<details>
<summary>Socket SCA research and exact preset IDs</summary>

# Socket SCA dependency source coverage

Research date: 2026-09-17. Implementation input for the proposed `socket` preset; no exclusions are active yet. Apply the shared coverage definition. IDs below come from `internal/analyze/default_rules.yaml`.

## Scope and findings

Count source formats that Socket can analyze with documented setup, including its local build adapters. This is source coverage, not a promise that every reference will appear in a particular scan. The candidate list includes conditional and experimental support, with conditions below.

Socket's GitHub integration reads selected dependency sources. CLI users can generate dependency graphs locally before uploading them. These are different execution paths, so using the preset assumes the customer performs the necessary setup. [GitHub installation](https://docs.socket.dev/docs/socket-for-github-installation), [manifest generation](https://docs.socket.dev/docs/socket-manifest).

## Confirmed source mappings

"Direct" means documented dependency-source parsing. "Adapter" means a documented build or package-manager workflow. Companion configuration files are not automatically covered merely because their language is supported.

| Dependency sources | Exact detector IDs | Path and conditions |
| --- | --- | --- |
| npm, Yarn, pnpm, Bun sources | `js`, `js-npm-lock`, `js-npm-shrinkwrap`, `js-yarn`, `js-pnpm-lock`, `js-bun-lock`, `js-bun-lockb` | Direct. Bun is public beta. [File detection](https://docs.socket.dev/docs/manifest-file-detection-in-socket). |
| pnpm workspace and Rush | `js-pnpm-workspace`, `js-rush` | Workspace support is documented; the collection list explicitly names both `.yaml` and `.yml` for pnpm workspaces. [Ecosystem support](https://docs.socket.dev/docs/language-support), [collected files](https://docs.socket.dev/docs/permissions). |
| Python lockfiles and project metadata | `python-uv`, `python-poetry-lock`, `python-pipfile-lock`, `python-pipfile`, `python-pyproject`, `python-setup-py` | Direct. [File detection](https://docs.socket.dev/docs/manifest-file-detection-in-socket). |
| Text files in requirements directories | `python-requirements-dir` | Explicit directory pattern. [File detection](https://docs.socket.dev/docs/manifest-file-detection-in-socket). |
| Maven | `java` | Direct `pom.xml` parsing. [Manifest generation](https://docs.socket.dev/docs/socket-manifest). |
| Gradle Groovy/Kotlin builds | `java-gradle`, `java-gradle-kts` | Adapter, using the actual Gradle build. [Gradle setup](https://docs.socket.dev/docs/gradle-setup-instructions-for-java-kotlin-and-scala). |
| Gradle resolution and catalog | `java-gradle-lockfile`, `java-gradle-version-catalog` | Direct; catalog declarations do not prove actual use. [Gradle setup](https://docs.socket.dev/docs/gradle-setup-instructions-for-java-kotlin-and-scala). |
| sbt | `scala-sbt-build` | Adapter invoking sbt. [Scala setup](https://docs.socket.dev/docs/scala-setup-instructions). |
| Gradle settings and sbt dependency definitions | `java-gradle-settings`, `java-gradle-settings-kts`, `scala-sbt-dependencies` | Conditional inference: the adapters execute the native build, so definitions used by that build contribute to its resolved project graph. This does not claim analysis of tool/plugin distributions. [Gradle setup](https://docs.socket.dev/docs/gradle-setup-instructions-for-java-kotlin-and-scala), [Scala setup](https://docs.socket.dev/docs/scala-setup-instructions). |
| Ruby | `ruby-gemfile-lock` | Direct. [Ruby support announcement](https://socket.dev/blog/rubygems-ecosystem-support-now-generally-available). |
| Ruby declarations | `ruby-gemfile`, `ruby-gemspec` | Adapter required for dependable coverage; generate CycloneDX using the vendor-recommended Ruby integration. [Ruby guidance](https://socket.dev/blog/rubygems-ecosystem-support-now-generally-available), [generator](https://github.com/CycloneDX/cyclonedx-ruby-gem). |
| NuGet projects and package lists | `dotnet-csproj`, `dotnet-fsproj`, `dotnet-vbproj`, `dotnet-packages-config`, `dotnet-packages-lock` | Direct. [File detection](https://docs.socket.dev/docs/manifest-file-detection-in-socket). |
| Go modules | `go-mod`, `go-sum` | Direct. [File detection](https://docs.socket.dev/docs/manifest-file-detection-in-socket). |
| Cargo | `rust-cargo`, `rust-cargo-lock` | Direct. [File detection](https://docs.socket.dev/docs/manifest-file-detection-in-socket). |
| Composer | `php-composer`, `php-composer-lock` | Direct. [File detection](https://docs.socket.dev/docs/manifest-file-detection-in-socket). |
| GitHub workflows and action definitions | `github-actions-workflow`, `github-actions-action` | Experimental, requires feature access. Direct and transitive composite-action dependencies count. [Actions announcement](https://socket.dev/blog/introducing-github-actions-scanning-support), [collected files](https://docs.socket.dev/docs/permissions). |

Socket documents npm 6-11, Yarn 1-3, and pnpm 5-11. Yarn/pnpm protocols have limitations; Yarn Plug'n'Play is planned. Poetry optional dependencies have documented limitations. These are reference-coverage restrictions on supported formats, not reasons to remove those formats from this list. [Ecosystem support](https://docs.socket.dev/docs/language-support).

## Required workflows

Gradle projects can run `socket manifest gradle .`, then `socket scan create .`. This resolves dependencies locally, so CI needs a working build environment and dependency access. Multiple independent builds need `socket manifest dynamic-sbom-inference .`. A committed lockfile is another supported path. Catalog-only scanning includes declared but unused entries and misses dependencies declared outside the catalog. [Gradle setup](https://docs.socket.dev/docs/gradle-setup-instructions-for-java-kotlin-and-scala).

sbt projects use `socket manifest scala .`, then scan the generated output. The adapter invokes sbt; its options allow selecting the binary, input/output directories, and passing sbt arguments. A source-only GitHub scan should not be assumed equivalent. [Scala setup](https://docs.socket.dev/docs/scala-setup-instructions).

Ruby's documented alternative is the CycloneDX Ruby generator. Install `cyclonedx-ruby`, run it against the resolved Ruby project, and commit or upload the resulting supported SBOM. This is a specific vendor-recommended workflow for Gemfile/gemspec projects, rather than an inference from generic SBOM import. [Ruby guidance](https://socket.dev/blog/rubygems-ecosystem-support-now-generally-available), [generator usage](https://github.com/CycloneDX/cyclonedx-ruby-gem).

Socket also exposes `socket manifest cdxgen`. Its default lifecycle is pre-build; a build-dependent workflow needs `--lifecycle build`. The existence of this generic command alone does not establish complete native coverage of every cdxgen input. [Socket cdxgen](https://docs.socket.dev/docs/socket-cdxgen).

## Partial, separate, or unverified mappings

| Sources / detector IDs | Finding and preset decision |
| --- | --- |
| `python-requirements` | Standard requirements text and variants are supported. However, deplens also matches `.in` and a broader filename regex. Full selector coverage is unverified, so retain this detector. `python-requirements-dir` is independently confirmed. [Collected files](https://docs.socket.dev/docs/permissions). |
| `python-pdm-lock`, `python-setup-cfg`, `python-constraints` | Do not infer these from Python support. PDM is documented through pyproject/pylock, not `pdm.lock`. No matching direct support was established for the other two. [Ecosystem support](https://docs.socket.dev/docs/language-support). |
| `python-conda-environment`, `python-conda-env-alt`, `python-conda-lock` | Conda has a native manifest-generation command and an environment-export recipe. Coverage is limited to packages also published on PyPI. Alternate filenames and conda-lock parsing remain unverified. Retain these detectors. [Manifest generation](https://docs.socket.dev/docs/socket-manifest), [Anaconda setup](https://docs.socket.dev/docs/anaconda-setup-instructions). |
| `java-gradle-wrapper`, `scala-sbt-build-props`, `scala-sbt-plugins` | Executing a native build does not establish analysis of build plugins or tool distributions. Keep outside the first list pending adapter verification. [Gradle setup](https://docs.socket.dev/docs/gradle-setup-instructions-for-java-kotlin-and-scala), [Scala setup](https://docs.socket.dev/docs/scala-setup-instructions). |
| `java-ivy`, `clojure-project-clj` | Socket collects the files, but collection alone does not prove dependency extraction. Retain pending parser evidence. [Collected files](https://docs.socket.dev/docs/permissions). |
| `dotnet-directory-packages-props`, `dotnet-directory-build` | Socket collects `*.props` and `*.targets`. Its broad MSBuild support claim suggests these configuration inputs work, but exact central-package and imported-target behavior was not established. Keep outside the confirmed list, consistently with Ivy's collection-only evidence. [Collected files](https://docs.socket.dev/docs/permissions). |
| `bazel-workspace`, `bazel-module`, `bazel-module-lock`, `bazel-build-file`, `bazel-third-party-bzl` | Native Bazel adapter covers Maven and PyPI. These detectors cover broader Bazel sources and dependency ecosystems. Do not exclude whole detectors. [Manifest generation](https://docs.socket.dev/docs/socket-manifest). |
| `swift-package`, `swift-package-resolved` | Exact inputs are documented, but the ecosystem table puts Swift CVE support behind Socket Basics. Keep separate from the initial SCA preset until that product boundary is deliberately included. [File detection](https://docs.socket.dev/docs/manifest-file-detection-in-socket), [ecosystem support](https://docs.socket.dev/docs/language-support). |
| `cpp-conanfile`, `cpp-conanfile-py`, `cpp-conan-lock`; `julia-project`, `julia-manifest`; `dart-pubspec`, `dart-pubspec-lock`; `elixir-mix`, `elixir-mix-lock`; `erlang-rebar-config`, `erlang-rebar-lock` | Socket Basics advertises Conan, Julia Pkg, Pub, and Hex CVE support. Exact source mappings are not documented in the reviewed SCA pages. These are candidates for separate Basics research, not verified SCA exclusions. [Ecosystem support](https://docs.socket.dev/docs/language-support). |
| `docker-dockerfile`, `docker-compose` | Basics has container scanning. That does not establish SCA source coverage of arbitrary Dockerfile commands or Compose references. [Basics product description](https://socket.dev/blog/socket-basics). |

Socket Basics is a separate scanning suite with its own execution and configuration. Its current repository describes bundled Trivy and other scanners. Do not copy all upstream scanner capabilities into the Socket SCA preset without verifying Socket's actual invocation and supported inputs. [Basics repository](https://github.com/SocketDev/socket-basics), [Basics API](https://docs.socket.dev/reference/getsocketbasicsconfig).

The remaining deplens ecosystems and source families have no established mapping in this research: Deno/Bower/import maps, legacy Go managers, Paket, CocoaPods/Carthage, Mill/Ant, other C/C++ managers, Clojure deps/Boot, Haskell, Perl/Raku/R, Lua/Zig/Nim/OCaml/Crystal/Gleam/Fortran/V, infrastructure/package orchestration, game engines, Solidity, vendored JavaScript banners, HTML scripts, and AWS Glue declarations. Absence from this report means unverified, not a claim that support is impossible. CocoaPods is explicitly planned in the [ecosystem table](https://docs.socket.dev/docs/language-support).

## Candidate preset data

The following list contains confirmed source-format mappings, the labeled native-resolver inferences for settings/dependency definitions, documented Ruby and JVM adapters, and experimental GitHub Actions. It excludes unresolved selector and product-boundary cases above. These are research data, not a new configuration schema.

```yaml
socket:
  - python-requirements-dir
  - python-uv
  - python-poetry-lock
  - python-pipfile-lock
  - python-pyproject
  - python-pipfile
  - python-setup-py
  - js
  - js-npm-shrinkwrap
  - js-npm-lock
  - js-yarn
  - js-pnpm-lock
  - js-bun-lock
  - js-bun-lockb
  - js-pnpm-workspace
  - js-rush
  - java
  - java-gradle-lockfile
  - java-gradle
  - java-gradle-kts
  - java-gradle-settings
  - java-gradle-settings-kts
  - java-gradle-version-catalog
  - scala-sbt-build
  - scala-sbt-dependencies
  - ruby-gemfile
  - ruby-gemfile-lock
  - ruby-gemspec
  - php-composer
  - php-composer-lock
  - dotnet-packages-config
  - dotnet-packages-lock
  - dotnet-fsproj
  - dotnet-vbproj
  - dotnet-csproj
  - go-mod
  - go-sum
  - rust-cargo
  - rust-cargo-lock
  - github-actions-action
  - github-actions-workflow
```

Before implementation, capture the authenticated supported-files response for a representative Socket organization if credentials are available. It exposes actual accepted filename patterns and can settle the requirements-selector question. This research used public sources and did not call an authenticated customer API. [Supported-files API](https://docs.socket.dev/reference/getsupportedfiles).

</details>

