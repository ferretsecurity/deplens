# Dependency Coverage

Cross-reference of every file format catalogued in `docs/planning/ResearchDeps.md` Step 3 against what deplens currently implements, at what maturity level, and where the implementation lives.

## Maturity Levels

| Level | Meaning |
|-------|---------|
| - | Not implemented; file is not recognized |
| 1 | Detected by filename/path match only; no content analysis |
| 2 | Presence check; confirms whether dependency declarations exist |
| 3 | Extracts dependency data (name, version, section, etc.) |
| 4 | Reserved — normalized cross-detector output (not yet defined) |

## Legend for Implementation Column

- **filename match** — rule with `filename-regex` or `path-glob` only (level 1)
- **json presence** / **yaml presence** / **xml presence** / **toml presence** — existence check via structured parser (level 2)
- **terraform resource check** — Terraform HCL parsed for specific resource conditions (level 2)
- Named parsers (e.g. `py-requirements`, `cargo-lock`) — dedicated extractor (level 3)
- **toml queries** — TOML parser with dot-path queries (level 3)
- **ini queries** — INI parser extracting specific section/key values (level 3)
- **python call** — Python AST matching for specific function calls (level 3)
- **banner-regex** — reads first 4096 bytes for a banner comment matching name+version (level 3)
- **html external-scripts** — parses `<script src>`, ES module imports, and importmap entries (level 3)
- **typescript cdk** — TypeScript AST parser for CDK construct calls (level 3)
- **python cdk** — Python AST parser for CDK construct calls (level 3)

---

## Comprehensive Table

| # | Language / Tool | Dep Storage Type | File Path & Name | Notes | Level | Implementation | Code | Tests | Testdata |
|---|----------------|-----------------|-----------------|-------|-------|---------------|------|-------|----------|
| 1 | Python (pip) | Manifest | `requirements.txt` | Flat list of `pkg==version`. Split variants (`requirements-dev.txt`, etc.) also matched. | 3 | `py-requirements` | [py_requirements.go](internal/analyze/py_requirements.go) | [py_requirements_test.go](internal/analyze/py_requirements_test.go) | [testdata/python/requirements-*](testdata/python/) |
| 2 | Python (pip) | Manifest | `constraints.txt` | Version constraints without declaring deps. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/python/constraints](testdata/python/constraints) |
| 3 | Python (pip-tools) | Manifest (source) | `requirements.in` | Source file for pip-compile; matched by the same filename-regex as `requirements.txt`. | 3 | `py-requirements` | [py_requirements.go](internal/analyze/py_requirements.go) | [py_requirements_test.go](internal/analyze/py_requirements_test.go) | [testdata/python/requirements-*](testdata/python/) |
| 4 | Python (pip-tools) | Lockfile | `requirements.txt` | Output of pip-compile — same filename as row 1. Covered by the same rule. | 3 | `py-requirements` | [py_requirements.go](internal/analyze/py_requirements.go) | [py_requirements_test.go](internal/analyze/py_requirements_test.go) | — |
| 5 | Python (setuptools) | Manifest | `setup.py` | Detects `setuptools.setup()` calls with `install_requires` / `extras_require` keyword args. | 3 | `python call` | [python.go](internal/analyze/python.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/python/setup-py-*](testdata/python/) |
| 6 | Python (setuptools) | Manifest | `setup.cfg` | Reads `[options]` → `install_requires`, `setup_requires`, and `[options.extras_require]` keys. | 3 | `ini queries` | [ini.go](internal/analyze/ini.go) | [ini_test.go](internal/analyze/ini_test.go) | [testdata/python/setup-cfg-*](testdata/python/) |
| 7 | Python (PEP 621) | Manifest | `pyproject.toml` | Queries `project.dependencies[]`, `project.optional-dependencies.*[]`, `dependency-groups.*[]`, `build-system.requires[]`. | 3 | `toml queries` | [toml.go](internal/analyze/toml.go) | [toml_test.go](internal/analyze/toml_test.go) | [testdata/toml/pyproject/](testdata/toml/pyproject/) |
| 8 | Python (Poetry) | Manifest | `pyproject.toml` | Additional queries: `tool.poetry.dependencies`, `tool.poetry.group.*.dependencies`. Same rule as row 7. | 3 | `toml queries` | [toml.go](internal/analyze/toml.go) | [toml_test.go](internal/analyze/toml_test.go) | [testdata/toml/pyproject/](testdata/toml/pyproject/) |
| 9 | Python (Poetry) | Lockfile | `poetry.lock` | Extracts `[[package]]` entries; ignores self-referencing path deps; deduplicates; emits git-sourced packages with extras. | 3 | `poetry-lock` | [poetry_lock.go](internal/analyze/poetry_lock.go) | [poetry_lock_test.go](internal/analyze/poetry_lock_test.go) | [testdata/python/poetry-lock-*](testdata/python/) |
| 10 | Python (Pipenv) | Manifest | `Pipfile` | TOML table-query extracting all package-bearing sections (`[packages]`, `[dev-packages]`, custom categories). | 3 | `toml queries` | [toml.go](internal/analyze/toml.go) | [toml_test.go](internal/analyze/toml_test.go) | [testdata/toml/pipfile*](testdata/toml/) |
| 11 | Python (Pipenv) | Lockfile | `Pipfile.lock` | Extracts `default` and `develop` sections as grouped `name==version` deps. | 3 | `pipfile-lock` | [pipfile_lock.go](internal/analyze/pipfile_lock.go) | [pipfile_lock_test.go](internal/analyze/pipfile_lock_test.go), [pipfile_lock_scan_test.go](internal/analyze/pipfile_lock_scan_test.go) | [testdata/python/pipfile-lock-*](testdata/python/) |
| 12 | Python (PDM) | Manifest | `pyproject.toml` | Uses PEP 621 format; covered by same TOML queries as rows 7–8. | 3 | `toml queries` | [toml.go](internal/analyze/toml.go) | [toml_test.go](internal/analyze/toml_test.go) | — |
| 13 | Python (PDM) | Lockfile | `pdm.lock` | Checks for a non-empty top-level `package` array. | 2 | `toml presence` | [toml.go](internal/analyze/toml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/python/pdm-lock-*](testdata/python/) |
| 14 | Python (Hatch) | Manifest | `pyproject.toml` | PEP 621 format; covered by same TOML queries as rows 7–8. | 3 | `toml queries` | [toml.go](internal/analyze/toml.go) | [toml_test.go](internal/analyze/toml_test.go) | — |
| 15 | Python (uv) | Manifest | `pyproject.toml` | PEP 621 compatible; covered by same TOML queries as rows 7–8. | 3 | `toml queries` | [toml.go](internal/analyze/toml.go) | [toml_test.go](internal/analyze/toml_test.go) | — |
| 16 | Python (uv) | Lockfile | `uv.lock` | Extracts `[[package]]` entries; skips self-style editable/virtual entries. | 3 | `uv-lock` | [uv_lock.go](internal/analyze/uv_lock.go) | [uv_lock_test.go](internal/analyze/uv_lock_test.go) | [testdata/python/uv-lock-*](testdata/python/) |
| 17 | Python (Conda) | Manifest | `environment.yml` | Checks for non-empty top-level `dependencies` key. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/python/conda-environment/](testdata/python/conda-environment/) |
| 18 | Python (Conda) | Manifest | `conda.yaml` | Alternative name. Both `conda.yml` and `conda.yaml` now matched by `python-conda-env-alt`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/python/conda-env-alt](testdata/python/conda-env-alt) |
| 19 | Python (conda-lock) | Lockfile | `conda-lock.yml` | Checks for non-empty top-level `package` list. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/python/conda-lock-*](testdata/python/) |
| 20 | Python | Vendored | `vendor/` or `_vendor/` | Directory of vendored Python packages. Not a file; no rule. | - | Not implemented | — | — | — |
| 21 | JavaScript (npm) | Manifest | `package.json` | Checks for non-empty `dependencies`, `devDependencies`, `peerDependencies`, or `optionalDependencies`. | 2 | `json presence` | [json.go](internal/analyze/json.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/js/package-json-*](testdata/js/) |
| 22 | JavaScript (npm) | Lockfile | `package-lock.json` | Extracts all packages from lockfile v1 (nested deps), v2/v3 (`node_modules/*` entries including transitive). | 3 | `package-lock` | [package_lock.go](internal/analyze/package_lock.go) | [package_lock_test.go](internal/analyze/package_lock_test.go) | [testdata/javascript/package-lock-*](testdata/javascript/) |
| 23 | JavaScript (npm) | Legacy lockfile | `npm-shrinkwrap.json` | Same `package-lock` parser reused; identical lockfile structure. | 3 | `package-lock` | [package_lock.go](internal/analyze/package_lock.go) | [package_lock_test.go](internal/analyze/package_lock_test.go) | [testdata/javascript/npm-shrinkwrap-*](testdata/javascript/) |
| 24 | JavaScript (Yarn v1) | Lockfile | `yarn.lock` | Extracts deduplicated `name@version` from classic Yarn v1 entries. | 3 | `yarn-lock` | [yarn_lock.go](internal/analyze/yarn_lock.go) | [yarn_lock_test.go](internal/analyze/yarn_lock_test.go), [yarn_lock_scan_test.go](internal/analyze/yarn_lock_scan_test.go) | [testdata/javascript/yarn-lock-v1-*](testdata/javascript/) |
| 25 | JavaScript (Yarn Berry v2+) | Lockfile | `yarn.lock` | Extracts from `__metadata`-prefixed modern lockfiles; same filename as v1. | 3 | `yarn-lock` | [yarn_lock.go](internal/analyze/yarn_lock.go) | [yarn_lock_test.go](internal/analyze/yarn_lock_test.go), [yarn_lock_scan_test.go](internal/analyze/yarn_lock_scan_test.go) | [testdata/javascript/yarn-lock-modern-*](testdata/javascript/) |
| 26 | JavaScript (Yarn Berry) | Vendored | `.yarn/cache/` | Zero-installs directory. Not a file; no rule. | - | Not implemented | — | — | — |
| 27 | JavaScript (Yarn Berry) | Config | `.pnp.cjs` / `.pnp.loader.mjs` | Plug'n'Play dependency map. Both variants matched by `js-pnp`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/js/pnp-cjs](testdata/js/pnp-cjs), [testdata/js/pnp-loader](testdata/js/pnp-loader) |
| 28 | JavaScript (pnpm) | Lockfile | `pnpm-lock.yaml` | Extracts root importer deps by section plus top-level `packages` map (transitive); deduplicates by `name@version`. | 3 | `pnpm-lock` | [pnpm_lock.go](internal/analyze/pnpm_lock.go) | [pnpm_lock_test.go](internal/analyze/pnpm_lock_test.go), [pnpm_lock_scan_test.go](internal/analyze/pnpm_lock_scan_test.go) | [testdata/javascript/pnpm-lock-*](testdata/javascript/) |
| 29 | JavaScript (pnpm) | Workspace | `pnpm-workspace.yaml` | Monorepo workspace declaration. Both `.yaml` and `.yml` matched by `js-pnpm-workspace`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/js/pnpm-workspace](testdata/js/pnpm-workspace) |
| 30 | JavaScript (Bun) | Lockfile | `bun.lockb` | Binary lockfile. Filename matched only; no content parsed. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/sample-monorepo/frontend/bun.lockb](testdata/sample-monorepo/frontend/bun.lockb) |
| 31 | JavaScript (Bun) | Lockfile | `bun.lock` | Text lockfile (newer Bun). Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/sample-monorepo/frontend/bun.lock](testdata/sample-monorepo/frontend/bun.lock) |
| 32 | JavaScript | Vendored | `node_modules/` | Committed `node_modules`. Directory; no rule. | - | Not implemented | — | — | — |
| 33 | JavaScript | Config | `.npmrc` | Registry config. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/js/npmrc](testdata/js/npmrc) |
| 34 | JavaScript | Config | `.yarnrc.yml` | Yarn Berry config. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/js/yarnrc](testdata/js/yarnrc) |
| 35 | JavaScript (Bower) | Legacy manifest | `bower.json` | Checks for non-empty `dependencies` or `devDependencies`. | 2 | `json presence` | [json.go](internal/analyze/json.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/js/bower-*](testdata/js/) |
| 36 | JavaScript (Bower) | Legacy vendored | `bower_components/` | Bower installed deps directory. Not a file; no rule. | - | Not implemented | — | — | — |
| 37 | JavaScript (Import Maps) | Manifest | `importmap.json` | Standalone JSON import map file. Inline `<script type="importmap">` in HTML is handled by the html-external-scripts rule. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/js/importmap](testdata/js/importmap) |
| 38 | TypeScript (Deno) | Manifest | `deno.json` / `deno.jsonc` | Checks for `imports` key. | 2 | `json presence` | [json.go](internal/analyze/json.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/json/deno-json-*](testdata/json/) |
| 39 | TypeScript (Deno) | Lockfile | `deno.lock` | Filename matched only; no content parsed. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/sample-monorepo/frontend/deno.lock](testdata/sample-monorepo/frontend/deno.lock) |
| 40 | Java (Maven) | Manifest | `pom.xml` | Checks for `project.dependencies.dependency` element; ignores XML namespaces. | 2 | `xml presence` | [xml.go](internal/analyze/xml.go) | [xml_test.go](internal/analyze/xml_test.go) | [testdata/xml/pom-*](testdata/xml/) |
| 41 | Java (Maven) | Manifest (multi-module) | `*/pom.xml` | Same rule applied recursively — child `pom.xml` files in subdirectories are matched. | 2 | `xml presence` | [xml.go](internal/analyze/xml.go) | [xml_test.go](internal/analyze/xml_test.go) | [testdata/sample-monorepo/vendor/pom.xml](testdata/sample-monorepo/vendor/pom.xml) |
| 42 | Java (Gradle) | Manifest | `build.gradle` | Filename matched only; no content parsed. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | — |
| 43 | Java (Gradle) | Manifest | `build.gradle.kts` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | — |
| 44 | Java (Gradle) | Workspace | `settings.gradle` / `settings.gradle.kts` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | — |
| 45 | Java (Gradle) | Lockfile | `gradle.lockfile` / `*.lockfile` | Filename match for `gradle.lockfile` only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/sample-monorepo/java-service/gradle.lockfile](testdata/sample-monorepo/java-service/gradle.lockfile) |
| 46 | Java (Gradle) | Manifest | `gradle/libs.versions.toml` | Version catalog. No rule. | - | Not implemented | — | — | — |
| 47 | Java (Gradle) | Config | `gradle/wrapper/gradle-wrapper.properties` | Gradle distribution pin. No rule. | - | Not implemented | — | — | — |
| 48 | Java (Gradle) | Manifest | `buildSrc/build.gradle(.kts)` | Shared build logic. Covered by `build.gradle` / `build.gradle.kts` filename rules if the file is named exactly those; `buildSrc/` path itself is not targeted. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 49 | Java (Ant + Ivy) | Manifest | `ivy.xml` | Ivy dep declarations. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/java/ivy](testdata/java/ivy) |
| 50 | Java (Ant + Ivy) | Config | `ivysettings.xml` | Ivy resolver config. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/java/ivy-settings](testdata/java/ivy-settings) |
| 51 | Java (Ant) | Manifest | `build.xml` | Ant build file. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/java/ant-build](testdata/java/ant-build) |
| 52 | Java | Vendored | `lib/` or `libs/` | Committed JAR files. Directory; no rule. | - | Not implemented | — | — | — |
| 53 | Kotlin | Manifest | `build.gradle.kts` | Same as row 43; `java-gradle-kts` rule applies. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 54 | Kotlin (Multiplatform) | Manifest | `build.gradle.kts` | KMP sourceSets. Same rule as row 43; no parser for KMP-specific content. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 55 | Scala (sbt) | Manifest | `build.sbt` | sbt build definition. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/scala/sbt-build](testdata/scala/sbt-build) |
| 56 | Scala (sbt) | Manifest | `project/plugins.sbt` | No rule. | - | Not implemented | — | — | — |
| 57 | Scala (sbt) | Manifest | `project/Dependencies.scala` | No rule. | - | Not implemented | — | — | — |
| 58 | Scala (sbt) | Config | `project/build.properties` | No rule. | - | Not implemented | — | — | — |
| 59 | Scala (Mill) | Manifest | `build.sc` | Mill build definition. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/scala/mill-build](testdata/scala/mill-build) |
| 60 | Ruby (Bundler) | Manifest | `Gemfile` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/sample-monorepo/ruby-app/Gemfile](testdata/sample-monorepo/ruby-app/Gemfile) |
| 61 | Ruby (Bundler) | Lockfile | `Gemfile.lock` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/sample-monorepo/ruby-app/Gemfile.lock](testdata/sample-monorepo/ruby-app/Gemfile.lock) |
| 62 | Ruby | Manifest | `*.gemspec` | Filename matched only (`.*\.gemspec$`). | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/ruby/gemspec/demo.gemspec](testdata/ruby/gemspec/demo.gemspec) |
| 63 | Ruby | Manifest (variants) | `gemfiles/*.gemfile` | Appraisal pattern. No rule. | - | Not implemented | — | — | — |
| 64 | Ruby | Vendored | `vendor/bundle/` | Directory; no rule. | - | Not implemented | — | — | — |
| 65 | Ruby | Vendored | `vendor/cache/` | Directory; no rule. | - | Not implemented | — | — | — |
| 66 | PHP (Composer) | Manifest | `composer.json` | Checks for `require` or `require-dev` keys. | 2 | `json presence` | [json.go](internal/analyze/json.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/json/composer-json-*](testdata/json/) |
| 67 | PHP (Composer) | Lockfile | `composer.lock` | Extracts `packages[]` and `packages-dev[]` as `name@version` with section metadata. | 3 | `composer-lock` | [composer_lock.go](internal/analyze/composer_lock.go) | [composer_lock_test.go](internal/analyze/composer_lock_test.go), [composer_lock_scan_test.go](internal/analyze/composer_lock_scan_test.go) | [testdata/php/composer-lock-*](testdata/php/) |
| 68 | PHP | Vendored | `vendor/` | Directory; no rule. | - | Not implemented | — | — | — |
| 69 | Go | Manifest | `go.mod` | Extracts all `require` directives; marks indirect deps via `section: indirect`. | 3 | `go-mod` | [go_mod.go](internal/analyze/go_mod.go) | [go_mod_test.go](internal/analyze/go_mod_test.go) | [testdata/go/mod-*](testdata/go/) |
| 70 | Go | Lockfile-like | `go.sum` | Filename matched only; cryptographic checksums file. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/sample-monorepo/go-service/go.sum](testdata/sample-monorepo/go-service/go.sum) |
| 71 | Go | Workspace | `go.work` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 72 | Go (dep) | Legacy manifest | `Gopkg.toml` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 73 | Go (dep) | Legacy lockfile | `Gopkg.lock` | Checks for non-empty `projects` array. | 2 | `toml presence` | [toml.go](internal/analyze/toml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/go/gopkg-lock-*](testdata/go/) |
| 74 | Go (Glide) | Legacy manifest | `glide.yaml` | Checks for `import` or `testImport` keys. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/go/glide-yaml-*](testdata/go/) |
| 75 | Go (Glide) | Legacy lockfile | `glide.lock` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 76 | Go (godep) | Legacy manifest | `Godeps/Godeps.json` | No rule. | - | Not implemented | — | — | — |
| 77 | Go | Vendored | `vendor/` | Go vendor directory. Directory; no rule. | - | Not implemented | — | — | — |
| 78 | Rust (Cargo) | Manifest | `Cargo.toml` | Checks for non-empty `[dependencies]`, `[dev-dependencies]`, `[build-dependencies]`, `workspace.dependencies`, or per-target dep tables. | 2 | `toml presence` | [toml.go](internal/analyze/toml.go) | [scan_test.go](internal/analyze/scan_test.go), [toml_test.go](internal/analyze/toml_test.go) | [testdata/toml/cargo-*](testdata/toml/) |
| 79 | Rust (Cargo) | Lockfile | `Cargo.lock` | Extracts `[[package]]` entries as `name@version`; reports conclusive empty for version-marker-only files. | 3 | `cargo-lock` | [cargo_lock.go](internal/analyze/cargo_lock.go) | [cargo_lock_test.go](internal/analyze/cargo_lock_test.go), [cargo_lock_scan_test.go](internal/analyze/cargo_lock_scan_test.go) | [testdata/rust/cargo-lock-*](testdata/rust/) |
| 80 | Rust (Cargo) | Workspace | `Cargo.toml` (root) | `[workspace]` with `workspace.dependencies` checked by same `rust-cargo` rule as row 78. | 2 | `toml presence` | [toml.go](internal/analyze/toml.go) | [toml_test.go](internal/analyze/toml_test.go) | — |
| 81 | Rust | Vendored | `vendor/` | `cargo vendor` output. Directory; no rule. | - | Not implemented | — | — | — |
| 82 | Rust | Config | `.cargo/config.toml` | Source replacement / registry overrides. No rule. | - | Not implemented | — | — | — |
| 83 | C# / .NET | Manifest | `*.csproj` | Checks for `Project.ItemGroup.PackageReference` element. | 2 | `xml presence` | [xml.go](internal/analyze/xml.go) | [xml_test.go](internal/analyze/xml_test.go) | [testdata/dotnet/csproj-*](testdata/dotnet/) |
| 84 | C# / .NET | Manifest | `*.fsproj` | F# project file. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/dotnet/fsproj](testdata/dotnet/fsproj) |
| 85 | C# / .NET | Manifest | `*.vbproj` | VB.NET project file. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/dotnet/vbproj](testdata/dotnet/vbproj) |
| 86 | C# / .NET | Manifest | `Directory.Packages.props` | Checks for `Project.ItemGroup.PackageVersion` element. | 2 | `xml presence` | [xml.go](internal/analyze/xml.go) | [xml_test.go](internal/analyze/xml_test.go) | [testdata/dotnet/directory-packages-props-*](testdata/dotnet/) |
| 87 | C# / .NET | Config | `Directory.Build.props` | Shared MSBuild properties. Both `props` and `targets` variants matched by `dotnet-directory-build`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/dotnet/directory-build-props](testdata/dotnet/directory-build-props) |
| 88 | C# / .NET | Config | `Directory.Build.targets` | Shared MSBuild targets. Both `props` and `targets` variants matched by `dotnet-directory-build`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/dotnet/directory-build-targets](testdata/dotnet/directory-build-targets) |
| 89 | C# / .NET | Lockfile | `packages.lock.json` | Checks for non-empty `dependencies` key. | 2 | `json presence` | [json.go](internal/analyze/json.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/dotnet/packages-lock-*](testdata/dotnet/) |
| 90 | C# / .NET | Legacy manifest | `packages.config` | Checks for `packages.package` element. | 2 | `xml presence` | [xml.go](internal/analyze/xml.go) | [xml_test.go](internal/analyze/xml_test.go) | [testdata/dotnet/packages-config-*](testdata/dotnet/) |
| 91 | C# / .NET | Config | `.config/dotnet-tools.json` | .NET local tool manifest. No rule. | - | Not implemented | — | — | — |
| 92 | C# / .NET (Paket) | Manifest | `paket.dependencies` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 93 | C# / .NET (Paket) | Lockfile | `paket.lock` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 94 | C# / .NET (Paket) | Manifest | `paket.references` | Per-project dep list. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/dotnet/paket-references](testdata/dotnet/paket-references) |
| 95 | C / C++ (CMake) | Manifest | `CMakeLists.txt` | `find_package()` / `FetchContent_Declare()`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/cpp/cmake](testdata/cpp/cmake) |
| 96 | C / C++ (CMake) | Config | `cmake/*.cmake` | Find modules. No rule. | - | Not implemented | — | — | — |
| 97 | C / C++ (Conan) | Manifest | `conanfile.txt` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/cpp/conanfile/conanfile.txt](testdata/cpp/conanfile/conanfile.txt) |
| 98 | C / C++ (Conan) | Manifest | `conanfile.py` | Python script form of Conan manifest. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/cpp/conanfile-py](testdata/cpp/conanfile-py) |
| 99 | C / C++ (Conan) | Lockfile | `conan.lock` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/sample-monorepo/cpp-app/conan.lock](testdata/sample-monorepo/cpp-app/conan.lock) |
| 100 | C / C++ (vcpkg) | Manifest | `vcpkg.json` | Checks for non-empty `dependencies` array. | 2 | `json presence` | [json.go](internal/analyze/json.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/cpp/vcpkg-*](testdata/cpp/) |
| 101 | C / C++ (vcpkg) | Config | `vcpkg-configuration.json` | Registry config. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/cpp/vcpkg-configuration](testdata/cpp/vcpkg-configuration) |
| 102 | C / C++ (Meson) | Manifest | `meson.build` | Meson build definition. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/cpp/meson](testdata/cpp/meson) |
| 103 | C / C++ (Meson) | Manifest | `subprojects/*.wrap` | `.wrap` dep fetch descriptors. No rule. | - | Not implemented | — | — | — |
| 104 | C / C++ | Config | `configure.ac` / `configure.in` | Autotools dep checks. Both variants matched by `cpp-autotools`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/cpp/autotools](testdata/cpp/autotools) |
| 105 | C / C++ (Google) | Manifest | `DEPS` | gclient-style multi-repo dep fetch. No rule. | - | Not implemented | — | — | — |
| 106 | C / C++ | Vendored | `third_party/` | Full source trees of deps. Directory; no rule. | - | Not implemented | — | — | — |
| 107 | C / C++ | Vendored | `vendor/`, `extern/`, `external/`, `deps/`, `lib/` | Alternative vendoring dirs. Directory; no rule. | - | Not implemented | — | — | — |
| 108 | Swift (SPM) | Manifest | `Package.swift` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | — |
| 109 | Swift (SPM) | Lockfile | `Package.resolved` | Checks for non-empty `pins` array. | 2 | `json presence` | [json.go](internal/analyze/json.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/swift/package-resolved-*](testdata/swift/) |
| 110 | Swift (CocoaPods) | Manifest | `Podfile` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | — |
| 111 | Swift (CocoaPods) | Lockfile | `Podfile.lock` | Checks for non-empty `PODS` or `DEPENDENCIES` keys. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/ios/podfile-lock-*](testdata/ios/) |
| 112 | Swift (CocoaPods) | Manifest | `*.podspec` | CocoaPods podspec files. Matched by `ios-podspec`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/ios/podspec](testdata/ios/podspec) |
| 113 | Swift (CocoaPods) | Vendored | `Pods/` | Installed pod source. Directory; no rule. | - | Not implemented | — | — | — |
| 114 | Swift (Carthage) | Manifest | `Cartfile` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 115 | Swift (Carthage) | Lockfile | `Cartfile.resolved` | Carthage resolved lockfile. Matched by `ios-cartfile-resolved`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/ios/cartfile-resolved](testdata/ios/cartfile-resolved) |
| 116 | Swift (Carthage) | Vendored | `Carthage/Build/` | Pre-built frameworks. Directory; no rule. | - | Not implemented | — | — | — |
| 117 | Dart | Manifest | `pubspec.yaml` | Checks for non-empty `dependencies`, `dev_dependencies`, or `dependency_overrides`. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/yaml/pubspec-*](testdata/yaml/) |
| 118 | Dart | Lockfile | `pubspec.lock` | Checks for non-empty `packages` key. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/yaml/pubspec-lock-*](testdata/yaml/) |
| 119 | Flutter | Manifest | `pubspec.yaml` | Same as row 117; Flutter adds a `flutter:` section but the rule does not distinguish. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | — | — |
| 120 | Elixir (Mix) | Manifest | `mix.exs` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/elixir/mix/mix.exs](testdata/elixir/mix/mix.exs) |
| 121 | Elixir (Mix) | Lockfile | `mix.lock` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/sample-monorepo/elixir-app/mix.lock](testdata/sample-monorepo/elixir-app/mix.lock) |
| 122 | Elixir (Umbrella) | Workspace | `apps/*/mix.exs` | Same `mix.exs` filename rule applies recursively to all subdirectories. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 123 | Erlang (Rebar3) | Manifest | `rebar.config` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 124 | Erlang (Rebar3) | Lockfile | `rebar.lock` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 125 | Erlang (Erlang.mk) | Manifest | `Makefile` | `DEPS = ...` directives. No rule. | - | Not implemented | — | — | — |
| 126 | Haskell (Cabal) | Manifest | `*.cabal` | Filename matched only (`.*\.cabal$`). | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/haskell/cabal/demo.cabal](testdata/haskell/cabal/demo.cabal) |
| 127 | Haskell (Cabal) | Config | `cabal.project` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 128 | Haskell (Cabal) | Lockfile | `cabal.project.freeze` | Cabal freeze file. Matched by `haskell-cabal-project-freeze`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/haskell/cabal-project-freeze](testdata/haskell/cabal-project-freeze) |
| 129 | Haskell (Stack) | Manifest | `package.yaml` | Checks for non-empty `dependencies` key. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/haskell/package-yaml-*](testdata/haskell/) |
| 130 | Haskell (Stack) | Config | `stack.yaml` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 131 | Haskell (Stack) | Lockfile | `stack.yaml.lock` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 132 | Clojure (Leiningen) | Manifest | `project.clj` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 133 | Clojure (tools.deps) | Manifest | `deps.edn` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 134 | Clojure (Boot) | Legacy manifest | `build.boot` | Boot build file. Matched by `clojure-boot`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/clojure/boot](testdata/clojure/boot) |
| 135 | Lua (LuaRocks) | Manifest | `*.rockspec` | LuaRocks package spec. Matched by `lua-rockspec`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/lua/rockspec](testdata/lua/rockspec) |
| 136 | R | Manifest | `DESCRIPTION` | `Imports:`, `Depends:` fields. No rule. | - | Not implemented | — | — | — |
| 137 | R (renv) | Lockfile | `renv.lock` | renv lockfile. Matched by `r-renv-lock`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/r/renv-lock](testdata/r/renv-lock) |
| 138 | R (packrat) | Legacy lockfile | `packrat/packrat.lock` | No rule. | - | Not implemented | — | — | — |
| 139 | R (packrat) | Vendored | `packrat/src/` | Directory; no rule. | - | Not implemented | — | — | — |
| 140 | Julia | Manifest | `Project.toml` | Checks for non-empty `[deps]` table. | 2 | `toml presence` | [toml.go](internal/analyze/toml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/julia/project-*](testdata/julia/) |
| 141 | Julia | Lockfile | `Manifest.toml` | Checks for non-empty entries under `deps` tables. | 2 | `toml presence` | [toml.go](internal/analyze/toml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/julia/manifest-*](testdata/julia/) |
| 142 | Perl | Manifest | `cpanfile` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/perl/cpanfile/cpanfile](testdata/perl/cpanfile/cpanfile) |
| 143 | Perl | Lockfile | `cpanfile.snapshot` | Carton lockfile. Matched by `perl-cpanfile-snapshot`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/perl/cpanfile-snapshot](testdata/perl/cpanfile-snapshot) |
| 144 | Perl | Manifest | `Makefile.PL` | ExtUtils::MakeMaker. Matched by `perl-makefile-pl`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/perl/makefile-pl](testdata/perl/makefile-pl) |
| 145 | Perl | Manifest | `Build.PL` | Module::Build. Matched by `perl-build-pl`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/perl/build-pl](testdata/perl/build-pl) |
| 146 | Perl | Manifest | `META.json` / `META.yml` | CPAN metadata. Matched by `perl-meta` (`META.json`, `META.yml`, `META.yaml`). | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/perl/meta-json](testdata/perl/meta-json) |
| 147 | Perl | Config | `dist.ini` | Dist::Zilla. Matched by `perl-dist-ini`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/perl/dist-ini](testdata/perl/dist-ini) |
| 148 | Raku (Perl 6) | Manifest | `META6.json` | Raku module metadata. Matched by `raku-meta`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/raku/meta](testdata/raku/meta) |
| 149 | Zig | Manifest | `build.zig.zon` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/zig/build-zon/build.zig.zon](testdata/zig/build-zon/build.zig.zon) |
| 150 | Zig | Config | `build.zig` | Build script referencing packages. Matched by `zig-build`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/zig/build](testdata/zig/build) |
| 151 | Nim | Manifest | `*.nimble` | Filename matched only (`.*\.nimble$`). | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/nim/nimble/demo.nimble](testdata/nim/nimble/demo.nimble) |
| 152 | OCaml (opam) | Manifest | `*.opam` | Filename matched only (`.*\.opam$`). | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/ocaml/opam/demo.opam](testdata/ocaml/opam/demo.opam) |
| 153 | OCaml (opam) | Lockfile | `*.opam.locked` | opam locked manifest. Matched by `ocaml-opam-locked`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/ocaml/opam-locked](testdata/ocaml/opam-locked) |
| 154 | OCaml (Dune) | Config | `dune-project` | Dune project file. Matched by `ocaml-dune-project`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/ocaml/dune-project](testdata/ocaml/dune-project) |
| 155 | OCaml (Dune) | Config | `dune` | Per-directory build rules. No rule. | - | Not implemented | — | — | — |
| 156 | OCaml (esy) | Manifest | `esy.json` or `package.json` | npm-style OCaml management. `esy.json` matched by `ocaml-esy`; `package.json` covered by JS rule (row 21). | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/ocaml/esy](testdata/ocaml/esy) |
| 157 | OCaml (esy) | Lockfile | `esy.lock/` | Directory-based lockfile. No rule. | - | Not implemented | — | — | — |
| 158 | V (Vlang) | Manifest | `v.mod` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/vlang/project/v.mod](testdata/vlang/project/v.mod) |
| 159 | Crystal | Manifest | `shard.yml` | Checks for non-empty top-level `dependencies` key. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/crystal/shard-*](testdata/crystal/) |
| 160 | Crystal | Lockfile | `shard.lock` | Crystal shard lockfile. Matched by `crystal-shard-lock`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/crystal/shard-lock](testdata/crystal/shard-lock) |
| 161 | Crystal | Vendored | `lib/` | Installed shards. Directory; no rule. | - | Not implemented | — | — | — |
| 162 | Gleam | Manifest | `gleam.toml` | Checks for non-empty `[dependencies]` table. | 2 | `toml presence` | [toml.go](internal/analyze/toml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/gleam/project-*](testdata/gleam/) |
| 163 | Gleam | Lockfile | `manifest.toml` | Gleam lockfile (lowercase). `julia-manifest` matches `^Manifest\.toml$` (capital M) only. Matched by `gleam-manifest`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/gleam/manifest](testdata/gleam/manifest) |
| 164 | Fortran (fpm) | Manifest | `fpm.toml` | Fortran Package Manager manifest. Matched by `fortran-fpm`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/fortran/fpm](testdata/fortran/fpm) |
| 165 | Nix | Manifest | `default.nix` / `shell.nix` | Matched by `nix-default-shell`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/nix/default-shell](testdata/nix/default-shell) |
| 166 | Nix (Flakes) | Manifest | `flake.nix` | Matched by `nix-flake`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/nix/flake](testdata/nix/flake) |
| 167 | Nix (Flakes) | Lockfile | `flake.lock` | Matched by `nix-flake-lock`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/nix/flake-lock](testdata/nix/flake-lock) |
| 168 | Terraform | Manifest | `*.tf` | Limited: the `terraform.aws_glue_job.python` rule detects `aws_glue_job` resources with `--additional-python-modules`. Does NOT detect general `required_providers { }` blocks. | 2 | `terraform resource check` | [terraform.go](internal/analyze/terraform.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/terraform/](testdata/terraform/) |
| 169 | Terraform | Lockfile | `.terraform.lock.hcl` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/terraform/lock/.terraform.lock.hcl](testdata/terraform/lock/.terraform.lock.hcl) |
| 170 | Ansible | Manifest | `requirements.yml` | Checks for non-empty `roles` or `collections` keys. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/ansible/requirements-*](testdata/ansible/) |
| 171 | Ansible | Manifest | `collections/requirements.yml` | The `ansible-requirements` rule uses `^requirements\.ya?ml$` and applies recursively, so `collections/requirements.yml` is matched. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | — | — |
| 172 | Helm | Manifest | `Chart.yaml` | Checks for non-empty top-level `dependencies` key. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/helm/chart-*](testdata/helm/) |
| 173 | Helm | Lockfile | `Chart.lock` | Matched by `helm-chart-lock`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/helm/chart-lock](testdata/helm/chart-lock) |
| 174 | Helm | Vendored | `charts/` | Vendored chart tarballs. Directory; no rule. | - | Not implemented | — | — | — |
| 175 | Docker | Manifest-like | `Dockerfile` | `FROM image:tag`. Covers `Dockerfile` and suffixed variants (`Dockerfile.dev`, `Dockerfile.prod`, etc.). | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/docker/dockerfile](testdata/docker/dockerfile) |
| 176 | Docker | Config | `docker-compose.yml` / `compose.yaml` | `image:` references. Covers `docker-compose.yml`, `docker-compose.yaml`, `compose.yml`, `compose.yaml`. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/docker/compose-v1](testdata/docker/compose-v1), [testdata/docker/compose-v2](testdata/docker/compose-v2) |
| 177 | GitHub Actions | Manifest-like | `.github/workflows/*.yml` | `uses: org/action@version`. No default rule. The `yaml` extraction parser supports custom rules with path expressions, but no built-in rule targets workflow files. | - | Not implemented (default rules) | — | — | [testdata/yaml-workflow/workflow.yaml](testdata/yaml-workflow/workflow.yaml) (custom-rule example only) |
| 178 | GitHub Actions | Manifest | `action.yml` / `action.yaml` | Composite action definition. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/github-actions/action-yml](testdata/github-actions/action-yml), [testdata/github-actions/action-yaml](testdata/github-actions/action-yaml) |
| 179 | Unity | Manifest | `Packages/manifest.json` | Path-glob `Packages/manifest.json`; checks for non-empty `dependencies` key. | 2 | `json presence` (path-glob) | [json.go](internal/analyze/json.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/unity/packages-manifest-*](testdata/unity/) |
| 180 | Unity | Lockfile | `Packages/packages-lock.json` | No rule. | - | Not implemented | — | — | — |
| 181 | Unity | Vendored | `Assets/Plugins/` | DLL directory. No rule. | - | Not implemented | — | — | — |
| 182 | Unreal Engine | Manifest | `*.uproject` | Matched by `unreal-uproject`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/unreal/uproject](testdata/unreal/uproject) |
| 183 | Unreal Engine | Manifest | `*.uplugin` | Matched by `unreal-uplugin`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/unreal/uplugin](testdata/unreal/uplugin) |
| 184 | Unreal Engine | Vendored | `Plugins/` | Directory; no rule. | - | Not implemented | — | — | — |
| 185 | Godot | Vendored | `addons/` | Directory; no rule. | - | Not implemented | — | — | — |
| 186 | Godot | Config | `plugin.cfg` | Addon descriptor. Matched by `godot-plugin-cfg`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/godot/plugin-cfg](testdata/godot/plugin-cfg) |
| 187 | React Native | Manifest | `package.json` | Covered by the standard `js` rule (row 21). | 2 | `json presence` | [json.go](internal/analyze/json.go) | — | — |
| 188 | React Native | Manifest | `ios/Podfile` | Covered by the `ios-podfile` rule (row 110). | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 189 | React Native | Lockfile | `ios/Podfile.lock` | Covered by the `ios-podfile-lock` rule (row 111). | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | — | — |
| 190 | React Native | Manifest | `android/build.gradle` | Covered by `java-gradle` rule (row 42). | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 191 | React Native | Manifest | `android/app/build.gradle` | Covered by `java-gradle` rule (row 42). | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 192 | Android | Manifest | `build.gradle(.kts)` (root + app) | Covered by `java-gradle` / `java-gradle-kts` rules (rows 42–43). | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | — | — |
| 193 | Android | Manifest | `gradle/libs.versions.toml` | Version catalog. No rule (same gap as row 46). | - | Not implemented | — | — | — |
| 194 | Solidity (Foundry) | Config | `foundry.toml` | Matched by `foundry-toml`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/foundry/toml](testdata/foundry/toml) |
| 195 | Solidity (Foundry) | Vendored | `lib/` | Git submodule deps. Directory; no rule. | - | Not implemented | — | — | — |
| 196 | Solidity (Foundry) | Config | `remappings.txt` | Import path remappings. Matched by `foundry-remappings`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/foundry/remappings](testdata/foundry/remappings) |
| 197 | Solidity (Hardhat) | Manifest | `package.json` | npm-based; covered by the standard `js` rule (row 21). | 2 | `json presence` | [json.go](internal/analyze/json.go) | — | — |
| 198 | Solidity (Foundry/Soldeer) | Lockfile | `soldeer.lock` | Matched by `soldeer-lock`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/soldeer/lock](testdata/soldeer/lock) |
| 199 | Bazel | Manifest (legacy) | `WORKSPACE` / `WORKSPACE.bazel` | `http_archive`, `git_repository`, etc. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/bazel/workspace](testdata/bazel/workspace), [testdata/bazel/workspace-bazel](testdata/bazel/workspace-bazel) |
| 200 | Bazel (Bzlmod) | Manifest | `MODULE.bazel` | `bazel_dep()` declarations. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/bazel/module](testdata/bazel/module) |
| 201 | Bazel (Bzlmod) | Lockfile | `MODULE.bazel.lock` | Bzlmod lockfile. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/bazel/module-lock](testdata/bazel/module-lock) |
| 202 | Bazel | Config | `BUILD` / `BUILD.bazel` | Per-package. `BUILD.bazel` detected; bare `BUILD` deferred (ambiguous). | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/bazel/build-bazel](testdata/bazel/build-bazel) |
| 203 | Bazel | Config | `*.bzl` in `third_party/` | Starlark dep fetch rules. No rule. | - | Not implemented | — | — | — |
| 204 | Nx | Config | `nx.json` | Workspace config. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/nx/config](testdata/nx/config) |
| 205 | Nx | Config | `project.json` | Per-project config. No rule. | - | Not implemented | — | — | — |
| 206 | Lerna | Config | `lerna.json` | Monorepo config. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/lerna/config](testdata/lerna/config) |
| 207 | Rush | Config | `rush.json` | Monorepo config. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/rush/config](testdata/rush/config) |
| 208 | Rush | Lockfile | `common/config/rush/pnpm-lock.yaml` | No path-glob rule targeting this location. The `pnpm-lock.yaml` filename rule would match it, but only at the root. The scanner recurses, so if the file is found it would match. | 3 | `pnpm-lock` (if found) | [pnpm_lock.go](internal/analyze/pnpm_lock.go) | — | — |
| 209 | Rush | Config | `common/config/rush/common-versions.json` | No rule. | - | Not implemented | — | — | — |
| 210 | Turborepo | Config | `turbo.json` | Monorepo pipeline config. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/turbo/config](testdata/turbo/config) |
| 211 | Pants | Config | `pants.toml` | Build system config. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/pants/config](testdata/pants/config) |
| 212 | Pants | Manifest | `BUILD` files | Per-directory targets. No rule. | - | Not implemented | — | — | — |
| 213 | Pants | Manifest | `3rdparty/python/requirements.txt` | Covered by the `python-requirements` rule (row 1) since the filename matches the regex. | 3 | `py-requirements` | [py_requirements.go](internal/analyze/py_requirements.go) | — | — |
| 214 | Pants | Manifest | `3rdparty/jvm/BUILD` | No rule. | - | Not implemented | — | — | — |
| 215 | Git Submodules | Manifest | `.gitmodules` | Submodule declarations. | 1 | filename-regex | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/git/submodules](testdata/git/submodules) |
| 216 | Homebrew | Manifest | `Brewfile` | Filename matched only. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/homebrew/brewfile/Brewfile](testdata/homebrew/brewfile/Brewfile) |
| 217 | Homebrew | Lockfile | `Brewfile.lock.json` | Matched by `homebrew-brewfile-lock`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/homebrew/brewfile-lock](testdata/homebrew/brewfile-lock) |
| 218 | Protocol Buffers (Buf) | Manifest | `buf.yaml` | Checks for non-empty `deps` key. | 2 | `yaml presence` | [yaml.go](internal/analyze/yaml.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/buf/module-*](testdata/buf/) |
| 219 | Protocol Buffers (Buf) | Lockfile | `buf.lock` | Matched by `buf-lock`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/buf/lock](testdata/buf/lock) |
| 220 | Puppet | Manifest | `Puppetfile` | Matched by `puppet-puppetfile`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/puppet/puppetfile](testdata/puppet/puppetfile) |
| 221 | Puppet | Manifest | `metadata.json` | No rule. | - | Not implemented | — | — | — |
| 222 | Chef (Berkshelf) | Manifest | `Berksfile` | Matched by `chef-berksfile`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/chef/berksfile](testdata/chef/berksfile) |
| 223 | Chef (Berkshelf) | Lockfile | `Berksfile.lock` | Matched by `chef-berksfile-lock`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/chef/berksfile-lock](testdata/chef/berksfile-lock) |
| 224 | Chef | Manifest | `metadata.rb` | `.rb` extension limits false positives. Matched by `chef-metadata`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/chef/metadata](testdata/chef/metadata) |
| 225 | Chef (Policyfile) | Manifest | `Policyfile.rb` | Matched by `chef-policyfile`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/chef/policyfile](testdata/chef/policyfile) |
| 226 | Chef (Policyfile) | Lockfile | `Policyfile.lock.json` | Matched by `chef-policyfile-lock`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/chef/policyfile-lock](testdata/chef/policyfile-lock) |
| 227 | Jsonnet | Manifest | `jsonnetfile.json` | Checks for non-empty `dependencies` array. | 2 | `json presence` | [json.go](internal/analyze/json.go) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/jsonnet/bundler-*](testdata/jsonnet/) |
| 228 | Jsonnet | Lockfile | `jsonnetfile.lock.json` | Matched by `jsonnet-lock`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/jsonnet/lock](testdata/jsonnet/lock) |
| 229 | Emacs (Cask) | Manifest | `Cask` | Matched by `emacs-cask`. | 1 | filename match | [default_rules.yaml](internal/analyze/default_rules.yaml) | [scan_test.go](internal/analyze/scan_test.go) | [testdata/emacs/cask](testdata/emacs/cask) |

---

## Summary

### Coverage by level

| Level | Count | Notes |
|-------|-------|-------|
| 3 — extracts deps | 22 | Full dep extraction with name, version, section metadata |
| 2 — presence check | 32 | Confirms dep declarations exist; does not extract names/versions |
| 1 — detected only | 53 | File identified by filename; no content analysis |
| - — not implemented | 122 | File type not recognized at all |

**Total catalogued:** 229 rows (some rows share the same rule; e.g. rows 7/8/12/14/15 all hit `pyproject.toml` via the same TOML rule).

### Level 3 extractors

| Extractor | Files handled |
|-----------|--------------|
| `py-requirements` | `requirements.txt`, `requirements.in`, `requirements/*.txt` |
| `uv-lock` | `uv.lock` |
| `poetry-lock` | `poetry.lock` |
| `pipfile-lock` | `Pipfile.lock` |
| `toml queries` | `pyproject.toml`, `Pipfile` |
| `python call` | `setup.py` |
| `ini queries` | `setup.cfg` |
| `package-lock` | `package-lock.json`, `npm-shrinkwrap.json` |
| `yarn-lock` | `yarn.lock` (v1 and Berry) |
| `pnpm-lock` | `pnpm-lock.yaml` |
| `composer-lock` | `composer.lock` |
| `cargo-lock` | `Cargo.lock` |
| `go-mod` | `go.mod` |
| `banner-regex` | `*.js` (banner comment detection) |
| `html external-scripts` | `*.html`, `*.htm`, `*.xhtml`, `*.tmpl`, `*.gohtml`, `*.mustache`, `*.hbs`, `*.njk` |
| `typescript cdk` | `*.ts` (CDK `CfnJob` construct) |
| `python cdk` | `*.py` (CDK `CfnJob` construct) |

### Notable gaps at level 2 or 1 (high-value targets for extraction)

| Row | File | Current level | Gap |
|-----|------|--------------|-----|
| 21 | `package.json` | 2 | Dep names are present in JSON; extraction is straightforward |
| 40 | `pom.xml` | 2 | Maven XML structure well-defined |
| 66 | `composer.json` | 2 | Lockfile (row 67) is level 3; manifest extraction would complement it |
| 73 | `Gopkg.lock` | 2 | TOML format; could use `toml queries` |
| 78 | `Cargo.toml` | 2 | TOML format; dep table keys are dep names |
| 83 | `*.csproj` | 2 | XML; `PackageReference Include` attribute contains the name |
| 109 | `Package.resolved` | 2 | JSON; `pins[].identity` or `pins[].package` contains name |
| 117 | `pubspec.yaml` | 2 | YAML; dep keys are package names |
| 140 | `Project.toml` | 2 | Julia TOML; `[deps]` keys are package names |
| 162 | `gleam.toml` | 2 | TOML; `[dependencies]` keys are package names |

### Files not implemented that appear in most real-world codebases

`gradle/libs.versions.toml` (rows 46, 193), `Cargo.toml` extraction (row 78), `build.gradle(.kts)` extraction (rows 42–43), `Gemfile.lock` parsing (row 61), `Gemfile` parsing (row 60), `*.csproj` extraction (row 83), `flake.lock` (row 167), `Dockerfile` (row 175), `.github/workflows/*.yml` (row 177).
