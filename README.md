# deplens

`deplens` is a small CLI for scanning a directory tree and reporting dependency-related manifests it finds. It is aimed at mixed-language repositories and can detect both standard manifest files and rule-based sources such as Terraform resources or HTML pages that load external scripts.

By default, the tool walks the target directory recursively, skips common generated/vendor directories, and prints a grouped summary. It can also emit JSON for machine-readable consumption and load additional detectors from a custom YAML rules file.

## Supported Detectors

Built-in detectors:

| Detector | Matches | Extracts dependencies |
| --- | --- | --- |
| filename regex match | Built-in filename rules: `*requirements*.txt`, `uv.lock`, `package.json`, `yarn.lock`, `pom.xml` | No |
| banner regex | JavaScript files whose first 4096 bytes match a configured `banner-regex` with capture groups 1 and 2 for package name and version | Yes |
| yaml | Path expression such as `workflow.steps[].config.packages.pip[]` to extract data from yaml files | Yes |
| html external scripts | HTML-like files (`.html`, `.htm`, `.xhtml`, `.tmpl`, `.gohtml`, `.mustache`, `.hbs`, `.njk`) containing external `<script src="https://...">` tags | Yes |
| html module scripts | HTML-like files (`.html`, `.htm`, `.xhtml`, `.tmpl`, `.gohtml`, `.mustache`, `.hbs`, `.njk`) containing `<script type="module">` blocks with `import "https://..."` module imports | Yes |
| html import maps | HTML-like files (`.html`, `.htm`, `.xhtml`, `.tmpl`, `.gohtml`, `.mustache`, `.hbs`, `.njk`) containing `<script type="importmap">` blocks with remote URLs in the `imports` map | Yes |
| terraform | Terraform `.tf` files with parsing content. For example containing a `aws_glue_job` resource with `default_arguments.--job-language = "python"` and `default_arguments.--additional-python-modules` present | No |

Default JavaScript banner rules use `filename-regex: '.*\.js$'` and return `name@version` from `banner-regex` capture groups 1 and 2. The built-in banner rule set includes `js-banner-block-start`, `js-banner-plain-block-start`, `js-banner-multiline-preserved`, `js-banner-line-comment`, and `js-banner-version-tagged`.

## Example

```bash
go run ./cmd/deplens ./testdata/sample-monorepo
```

Example output:

```text
Root: /path/to/project

python-requirements
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
