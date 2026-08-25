# Public registries for analyzer-loop work items

This is a planning aid for the 112 work items in
`.deplens/analyzer-implementation.yaml`.  It states the normal public source,
not a guarantee about every dependency in a particular file.  `No single
registry` means the format itself can name a private host, Git repository,
local path, or arbitrary URL; an analyzer should extract that source rather
than inventing a registry name.

## Public registry is normally clear

| Registry | Detector IDs | Limitation |
| --- | --- | --- |
| [Bazel Central Registry (BCR)](https://bazel.build/external/registry) | `bazel-module` | `MODULE.bazel` can select another registry or use overrides. |
| [Buf Schema Registry](https://buf.build/docs/bsr/module/dependency-management/) (`buf.build`) | `buf`, `buf-lock` | A module name can name a private BSR instance. |
| [Chef Supermarket](https://docs.chef.io/workstation/tools/berkshelf/) | `chef-berksfile`, `chef-berksfile-lock`, `chef-metadata`, `chef-policyfile`, `chef-policyfile-lock` | It is the default for Berkshelf; policies and cookbooks can use other Supermarkets, Chef servers, Git, or local paths. |
| [ConanCenter](https://docs.conan.io/2/tutorial/conan_repositories/other.html) | `cpp-conanfile`, `cpp-conanfile-py`, `cpp-conan-lock` | Conan remotes are configurable; the current public default is `center2.conan.io`. |
| [vcpkg public registry](https://learn.microsoft.com/vcpkg/concepts/registries) | `cpp-vcpkg`, `cpp-vcpkg-config` | The built-in baseline uses the `microsoft/vcpkg` Git registry; custom Git, artifact, and filesystem registries are supported. |
| [pub.dev](https://dart.dev/tools/pub/dependencies) | `dart-pubspec`, `dart-pubspec-lock` | A dependency can instead be `git`, `path`, or another hosted server. |
| [NuGet.org](https://learn.microsoft.com/nuget/consume-packages/configuring-nuget-behavior) | `dotnet-packages-lock` | Sources are configured and can be private. |
| [Hex](https://hexdocs.pm/mix/Mix.Tasks.Deps.html) | `elixir-mix`, `erlang-rebar-config`, `erlang-rebar-lock`, `gleam` | Mix/Rebar/Gleam also support Git and local dependencies; Hex repositories can be private. |
| [fpm registry](https://fpm.fortran-lang.org/registry/index.html) | `fortran-fpm` | fpm also supports Git dependencies. |
| [Hackage](https://cabal.readthedocs.io/en/stable/config.html) | `haskell-cabal`, `haskell-cabal-project-freeze`, `haskell-package-yaml` | Cabal repositories are configurable. `package.yaml` is converted to Cabal metadata. |
| [Hackage](https://docs.haskellstack.org/en/stable/topics/package_locations/) and [Stackage](https://www.stackage.org/) | `haskell-stack`, `haskell-stack-lock` | Stack may use a snapshot, Hackage, Git, archive, or local package. |
| [Homebrew](https://docs.brew.sh/Brew-Bundle-and-Brewfile) | `homebrew-brewfile`, `homebrew-brewfile-lock` | Formulae/casks are normally Homebrew; `tap` may add arbitrary Git repositories. |
| [CocoaPods trunk/CDN](https://guides.cocoapods.org/making/private-cocoapods.html) | `ios-podfile`, `ios-podfile-lock`, `ios-podspec` | Podfile and podspec sources can be private Specs repos, Git, or URLs. |
| [Maven Central](https://maven.apache.org/guides/introduction/introduction-to-repositories.html) | `java-ivy`, `scala-mill`, `scala-sbt-build` | Maven/Ivy repositories are configurable. sbt plugins also have a dedicated plugin repository. |
| [npm registry](https://docs.npmjs.com/cli/v11/using-npm/registry) | `js-bun-lock` | npm-compatible registries are configurable. |
| [General Julia registry](https://pkgdocs.julialang.org/v1/registries/) | `julia-project` | Julia supports multiple, private, and Git registries. |
| [LuaRocks.org](https://github.com/luarocks/luarocks/wiki/Using-LuaRocks) | `lua-rockspec` | Rock servers are configurable and a rockspec can use source URLs. |
| [opam-repository](https://opam.ocaml.org/doc/Manual.html#Repositories) | `ocaml-dune-project`, `ocaml-opam` | opam repositories are configurable. |
| [PyPI](https://packaging.python.org/en/latest/guides/installing-using-pip-and-virtual-environments/) | `python-constraints`, `python-pdm-lock`, `pants-config` | Python indexes can be overridden; Pants' own plugin/tool requirements use configured Python repositories. |
| [RubyGems.org](https://guides.rubygems.org/command-reference/) | `ruby-gemspec` | RubyGems sources can be changed; a gemspec can also have non-registry development dependencies. |
| [Maven Central](https://www.scala-sbt.org/1.x/docs/Library-Dependencies.html) | `scala-mill`, `scala-sbt-build` | Mill and sbt can use arbitrary repositories. |
| [Soldeer](https://soldeer.xyz/guide/installation-and-usage) | `soldeer-lock` | Soldeer dependencies may also use Git. |
| [Terraform Registry](https://developer.hashicorp.com/terraform/cli/config/config-file#provider-installation) | `terraform-lock` | Terraform discovers provider registries from their hostname; mirrors and private registries are supported. |
| [VPM](https://modules.vlang.io/) | `vlang` | V modules can also be fetched from Git hosts. |

## A default ecosystem exists, but it is not a single registry

| Detector IDs | Normal public ecosystem | Why it remains non-single |
| --- | --- | --- |
| `bazel-build-file`, `bazel-workspace` | BCR is common for Bazel modules | `WORKSPACE`/BUILD repository rules can fetch arbitrary Git repositories and URLs. [Bazel documents registry selection and non-registry repository rules.](https://bazel.build/external/overview) |
| `clojure-boot`, `clojure-deps-edn`, `clojure-project-clj` | Maven Central and Clojars | Clojure dependency coordinates may select Maven repositories, Git, local roots, or custom aliases. [deps.edn reference](https://clojure.org/reference/deps_edn). |
| `cpp-meson`, `cpp-meson-wrap` | [Meson WrapDB](https://mesonbuild.com/Wrap-dependency-system-manual.html) | Wrap files can also specify arbitrary Git, URL, and directory sources. |
| `crystal-shard`, `crystal-shard-lock` | [Shards](https://github.com/crystal-lang/shards#dependencies) commonly uses GitHub | Shards dependencies are Git repository locations, not a central registry. |
| `deno-json`, `deno-jsonc`, `deno-lock` | [JSR](https://docs.deno.com/runtime/fundamentals/modules/#jsr-packages) and npm | Deno imports can use JSR, npm, HTTPS URLs, Git-style URLs, or import maps. |
| `dotnet-paket-dependencies`, `dotnet-paket-lock`, `dotnet-paket-references` | NuGet.org | Paket supports NuGet feeds, GitHub, Git, HTTP, and local sources. [Paket dependency groups](https://fsprojects.github.io/Paket/dependencies-file.html). |
| `emacs-cask` | GNU ELPA and MELPA | Cask lists package archives; there is no single mandatory archive. [Cask syntax](https://cask.readthedocs.io/en/latest/guide/). |
| `foundry-toml` | Soldeer or Git sources | Foundry itself has no single package registry. Dependency installation and remappings can point to Git/local sources. [Foundry dependency guide](https://getfoundry.sh/projects/dependencies/). |
| `git-submodules` | Git hosting, often GitHub | Each submodule has an arbitrary Git URL. [Git submodule documentation](https://git-scm.com/docs/gitmodules). |
| `github-actions-action` | GitHub Actions Marketplace/GitHub repositories | `uses:` can reference an action repository, Docker image, or local path. [Metadata syntax](https://docs.github.com/actions/sharing-automations/creating-actions/metadata-syntax-for-github-actions). |
| `go-glide-yaml`, `go-glide-lock`, `go-gopkg-toml`, `go-gopkg-lock` | Go import paths, often `proxy.golang.org` | Glide and dep resolve arbitrary VCS/import paths, not a required registry. [Go module proxy protocol](https://go.dev/ref/mod#protocol). |
| `helm-chart`, `helm-chart-lock` | [Helm chart repositories](https://helm.sh/docs/topics/chart_repository/) and OCI registries | A chart specifies a repository URL; Artifact Hub is discovery, not the only source. |
| `ios-cartfile`, `ios-cartfile-resolved` | Git hosting, often GitHub | Carthage dependencies are Git repository URLs. [Cartfile format](https://github.com/Carthage/Carthage/blob/master/Documentation/README.md). |
| `js-bower` | Bower registry/GitHub | Bower packages may be named packages, Git endpoints, URLs, or local paths. [Bower specification](https://bower.io/docs/api/#install). |
| `js-importmap` | URL CDNs, commonly JSPM/unpkg | Import maps map names to arbitrary URLs; they declare no registry. [Import maps specification](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/script/type/importmap). |
| `jsonnet-bundler`, `jsonnet-lock` | GitHub is common | jsonnet-bundler dependencies are Git repositories or local paths. [jb documentation](https://github.com/jsonnet-bundler/jsonnet-bundler#jsonnet-bundler). |
| `nix-default-shell`, `nix-flake`, `nix-flake-lock` | [Nix flake registry](https://nixos.org/manual/nix/stable/command-ref/new-cli/nix3-registry) defaults and GitHub | Flake inputs are arbitrary URLs/paths; a lock file records those exact inputs. |
| `ocaml-esy` | npm registry and opam packages | esy resolves npm packages and can also consume Git/local/opam sources. [esy manifest](https://esy.sh/docs/en/manifest.html). |
| `perl-build-pl`, `perl-cpanfile`, `perl-cpanfile-snapshot`, `perl-makefile-pl` | [CPAN](https://metacpan.org/) | Perl metadata can also refer to local, Git, or non-CPAN distributions. |
| `python-conda-env-alt`, `python-conda-environment` | conda-forge/defaults plus PyPI for `pip:` | Conda environment files choose ordered channels and can mix pip packages. [Conda environment guide](https://docs.conda.io/projects/conda/en/latest/user-guide/tasks/manage-environments.html). |
| `r-renv-lock` | CRAN | renv also records Bioconductor, GitHub, GitLab, URL, and local sources. [renv lockfiles](https://rstudio.github.io/renv/reference/lockfile.html). |
| `raku-meta` | Raku ecosystems such as zef/fez | META6 identifiers do not fix a unique registry; distributions may come from multiple ecosystems or Git. [Raku META6 specification](https://design.raku.org/S22.html). |
| `ruby-appraisal` | RubyGems, indirectly | An Appraisal generates Gemfiles; it is not itself the dependency manifest. [Appraisal README](https://github.com/thoughtbot/appraisal). |
| `swift-package`, `swift-package-resolved` | [Swift Package Registry](https://github.com/swiftlang/swift-evolution/blob/main/proposals/0292-package-registry-service.md) and Git | Swift packages can be registry identities or source-control URLs; registries are configurable. |
| `zig-build-zon` | No public registry | `build.zig.zon` dependencies are URL/path references plus hashes. [Zig package manager documentation](https://ziglang.org/learn/build-system/). |

## These files do not declare external dependencies

Their surrounding build may use dependencies, but the file named by this
detector is configuration, workspace metadata, or a local-module declaration.
An analyzer should normally report **no external dependency** from this file,
not guess a registry.

| Detector IDs | Reason |
| --- | --- |
| `cpp-autotools`, `cpp-cmake`, `cpp-cmake-modules` | Build scripts can download/find arbitrary packages, but there is no package-source standard. |
| `dotnet-directory-build` | Shared MSBuild properties, not package declarations. |
| `foundry-remappings` | Local import aliases only. |
| `go-work` | Lists local Go module directories; external requirements are in `go.mod`. [Go workspaces](https://go.dev/ref/mod#workspaces). |
| `godot-plugin-cfg` | Godot plugin metadata; no package resolver is specified. |
| `java-gradle-settings`, `java-gradle-settings-kts` | Gradle settings may configure plugin/dependency repositories, but do not declare the project's library dependencies. [Gradle settings](https://docs.gradle.org/current/userguide/settings_file_basics.html). |
| `js-lerna`, `js-nx`, `js-pnpm-workspace`, `js-rush`, `js-turbo` | Monorepo/task configuration; package dependencies live in package manifests and lockfiles. |
| `js-npmrc`, `js-yarnrc` | Registry/client configuration, not dependency declarations. They may explicitly select a custom registry. |
| `unreal-uplugin`, `unreal-uproject` | Unreal project/plugin descriptors list engine modules/plugins, generally local, engine, or Marketplace-managed; no standard dependency registry URL is declared. |
| `zig-build` | Build script; package metadata is normally in `build.zig.zon`. |

## Notes for implementation

* The same `registry` result should not be forced onto a lockfile if its
  entries carry a real host/URL. Extract the actual source string first.
* “Public default” is useful for understanding an unqualified package name;
  it is not evidence that a particular project uses that public service.
* `buf-lock` is a special case with valid empty `deps`; absence of entries is
  not a malformed dependency source.
