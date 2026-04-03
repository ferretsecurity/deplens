# deplens

`deplens` is a small CLI for scanning a directory tree and reporting dependency-related manifests it finds. It is aimed at mixed-language repositories and can detect both standard manifest files and rule-based sources such as Terraform resources or HTML pages that load external scripts.

By default, the tool walks the target directory recursively, skips common generated/vendor directories, and prints a grouped summary. The default detector rules are embedded into the binary at build time. The tool can also emit JSON for machine-readable consumption and load additional detectors from a custom YAML rules file passed with `--rules`.

## Supported Detectors

Rules may use `filename-regex`, `path-glob`, or both. When both selectors are present on the same rule, they are combined with AND semantics, so the file must match both conditions before the detector runs.

Built-in detectors:

| Detector | Matches | Extracts dependencies |
| --- | --- | --- |
| filename regex match | Built-in filename rules: `*requirements*.txt`, `*requirements*.in`, `uv.lock`, `package.json`, `yarn.lock`, `pom.xml` | No |
| toml | TOML files matched by a rule such as built-in `python-pyproject` for `pyproject.toml`; extracts from `project.dependencies[]`, `project.optional-dependencies.*[]`, `dependency-groups.*[]`, `tool.poetry.dependencies`, and `tool.poetry.group.*.dependencies` | Yes |
| python call | Python files matched by a rule such as built-in `python-setup-py` for `setup.py`; detects imported function calls with specific keyword arguments, for example `setuptools.setup(..., install_requires=..., extras_require=...)` | No |
| banner regex | JavaScript files whose first 4096 bytes match a configured `banner-regex` with capture groups 1 and 2 for package name and version | Yes |
| yaml | Path expression such as `workflow.steps[].config.packages.pip[]` to extract data from yaml files | Yes |
| html external scripts | HTML-like files (`.html`, `.htm`, `.xhtml`, `.tmpl`, `.gohtml`, `.mustache`, `.hbs`, `.njk`) containing external `<script src="https://...">` tags | Yes |
| html module scripts | HTML-like files (`.html`, `.htm`, `.xhtml`, `.tmpl`, `.gohtml`, `.mustache`, `.hbs`, `.njk`) containing `<script type="module">` blocks with `import "https://..."` module imports | Yes |
| html import maps | HTML-like files (`.html`, `.htm`, `.xhtml`, `.tmpl`, `.gohtml`, `.mustache`, `.hbs`, `.njk`) containing `<script type="importmap">` blocks with remote URLs in the `imports` map | Yes |
| terraform | Terraform `.tf` files with parsing content. For example containing a `aws_glue_job` resource with `default_arguments.--job-language = "python"` and `default_arguments.--additional-python-modules` present | No |
| typescript cdk construct | TypeScript `.ts` files parsed with an AST. For example containing `new glue.CfnJob(..., { defaultArguments: { "--job-language": "python", "--additional-python-modules": "pandas==2.2.1" }})` imported from `aws-cdk-lib/aws-glue` | Yes |
| python cdk construct | Python `.py` files with statically-resolved CDK `CfnJob(...)` calls. For example containing `glue.CfnJob(..., default_arguments={"--job-language": "python", "--additional-python-modules": "pandas==2.2.1"})` imported from `aws_cdk.aws_glue` | Yes |

Default JavaScript banner rules use `filename-regex: '.*\.js$'` and return `name@version` from `banner-regex` capture groups 1 and 2. The built-in banner rule set includes `js-banner-block-start`, `js-banner-plain-block-start`, `js-banner-multiline-preserved`, `js-banner-line-comment`, and `js-banner-version-tagged`.

The default Python requirements rules include both a filename selector for `*requirements*.txt` and `*requirements*.in`, plus a path selector for `**/requirements/*.txt`.

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

js
- frontend/package.json

js-yarn
- frontend/yarn.lock

java
- java-service/pom.xml
```

When `pyproject.toml` is present, it is reported as `python-pyproject`, for example:

```text
python-pyproject
- pyproject.toml
```

When `setup.py` contains a `setuptools.setup(...)` call with `install_requires` or `extras_require`, it is reported as `python-setup-py`, for example:

```text
python-setup-py
- setup.py
```
