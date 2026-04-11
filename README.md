# deplens

`deplens` is a small CLI for scanning a directory tree and reporting dependency-related manifests it finds. It is aimed at mixed-language repositories and can detect both standard manifest files and rule-based sources such as Terraform resources or HTML pages that load external scripts.

By default, the tool walks the target directory recursively, skips common generated/vendor directories, and prints a summary plus path-first manifest details with explicit dependency-status labels. The default detector rules are embedded into the binary at build time. The tool can also emit JSON for machine-readable consumption and load additional detectors from a custom YAML rules file passed with `--rules`.

JSON output contains a top-level `root` plus `manifests`. Each manifest entry includes `type`, `path`, optional `dependencies`, and `has_dependencies`. Each dependency is emitted as an object with `name` and optional `section`. `has_dependencies` is `null` when the detector cannot determine dependency presence, `true` when extraction confirmed at least one dependency, and `false` when a detector or extractor conclusively matched but found none.

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
| filename regex match | Built-in filename rules: `*requirements*.txt`, `*requirements*.in`, `uv.lock`, `poetry.lock`, `Pipfile.lock`, `pdm.lock`, `conda-lock.yml`, `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock`, `bun.lockb`, `deno.lock`, `bower.json`, `npm-shrinkwrap.json`, `gradle.lockfile`, `build.gradle`, `build.gradle.kts`, `settings.gradle`, `settings.gradle.kts`, `Gemfile`, `Gemfile.lock`, `*.gemspec`, `Package.swift`, `Podfile`, `Cartfile`, `composer.lock`, `pubspec.lock`, `rebar.config`, `rebar.lock`, `deps.edn`, `project.clj`, `stack.yaml`, `stack.yaml.lock`, `cabal.project`, `*.cabal`, `package.yaml`, `packages.config`, `packages.lock.json`, `Directory.Packages.props`, `paket.dependencies`, `paket.lock`, `go.mod`, `go.sum`, `go.work`, `Gopkg.toml`, `glide.yaml`, `Cargo.lock`, `*.csproj`, `Gopkg.lock`, `glide.lock`, `conanfile.txt`, `conan.lock`, `vcpkg.json`, `Package.resolved`, `Podfile.lock`, `mix.exs`, `mix.lock`, `Project.toml`, `Manifest.toml`, `cpanfile`, `build.zig.zon`, `*.nimble`, `*.opam`, `shard.yml`, `gleam.toml`, `v.mod`, `Chart.yaml`, `requirements.yml`, `requirements.yaml`, `buf.yaml`, `Brewfile`, `jsonnetfile.json`, `.terraform.lock.hcl` | No | 1 |
| path glob match | Built-in path-glob rules such as `python-requirements-dir` for `**/requirements/*.txt` | No | 1 |
| json presence check | `package.json`; reports dependency presence when any of `dependencies`, `devDependencies`, `peerDependencies`, or `optionalDependencies` is a non-empty object. Also used for `composer.json` via `require` / `require-dev`, and `deno.json` / `deno.jsonc` via `imports` | No | 2 |
| xml presence check | `pom.xml`; reports dependency presence when any configured element path exists, for example `project.dependencies.dependency`; XML namespaces are ignored for matching | No | 2 |
| toml presence check | `Cargo.toml`; reports dependency presence when any of `dependencies`, `dev-dependencies`, `build-dependencies`, `workspace.dependencies`, `target.*.dependencies`, `target.*.dev-dependencies`, or `target.*.build-dependencies` is a non-empty table | No | 2 |
| toml | TOML files matched by a rule such as built-in `python-pyproject` for `pyproject.toml`; extracts from `build-system.requires[]`, `project.dependencies[]`, `project.optional-dependencies.*[]`, `dependency-groups.*[]`, `tool.poetry.dependencies`, and `tool.poetry.group.*.dependencies` | Yes | 3 |
| pipfile | `Pipfile` matched by the built-in `python-pipfile` rule; reports only when the file contains at least one dependency-bearing package section such as `[packages]`, `[dev-packages]`, or a custom package category like `[docs]` | Yes | 3 |
| python call | Python files matched by a rule such as built-in `python-setup-py` for `setup.py`; detects imported function calls with specific keyword arguments, for example `setuptools.setup(..., install_requires=..., extras_require=...)`, and can extract from simple literal arrays in `install_requires=[...]` plus `extras_require={"group": [...]}` | Yes | 3 |
| ini | INI files matched by a rule such as built-in `python-setup-cfg` for `setup.cfg`; extracts from `[options]` keys `setup_requires` and `install_requires`, plus all keys under `[options.extras_require]`, when values are written as static multiline lists | Yes | 3 |
| banner regex | JavaScript files whose first 4096 bytes match a configured `banner-regex` with capture groups 1 and 2 for package name and version | Yes | 3 |
| yaml presence check | `pubspec.yaml`; reports dependency presence when any of `dependencies`, `dev_dependencies`, or `dependency_overrides` is present and non-empty | No | 2 |
| yaml | Path expression such as `workflow.steps[].config.packages.pip[]` to extract data from yaml files, or a presence check such as `dependencies` to detect files that declare a dependency section without extracting it | Sometimes | 3 |
| html external scripts | HTML-like files (`.html`, `.htm`, `.xhtml`, `.tmpl`, `.gohtml`, `.mustache`, `.hbs`, `.njk`) matched by the built-in `html-external-scripts` rule; extracts remote URLs from external `<script src="https://...">` tags, `<script type="module">` imports, and `<script type="importmap">` `imports` entries | Yes | 3 |
| terraform | Terraform `.tf` files with parsing content. For example containing a `aws_glue_job` resource with `default_arguments.--job-language = "python"` and `default_arguments.--additional-python-modules` present | No | 2 |
| typescript cdk construct | TypeScript `.ts` files parsed with an AST. For example containing `new glue.CfnJob(..., { defaultArguments: { "--job-language": "python", "--additional-python-modules": "pandas==2.2.1" }})` imported from `aws-cdk-lib/aws-glue` | Yes | 3 |
| python cdk construct | Python `.py` files with statically-resolved CDK `CfnJob(...)` calls. For example containing `glue.CfnJob(..., default_arguments={"--job-language": "python", "--additional-python-modules": "pandas==2.2.1"})` imported from `aws_cdk.aws_glue` | Yes | 3 |

The same maturity model applies to custom rules passed with `--rules`; selector-only rules are level 1, presence-check rules such as `json.exists-any`, `xml.exists-any`, `toml.table-exists-any`, and `yaml.exists-any` are level 2, extraction rules are level 3, and level 4 is reserved for future normalized output.

Default JavaScript banner rules use `filename-regex: '.*\.js$'` and return `name@version` from `banner-regex` capture groups 1 and 2. The built-in banner rule set includes `js-banner-block-start`, `js-banner-plain-block-start`, `js-banner-multiline-preserved`, `js-banner-line-comment`, and `js-banner-version-tagged`.

The default Python requirements rules include both a filename selector for `*requirements*.txt` and `*requirements*.in`, plus a path selector for `**/requirements/*.txt`.

The default rules also include `python-conda-environment` for `environment.yml` and `environment.yaml`, which reports the file only when a top-level `dependencies` key is present.

Several additional ecosystem-specific filenames and extensions are also tracked at Level 1 only, including `mix.exs`, `*.gemspec`, `*.cabal`, `package.yaml`, `conanfile.txt`, `vcpkg.json`, `Project.toml`, `Manifest.toml`, `cpanfile`, `build.zig.zon`, `*.nimble`, `*.opam`, `shard.yml`, `gleam.toml`, `v.mod`, `Chart.yaml`, `requirements.yml`, `buf.yaml`, `Brewfile`, `jsonnetfile.json`, and `.terraform.lock.hcl`. These rules identify candidate dependency files but do not yet determine dependency presence or extract dependency data.

When `Pipfile` is present, it is reported as `python-pipfile` only if at least one dependency-bearing package section exists, for example `[packages]`, `[dev-packages]`, or a custom package-category section. Extracted dependencies are emitted from those sections, while metadata sections such as `[[source]]` and `[requires]` are ignored.

## Example

```bash
go run ./cmd/deplens ./testdata/sample-monorepo
```

Representative output:

```text
Root: /path/to/project

Found 24 manifests:
- 24 with dependency status unknown

apps/backend/requirements/base.txt [dependency status unknown]

backend/Pipfile.lock [matched]

backend/uv.lock [matched]

go-service/go.sum [matched]

requirements.txt [matched]
```

When dependencies are extracted from `pyproject.toml`, the output is grouped by section:

```text
pyproject.toml [3 deps]
  project.dependencies:
    - requests>=2.31
  project.optional-dependencies.dev:
    - pytest>=8
    - ruff>=0.4
```

When a Conda environment file contains a top-level `dependencies` key but the detector does not extract the individual entries, it is reported with an explicit status label:

```text
environment.yml [dependencies present, not extracted]
```

When `package.json` contains at least one dependency declaration section with entries, it is also reported with an explicit status label:

```text
package.json [dependencies present, not extracted]
```

Before this change, several structured manifests such as `composer.json`, `deno.json`, `deno.jsonc`, `Cargo.toml`, and `pubspec.yaml` were reported only as `[matched]`. They now report a conclusive Level 2 status. For example:

```text
# Before
composer.json [matched]
Cargo.toml [matched]

# After
composer.json [dependencies present, not extracted]
Cargo.toml [no dependencies]
```

`pom.xml` now also reports a conclusive Level 2 status instead of a generic match:

```text
# Before
pom.xml [matched]

# After, when direct Maven dependencies are declared
pom.xml [dependencies present, not extracted]

# After, when no direct Maven dependencies are declared and --show-empty is used
pom.xml [no dependencies]
```

When you need to audit empty matches as well, use `--show-empty` to include entries whose dependency status is conclusively empty:

```text
setup.cfg [no dependencies]
```

That also applies to `package.json` files that do not contain any non-empty `dependencies`, `devDependencies`, `peerDependencies`, or `optionalDependencies` sections:

```text
package.json [no dependencies]
```

When `setup.py` contains a `setuptools.setup(...)` call with `install_requires` or `extras_require`, extracted dependencies render either as a flat list or grouped by section. For example:

```text
setup.py [2 deps]
  install_requires:
    - requests>=2.31
  extras_require.dev:
    - pytest>=8
```

When `setup.cfg` contains declarative setuptools dependency keys such as `[options] install_requires`, `[options] setup_requires`, or entries under `[options.extras_require]`, static multiline values are extracted and rendered similarly:

```text
setup.cfg [2 deps]
  options.install_requires:
    - requests>=2.31
  options.extras_require.dev:
    - pytest>=8
```

When dependencies are extracted without section metadata, the output stays flat:

```text
requirements.txt [2 deps]
  - requests>=2.31
  - pendulum>=3
```

When a manifest mixes sectioned and unsectioned dependencies, the unsectioned entries are placed under `[default group]`:

```text
mixed.toml [3 deps]
  [default group]
    - build>=1.2
  tool.custom.dev:
    - pytest>=8
  tool.custom.docs:
    - mkdocs>=1.6
```
