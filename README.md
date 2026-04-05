# deplens

`deplens` is a small CLI for scanning a directory tree and reporting dependency-related manifests it finds. It is aimed at mixed-language repositories and can detect both standard manifest files and rule-based sources such as Terraform resources or HTML pages that load external scripts.

By default, the tool walks the target directory recursively, skips common generated/vendor directories, and prints a grouped summary. The default detector rules are embedded into the binary at build time. The tool can also emit JSON for machine-readable consumption and load additional detectors from a custom YAML rules file passed with `--rules`.

JSON output contains a top-level `root` plus `manifests`. Each manifest entry includes `type`, `path`, optional `dependencies`, and `has_dependencies`. Each dependency is emitted as an object with `name` and optional `section`. `has_dependencies` is `null` when the detector cannot determine dependency presence, `true` when extraction confirmed at least one dependency, and `false` when a detector or extractor conclusively matched but found none.

## Supported Detectors

Rules may use `filename-regex`, `path-glob`, or both. When both selectors are present on the same rule, they are combined with AND semantics, so the file must match both conditions before the detector runs.

Built-in detectors:

| Detector | Matches | Extracts dependencies |
| --- | --- | --- |
| filename regex match | Built-in filename rules: `*requirements*.txt`, `*requirements*.in`, `uv.lock`, `poetry.lock`, `Pipfile.lock`, `pdm.lock`, `conda-lock.yml`, `package.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock`, `bun.lockb`, `deno.lock`, `pom.xml`, `gradle.lockfile`, `Gemfile.lock`, `go.sum`, `Gopkg.lock`, `glide.lock`, `conan.lock`, `Package.resolved`, `Podfile.lock`, `mix.lock` | No |
| path glob match | Built-in path-glob rules such as `python-requirements-dir` for `**/requirements/*.txt` | No |
| toml | TOML files matched by a rule such as built-in `python-pyproject` for `pyproject.toml`; extracts from `build-system.requires[]`, `project.dependencies[]`, `project.optional-dependencies.*[]`, `dependency-groups.*[]`, `tool.poetry.dependencies`, and `tool.poetry.group.*.dependencies` | Yes |
| pipfile | `Pipfile` matched by the built-in `python-pipfile` rule; reports only when the file contains at least one dependency-bearing package section such as `[packages]`, `[dev-packages]`, or a custom package category like `[docs]` | Yes |
| python call | Python files matched by a rule such as built-in `python-setup-py` for `setup.py`; detects imported function calls with specific keyword arguments, for example `setuptools.setup(..., install_requires=..., extras_require=...)`, and can extract from simple literal arrays in `install_requires=[...]` plus `extras_require={"group": [...]}` | Yes |
| ini | INI files matched by a rule such as built-in `python-setup-cfg` for `setup.cfg`; extracts from `[options]` keys `setup_requires` and `install_requires`, plus all keys under `[options.extras_require]`, when values are written as static multiline lists | Yes |
| banner regex | JavaScript files whose first 4096 bytes match a configured `banner-regex` with capture groups 1 and 2 for package name and version | Yes |
| yaml | Path expression such as `workflow.steps[].config.packages.pip[]` to extract data from yaml files, or a presence check such as `dependencies` to detect files that declare a dependency section without extracting it | Sometimes |
| html external scripts | HTML-like files (`.html`, `.htm`, `.xhtml`, `.tmpl`, `.gohtml`, `.mustache`, `.hbs`, `.njk`) matched by the built-in `html-external-scripts` rule; extracts remote URLs from external `<script src="https://...">` tags, `<script type="module">` imports, and `<script type="importmap">` `imports` entries | Yes |
| terraform | Terraform `.tf` files with parsing content. For example containing a `aws_glue_job` resource with `default_arguments.--job-language = "python"` and `default_arguments.--additional-python-modules` present | No |
| typescript cdk construct | TypeScript `.ts` files parsed with an AST. For example containing `new glue.CfnJob(..., { defaultArguments: { "--job-language": "python", "--additional-python-modules": "pandas==2.2.1" }})` imported from `aws-cdk-lib/aws-glue` | Yes |
| python cdk construct | Python `.py` files with statically-resolved CDK `CfnJob(...)` calls. For example containing `glue.CfnJob(..., default_arguments={"--job-language": "python", "--additional-python-modules": "pandas==2.2.1"})` imported from `aws_cdk.aws_glue` | Yes |

Default JavaScript banner rules use `filename-regex: '.*\.js$'` and return `name@version` from `banner-regex` capture groups 1 and 2. The built-in banner rule set includes `js-banner-block-start`, `js-banner-plain-block-start`, `js-banner-multiline-preserved`, `js-banner-line-comment`, and `js-banner-version-tagged`.

The default Python requirements rules include both a filename selector for `*requirements*.txt` and `*requirements*.in`, plus a path selector for `**/requirements/*.txt`.

The default rules also include `python-conda-environment` for `environment.yml` and `environment.yaml`, which reports the file only when a top-level `dependencies` key is present.

When `Pipfile` is present, it is reported as `python-pipfile` only if at least one dependency-bearing package section exists, for example `[packages]`, `[dev-packages]`, or a custom package-category section. Extracted dependencies are emitted from those sections, while metadata sections such as `[[source]]` and `[requires]` are ignored.

## Example

```bash
go run ./cmd/deplens ./testdata/sample-monorepo
```

Example output:

```text
Root: /path/to/project

python-requirements
- requirements.qt6_3.in
- requirements.txt

python-uv
- backend/uv.lock

python-poetry-lock
- backend/poetry.lock

python-pipfile-lock
- backend/Pipfile.lock

python-pdm-lock
- backend/pdm.lock

python-conda-lock
- backend/conda-lock.yml

js
- frontend/package.json

js-yarn
- frontend/yarn.lock

js-pnpm-lock
- frontend/pnpm-lock.yaml

js-bun-lock
- frontend/bun.lock

js-bun-lockb
- frontend/bun.lockb

deno-lock
- frontend/deno.lock

java
- java-service/pom.xml

java-gradle-lockfile
- java-service/gradle.lockfile

ruby-gemfile-lock
- ruby-app/Gemfile.lock

go-sum
- go-service/go.sum

go-gopkg-lock
- go-service/Gopkg.lock

go-glide-lock
- go-service/glide.lock

cpp-conan-lock
- cpp-app/conan.lock

swift-package-resolved
- ios-app/Package.resolved

ios-podfile-lock
- ios-app/Podfile.lock

elixir-mix-lock
- elixir-app/mix.lock
```

When `pyproject.toml` is present, it is reported as `python-pyproject`, for example:

```text
python-pyproject
- pyproject.toml
```

When a Conda environment file contains a top-level `dependencies` key, it is reported as `python-conda-environment`, for example:

```text
python-conda-environment
- environment.yml
```

When `setup.py` contains a `setuptools.setup(...)` call with `install_requires` or `extras_require`, it is reported as `python-setup-py`. For simple literal forms such as `install_requires=[...]` and `extras_require={"dev": [...]}`, dependencies are extracted as well, for example:

```text
python-setup-py
- setup.py
  - requests>=2.31
  - pytest>=8
```

When `setup.cfg` contains declarative setuptools dependency keys such as `[options] install_requires`, `[options] setup_requires`, or entries under `[options.extras_require]`, it is reported as `python-setup-cfg`. For static multiline values, dependencies are extracted as well, for example:

```text
python-setup-cfg
- setup.cfg
  - requests>=2.31
  - pytest>=8
```
