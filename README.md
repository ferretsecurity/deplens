# deplens

`deplens` is a small CLI for scanning a directory tree and reporting dependency-related manifests it finds. It is aimed at mixed-language repositories and can detect both standard manifest files and rule-based sources such as Terraform resources or HTML pages that load external scripts.

By default, the tool walks the target directory recursively, skips common generated/vendor directories, and prints a summary plus path-first manifest details with explicit dependency-status labels. The default detector rules are embedded into the binary at build time. The tool can also emit JSON for machine-readable consumption and load additional detectors from a custom YAML rules file passed with `--rules`.

JSON output contains a top-level `root` plus `manifests`. Each manifest entry includes `type`, `path`, optional `dependencies`, `has_dependencies`, and optional `warnings`. Each dependency is emitted as an object with `name` and optional `section`. `has_dependencies` is `null` when the detector cannot determine dependency presence, `true` when extraction confirmed at least one dependency, and `false` when a detector or extractor conclusively matched but found none.

Human-readable output starts with summary counts and then prints one block per manifest path. By default, manifests that were conclusively matched but found to have no dependencies are counted in the summary and omitted from the detailed list; pass `--show-empty` to include them. Files with extracted dependencies show either a flat list or sectioned groups. If a file mixes sectioned and unsectioned dependencies, the unsectioned entries are rendered under `[default group]`.

For project-specific terminology used in the CLI documentation and output descriptions, see [docs/glossary.md](docs/glossary.md).

## Detector Maturity Model

`deplens` uses a detector maturity model to describe detector capability. The maturity level applies to detector capability itself, not the outcome for an individual matched file. Per-file outcomes are still represented by scan results such as `has_dependencies` and `dependencies`.

The maturity levels are:

- Level 1: the detector can identify candidate files.
- Level 2: the detector can determine whether a matched file contains dependency declarations.
- Level 3: the detector can extract dependency data in a detector-specific form.
- Level 4: the detector can extract dependency data into a normalized cross-detector format.

No current detector is level 4 because `deplens` does not yet define a shared normalized dependency schema.

## Supported Detectors

Rules may use `filename-regex`, `path-glob`, or both. When both selectors are present on the same rule, they are combined with AND semantics, so the file must match both conditions before the detector runs.

Built-in detectors:

| Detector | Matches | Extracts dependencies | Maturity |
| --- | --- | --- | --- |
| filename regex match | Built-in filename rules: `Pipfile.lock`, `pdm.lock`, `conda-lock.yml`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock`, `bun.lockb`, `deno.lock`, `bower.json`, `npm-shrinkwrap.json`, `gradle.lockfile`, `build.gradle`, `build.gradle.kts`, `settings.gradle`, `settings.gradle.kts`, `Gemfile`, `Gemfile.lock`, `*.gemspec`, `Package.swift`, `Podfile`, `Cartfile`, `pubspec.lock`, `rebar.config`, `rebar.lock`, `deps.edn`, `project.clj`, `stack.yaml`, `stack.yaml.lock`, `cabal.project`, `*.cabal`, `package.yaml`, `packages.lock.json`, `paket.dependencies`, `paket.lock`, `go.sum`, `go.work`, `Gopkg.toml`, `glide.yaml`, `Cargo.lock`, `Gopkg.lock`, `glide.lock`, `conanfile.txt`, `conan.lock`, `vcpkg.json`, `Package.resolved`, `Podfile.lock`, `mix.exs`, `mix.lock`, `Manifest.toml`, `cpanfile`, `build.zig.zon`, `*.nimble`, `*.opam`, `v.mod`, `Brewfile`, `.terraform.lock.hcl` | No | 1 |
| path glob match | Selector-only path-glob rules, for example a custom rule such as `apps/**/package.json` | No | 1 |
| json presence check | `package.json`; reports dependency presence when any of `dependencies`, `devDependencies`, `peerDependencies`, or `optionalDependencies` is a non-empty object. Also used for `composer.json` via `require` / `require-dev`, `deno.json` / `deno.jsonc` via `imports`, `Packages/manifest.json` via `dependencies`, and `jsonnetfile.json` via a non-empty `dependencies` array | No | 2 |
| package lock | `package-lock.json`; extracts versioned root project dependencies from lockfile version 1 `dependencies`, and from lockfile version 2 or 3 root-package `packages[""].dependencies` plus `optionalDependencies` | Yes | 3 |
| composer lock | `composer.lock`; extracts package entries from `packages[]` and `packages-dev[]`, emitting `name@version` when a version is available | Yes | 3 |
| xml presence check | `pom.xml`; reports dependency presence when any configured element path exists, for example `project.dependencies.dependency`; XML namespaces are ignored for matching. Also used for `*.csproj` via `Project.ItemGroup.PackageReference`, `Directory.Packages.props` via `Project.ItemGroup.PackageVersion`, and `packages.config` via `packages.package` | No | 2 |
| toml presence check | `Cargo.toml`; reports dependency presence when any of `dependencies`, `dev-dependencies`, `build-dependencies`, `workspace.dependencies`, `target.*.dependencies`, `target.*.dev-dependencies`, or `target.*.build-dependencies` is a non-empty table. Also used for `Project.toml` via `[deps]` and `gleam.toml` via `[dependencies]` | No | 2 |
| go mod | `go.mod`; extracts direct dependencies from `require` directives and ignores `replace` plus indirect-only requirements | Yes | 3 |
| toml | TOML files matched by a rule such as built-in `python-pyproject` for `pyproject.toml`; extracts from `build-system.requires[]`, `project.dependencies[]`, `project.optional-dependencies.*[]`, `dependency-groups.*[]`, `tool.poetry.dependencies`, and `tool.poetry.group.*.dependencies` | Yes | 3 |
| pipfile | `Pipfile` matched by the built-in `python-pipfile` rule; reports only when the file contains at least one dependency-bearing package section such as `[packages]`, `[dev-packages]`, or a custom package category like `[docs]` | Yes | 3 |
| py requirements | Pip requirements files matched by built-in `python-requirements` and `python-requirements-dir`; extracts static non-empty, non-comment requirement lines from files such as `requirements.txt`, `requirements.in`, and `requirements/base.txt`, recursively expands local `-r`, `--requirement`, and `--requirements` includes, and ignores directives such as `-c`, `--index-url`, and `--hash` | Yes | 3 |
| poetry lock | `poetry.lock` matched by the built-in `python-poetry-lock` rule; extracts retained `[[package]]` entries with `name` and `version`, ignores `category`, `groups`, `optional`, and `markers`, skips self-style directory entries plus git-sourced packages, deduplicates exact `name==version` repeats, and reports conclusive empty files when only metadata or filtered entries remain | Yes | 3 |
| uv lock | `uv.lock` matched by the built-in `python-uv` rule; extracts retained package entries from `[[package]]` records, ignores self-style editable/workspace/virtual entries, and reports conclusive empty files when only `version = 1` is present | Yes | 3 |
| python call | Python files matched by a rule such as built-in `python-setup-py` for `setup.py`; detects imported function calls with specific keyword arguments, for example `setuptools.setup(..., install_requires=..., extras_require=...)`, and can extract from simple literal arrays in `install_requires=[...]` plus `extras_require={"group": [...]}` | Yes | 3 |
| ini | INI files matched by a rule such as built-in `python-setup-cfg` for `setup.cfg`; extracts from `[options]` keys `setup_requires` and `install_requires`, plus all keys under `[options.extras_require]`, when values are written as static multiline lists | Yes | 3 |
| banner regex | JavaScript files whose first 4096 bytes match a configured `banner-regex` with capture groups 1 and 2 for package name and version | Yes | 3 |
| yaml presence check | `pubspec.yaml`; reports dependency presence when any of `dependencies`, `dev_dependencies`, or `dependency_overrides` is present and non-empty. Also used for `Chart.yaml` and `shard.yml` via a non-empty top-level `dependencies` key, `buf.yaml` via `deps`, and Ansible `requirements.yml` / `requirements.yaml` via non-empty `roles` or `collections` | No | 2 |
| yaml | Path expression such as `workflow.steps[].config.packages.pip[]` to extract data from yaml files, or a presence check such as `dependencies` to detect files that declare a dependency section without extracting it | Sometimes | 3 |
| html external scripts | HTML-like files (`.html`, `.htm`, `.xhtml`, `.tmpl`, `.gohtml`, `.mustache`, `.hbs`, `.njk`) matched by the built-in `html-external-scripts` rule; extracts remote URLs from external `<script src="https://...">` tags, `<script type="module">` imports, and `<script type="importmap">` `imports` entries | Yes | 3 |
| terraform | Terraform `.tf` files with parsing content. For example containing a `aws_glue_job` resource with `default_arguments.--job-language = "python"` and `default_arguments.--additional-python-modules` present | No | 2 |
| typescript cdk construct | TypeScript `.ts` files parsed with an AST. For example containing `new glue.CfnJob(..., { defaultArguments: { "--job-language": "python", "--additional-python-modules": "pandas==2.2.1" }})` imported from `aws-cdk-lib/aws-glue` | Yes | 3 |
| python cdk construct | Python `.py` files with statically-resolved CDK `CfnJob(...)` calls. For example containing `glue.CfnJob(..., default_arguments={"--job-language": "python", "--additional-python-modules": "pandas==2.2.1"})` imported from `aws_cdk.aws_glue` | Yes | 3 |

The same maturity model applies to custom rules passed with `--rules`; selector-only rules are level 1, presence-check rules such as `json.exists-any`, `xml.exists-any`, `toml.table-exists-any`, and `yaml.exists-any` are level 2, extraction rules are level 3, and level 4 is reserved for future normalized output.

Default JavaScript banner rules use `filename-regex: '.*\.js$'` and return `name@version` from `banner-regex` capture groups 1 and 2. The built-in banner rule set includes `js-banner-block-start`, `js-banner-plain-block-start`, `js-banner-multiline-preserved`, `js-banner-line-comment`, and `js-banner-version-tagged`.

The default Python requirements rules use the `py-requirements` detector for both a filename selector matching `*requirements*.txt` and `*requirements*.in`, plus a path selector for `**/requirements/*.txt`. The detector extracts static dependency lines, joins trailing `\` continuations, ignores blank lines and `#` comments, recursively resolves local `-r`, `--requirement`, and `--requirements` includes relative to the including file, and skips non-dependency directives such as `-c`, `--constraint`, `--index-url`, `--extra-index-url`, `--find-links`, `--trusted-host`, and `--hash`. If an included file cannot be read or an include cycle is detected, the manifest is still reported and a warning is attached to the result.

The default `js-npm-lock` rule now uses the dedicated `package-lock` detector. It extracts only the root project's declared dependencies, not every transitive `node_modules/...` entry in the lockfile. For lockfile version 1, dependencies come from the top-level `dependencies` object and are emitted as `name@version` when a version is available. For lockfile versions 2 and 3, dependency names come from the root package entry at `packages[""]`, including both `dependencies` and `optionalDependencies`, and versions are resolved from the matching `packages["node_modules/<name>"]` entries. Duplicates are removed. If a version cannot be resolved, `deplens` still emits the package name.

The default `python-poetry-lock` rule now uses the dedicated `poetry-lock` detector. It extracts retained package entries from `poetry.lock`, ignores `category`, `groups`, `optional`, and `markers`, skips self-style directory entries and git-sourced packages, deduplicates exact duplicate `name==version` entries, and reports `has_dependencies=false` when the file is metadata-only or all package entries are filtered out.

The default `python-uv` rule now uses the dedicated `uv-lock` detector. It extracts retained package entries from `uv.lock`, skips self-style editable/workspace/virtual entries, and reports `has_dependencies=false` when the file is only a version marker.

The default rules also include `python-conda-environment` for `environment.yml` and `environment.yaml`, which reports the file only when a top-level `dependencies` key is present.

Several additional ecosystem-specific filenames and extensions are still tracked at Level 1 only, including `mix.exs`, `*.gemspec`, `*.cabal`, `package.yaml`, `conanfile.txt`, `vcpkg.json`, `Manifest.toml`, `cpanfile`, `build.zig.zon`, `*.nimble`, `*.opam`, `v.mod`, `Brewfile`, and `.terraform.lock.hcl`. These rules identify candidate dependency files but do not yet determine dependency presence or extract dependency data.

The default rules now treat `buf.yaml` as a level 2 detector by checking for a non-empty top-level `deps` key, Ansible `requirements.yml` / `requirements.yaml` as level 2 by checking for non-empty `roles` or `collections`, and `jsonnetfile.json` as level 2 by checking for a non-empty `dependencies` array.

When `Pipfile` is present, it is reported as `python-pipfile` only if at least one dependency-bearing package section exists, for example `[packages]`, `[dev-packages]`, or a custom package-category section. Extracted dependencies are emitted from those sections, while metadata sections such as `[[source]]` and `[requires]` are ignored.

## Example

```bash
go run ./cmd/deplens ./testdata/sample-monorepo
```

Example output:

```text
Root: /path/to/project

Found 32 manifests:
- 4 with extracted dependencies
- 2 confirmed empty
- 3 with dependencies present, not extracted
- 23 with dependency status unknown

requirements.txt [2 deps]
  - requests>=2.31
  - pendulum>=3

pyproject.toml [3 deps]
  project.dependencies:
    - requests>=2.31
  project.optional-dependencies.dev:
    - pytest>=8
    - ruff>=0.4

package.json [dependencies present, not extracted]
go.mod [1 dep]
  - github.com/stretchr/testify
go.sum [matched]
setup.cfg [no dependencies]
```

For `package-lock.json`, older filename-only behavior reported the file as matched without extracting dependencies:

```text
package-lock.json [matched]
```

With the default `package-lock` detector, the same root project dependencies are extracted. The `testdata/javascript` fixtures include examples for lockfile versions 1, 2, and 3:

```text
package-lock-v1-with-deps/package-lock.json [2 deps]
  - left-pad@1.3.0
  - lodash@4.17.21

package-lock-v2-with-deps/package-lock.json [3 deps]
  - @types/node@20.12.7
  - left-pad@1.3.0
  - lodash@4.17.21

package-lock-v3-with-deps/package-lock.json [3 deps]
  - @types/node@20.12.7
  - left-pad@1.3.0
  - lodash@4.17.21
```

For `composer.lock`, older filename-only behavior reported the file as matched without extracting dependencies:

```text
composer.lock [matched]
```

With the default `composer-lock` detector, package entries are extracted from `packages[]` and `packages-dev[]`. In `testdata/sample-monorepo/php-app` the same file is now reported as:

```text
composer.lock [1 dep]
  - monolog/monolog@3.6.0
```
