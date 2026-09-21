# deplens

`deplens` is a command-line tool that finds dependency sources in a directory tree. It recognizes manifests, lockfiles, build definitions, CI workflows, deployment files, tool configuration, vendored files, and other places where a project declares or uses dependencies.

It also runs policy checks, such as finding a missing lockfile, conflicting JavaScript lockfiles, or a dependency source without a CODEOWNER.

The scan is local and deterministic, self-contained, `deplens` makes no network calls and does not scan dependencies for vulnerabilities.

Dependencies are not always declared in one manifest. A repository can reference them from Dockerfiles, CI workflows, version catalogs, source code, and many ecosystem-specific files. `deplens` gives you one inventory across the whole repository and reports policy problems that can be checked without contacting an external service.

## Installation

Install the latest version with Go:

```bash
go install github.com/ferretsecurity/deplens/cmd/deplens@latest
```

Or build it from a local checkout:

```bash
go build ./cmd/deplens
```

## Usage

Scan the current directory:

```bash
deplens .
```

The path is optional and defaults to the current directory. Common options are:

```text
--json                         Emit machine-readable JSON
--rules rules.yaml             Replace the built-in ruleset
--extend-rules company.yaml    Append detectors and checks, repeatable
--exclude-preset NAME         Exclude snyk or socket source coverage, repeatable
--disable-rule ID              Exclude one detector ID, repeatable
--disable-check ID             Exclude one policy check ID, repeatable
--ignore dist,build,vendor     Replace the default ignored directories
--show-without-dependencies    Include sources confirmed to contain no dependencies
--generate python-requirements Generate requirements from eligible grouped sources
--overwrite-generated          Replace intended generated files after full preflight
```

Run `deplens --help` to see the complete command usage.

## Output

By default, `deplens` prints each dependency source, its type, and any dependencies it could extract. Policy findings appear after the inventory with a suggested remediation:

```text
Root: /work/example

Found 2 dependency sources:

Dockerfile [deployment-definition · 1 dependency]
  FROM:
    - ubuntu@22.04

package.json [manifest · 1 dependency]
  dependencies:
    - express@^5.1.0

Found 1 policy finding:

package.json [medium] npm project has dependencies but no npm lockfile
  check: javascript-npm-lockfile-missing
  expected: package-lock.json or npm-shrinkwrap.json
  remediation: Run `npm install` and commit the generated lockfile.
```

Use `--json` for machine-readable format. JSON output includes the detected sources, extracted dependency references, check runs, findings, and diagnostics. Findings do not change the successful exit status. See the [dependency reference specification](docs/dependency-reference.md) for the dependency object fields, values, and omission rules.

## Rules

`deplens` is driven by a YAML ruleset. Detector rules describe which files are dependency sources and how to analyze them. Check rules evaluate repository-wide policies after all sources have been found. Here is a small ruleset that extracts Go modules and checks that `go.sum` exists when needed:

```yaml
rules:
  - id: go-mod
    package-type: golang
    form: manifest
    roles: [declaration, constraint, resolution]
    filename-regex: '^go\.mod$'
    analyzer:
      type: go-mod

checks:
  - id: go-sum-missing
    summary: Go module has dependencies but no go.sum
    severity: medium
    evaluator:
      type: go-sum-missing
    remediation: Run `go mod tidy` and commit go.sum.
```

Pass a ruleset with `deplens --rules rules.yaml .`. A custom file replaces all built-in detectors and checks; it does not extend them. Rules use strict validation, so unknown fields and unsupported analyzer or evaluator types are rejected. See the [built-in rules](internal/analyze/default_rules.yaml) for complete examples and the [glossary](docs/glossary.md) for terms such as forms, roles, presence, and extraction.

Use `--extend-rules` to keep the built-ins and append company or team definitions:

```bash
deplens --extend-rules company.yaml --extend-rules team.yaml .
deplens --rules company-base.yaml --extend-rules team.yaml .
```

Each occurrence takes one path relative to the current working directory. Put flags before the scan path. Commas are part of filenames, not separators.

An extension uses the same strict YAML schema and contains one document with `rules`, `checks`, or both. A check-only extension is valid; an empty extension is not. A replacement base still requires at least one detector. The base's detectors run first, followed by extensions in flag order and detectors in document order. The first detector that recognizes a file wins. Checks run in stable ID order.

Detector IDs must be unique across the base and all extensions. Check IDs must also be unique, but a detector and a check may share an ID. Definitions never silently override one another. Invalid definitions and ID collisions fail before scanning, with the file origin and definition ID in the error.

Use exact, case-sensitive IDs to remove detectors or checks after composition:

```bash
deplens --disable-rule go-sum --disable-check dependency-source-codeowners-missing .
deplens --disable-rule js --extend-rules company-replacement.yaml .
```

Each exclusion takes one ID. Wildcards, comma lists, and analyzer names are not selectors. Repeated exclusions are harmless. IDs must exist in the combined base and extensions; a built-in ID omitted from a custom base is unknown. Exclusions apply last regardless of flag order. Invalid definitions and duplicate IDs still fail even if excluded. A replacement detector must use a different ID; surviving detectors keep their order and first-recognized behavior.

Disabled checks produce no runs or findings and do not change detection. Explicit detector exclusions can remove evidence required by a policy evaluator. Affected checks report a `detector-disabled` skip in human and JSON output, naming the removed prerequisite IDs and an identifiable project root, or the scan root when discovery is unavailable. This also applies to custom checks using the same evaluator.

Prerequisites include manifest discovery, accepted lockfile alternatives, package-manager evidence, and workspace ownership. Skips are conservative: removing an accepted alternative or competing manager detector skips the affected evaluator even if another lockfile or custom replacement survives. Unrelated evaluators continue. CODEOWNERS still checks surviving sources; when explicit exclusions leave no sources, it reports a skip. Missing detectors in a custom base alone do not cause exclusion skips.

Removing every detector or check is valid. JSON retains empty arrays and the existing schema. Findings and skips keep a successful exit status; configuration errors fail before scanning.

### Generate Python requirements

Generation is opt-in and works without a vendor preset. The built-in native Terraform, Python CDK, and TypeScript CDK Glue detectors are eligible. Custom YAML rules can opt in with `generate: python-requirements` and grouped extraction:

```yaml
rules:
  - id: company-workflows
    package-type: pypi
    form: automation-definition
    roles: [declaration, constraint]
    filename-regex: '^workflow\.yaml$'
    generate: python-requirements
    analyzer:
      type: yaml
      groups:
        query: '.workflows[]'
        name-query: '.name // $key'
        dependencies-query: '.configuration.python.dependencies'
```

The group query uses embedded jq and may select a root or nested list or mapping, including quoted keys and filters. It must select existing nodes from the source document; construction, merging, and reshaping are rejected. The name and dependency queries run relative to each selected group. For mapping values, `$key` contains the selected mapping key, so `name-query: '$key'` can name groups without a separate field.

Unique safe names are used unchanged. Unsafe filename characters are converted to `-`. Missing names, duplicate names, and names that collide after conversion receive a deterministic source-location suffix. Reports retain the original name. Generated files stay beside their source, and groups selected through filters keep their original source locations rather than their result positions.

Run generation with:

```bash
deplens --extend-rules company.yaml --generate python-requirements .
```

In CI, add `--overwrite-generated` when the workspace may contain output from an earlier run:

```bash
deplens --extend-rules company.yaml --generate python-requirements --overwrite-generated .
```

Without the overwrite flag, a rerun refuses the existing destination:

```text
$ deplens --extend-rules company.yaml --generate python-requirements .
error: generated destination already exists: workflow.yaml-daily.generated-requirements.txt
```

The explicit rerun replaces that planned file and reports the result:

```text
$ deplens --extend-rules company.yaml --generate python-requirements --overwrite-generated .
Generated 1 requirements file:
  workflow.yaml (daily) -> workflow.yaml-daily.generated-requirements.txt
```

Without `--generate`, the same command only scans. With generation enabled, a `daily` group in `workflow.yaml` produces `workflow.yaml-daily.generated-requirements.txt` beside the source. Each group gets a separate file. Names, version constraints, extras, and environment markers retain their declaration text. Local paths, URLs, Git requirements, pip options, included files, and malformed requirements fail generation.

Generation validates every selected source and destination before it writes. Missing dependency fields and empty lists are reported as separate successful skips. Existing destinations cause an error and remain unchanged unless `--overwrite-generated` is set. Overwrite replaces only the files planned by the current run. It does not follow destination symlinks, delete stale outputs, or provide a cross-file transaction if a filesystem write fails after writing starts. Generated paths in JSON are relative to the absolute scan `root`, so CI can resolve them with `root + path`. Poetry, uv, built-in detectors without explicit eligibility, disabled rules, and previously generated requirements files do not generate output.

For per-group Socket and Snyk jobs driven by those JSON paths, including zero-output handling and separate Python environments, see [Scan generated Python requirements in CI](docs/generated-requirements-vendor-ci.md). Live vendor acceptance status and exact fixtures are recorded there too.

The built-in `terraform.aws_glue_job.python` detector reads native `.tf` files and creates one group per `aws_glue_job` resource. It reads `--additional-python-modules` from `default_arguments` and `non_overridable_arguments`; a non-overridable value wins when both maps declare it. An omitted `--job-language` means Python, while a literal `scala` value excludes the job. Variables and other expressions are not evaluated. A dynamic language, modules value, relevant argument map, or installer option blocks generation before any files are written. Dynamic unrelated arguments do not block literal module extraction.

Git, URL, wheel, path, requirements-file, and custom-repository declarations are unsupported. Missing and empty module lists remain successful skips. Generated files describe the Terraform declarations, not the complete packages installed in a deployed Glue runtime.

For example, an ordinary scan of two Terraform Glue jobs reports their dependencies without writing files:

```text
jobs.tf [source-code · 3 dependencies]
```

Explicit generation writes each job separately:

```text
Generated 2 requirements files:
  jobs.tf (daily) -> jobs.tf-daily.generated-requirements.txt
  jobs.tf (legacy) -> jobs.tf-legacy.generated-requirements.txt
```

The built-in `typescript.cdk.aws_glue_job.python` detector is also eligible. Every statically readable Glue `CfnJob` is exported separately, using its construct ID when available. Duplicate or unreadable IDs use the same location-based disambiguation as YAML groups. If a selected job's properties or Python module declaration cannot be evaluated statically, generation fails before writing any planned file; deplens never executes CDK code.

Databricks bundle `.yaml` and `.yml` files are detected by their `resources.jobs.*.tasks` content, regardless of the filename or directory. Each base or target-specific task is a separate group. Deplens reads literal `libraries[].pypi.package` values and ignores Maven libraries in the same task. Git, wheel, URL or path, custom repository, variable, requirements-file, and malformed Python declarations are reported and stop all writes. The scanner does not follow bundle includes, resolve variables, or merge base and target settings. A generated file describes only the declarations in that source, task, and scope, not the complete deployed task.

```text
$ deplens bundle-root
Root: /workspace/bundle-root

Found 1 dependency source:

config/anything.yaml [manifest · 2 dependencies]
  ingest:
    - requests>=2.32
    - urllib3<3

$ deplens --generate python-requirements bundle-root
Generated 2 requirements files:
  config/anything.yaml (ingest) -> config/anything.yaml-ingest-at-resources.jobs.analytics.tasks-0.generated-requirements.txt
  config/anything.yaml (ingest) -> config/anything.yaml-ingest-at-targets.production.resources.jobs.analytics.tasks-0.generated-requirements.txt
```

For example, an ordinary scan of two TypeScript Glue jobs reports one source:

```text
jobs.ts [source-code · 3 dependencies]
```

Explicit generation adds one file per job without a vendor preset:

```text
Generated 2 requirements files:
  jobs.ts (daily) -> jobs.ts-daily.generated-requirements.txt
  jobs.ts (legacy) -> jobs.ts-legacy.generated-requirements.txt
```

Python CDK sources also produce one file per matching `aws_glue.CfnJob`. Deplens uses the construct ID as the group name when it is a static string and falls back to the call location when it is not. Aliases, multiline calls, reused argument dictionaries, duplicate IDs, and incompatible requirements remain separate. If any selected job cannot be read completely, generation fails before writing files. Deplens parses source text only. It does not import or execute Python code.

For example, two constructs named `daily` and `legacy` in `jobs.py` change the output from a normal scan with no file writes:

```text
jobs.py [source-code · 2 dependencies]
```

to an explicit generation report:

```text
Generated 2 requirements files:
  jobs.py (daily) -> jobs.py-daily.generated-requirements.txt
  jobs.py (legacy) -> jobs.py-legacy.generated-requirements.txt
```

Example output changes from an ordinary scan:

```text
workflow.yaml [automation-definition · 2 dependencies]
```

to an explicit generation report:

```text
Generated 2 requirements files:
  workflow.yaml (daily) -> workflow.yaml-daily.generated-requirements.txt
  workflow.yaml (legacy) -> workflow.yaml-legacy.generated-requirements.txt
```

### Vendor coverage presets

Use `deplens --extend-rules company.yaml --exclude-preset socket --disable-rule python-poetry-lock --disable-check javascript-npm-lockfile-missing .` to focus on source coverage gaps. Combine `--exclude-preset snyk --exclude-preset socket` for their union. Presets match IDs after composition; use new IDs for custom replacements. They skip affected checks and leave unrelated checks running.

The offline lists are the 2026-09-17 research snapshot, with 43 Snyk Open Source IDs and 41 Socket SCA IDs. Coverage may require vendor flags, restoration, builds, or documented adapters. A preset does not validate your actual vendor scan or its dependency reference coverage. See [membership, required setup, and full research evidence](docs/coverage-presets.md).


## Supported dependency sources

The 185 built-in detectors cover common package managers and languages as well as containers, CI systems, deployment tools, build systems, and infrastructure configuration. Some sources support full dependency extraction, while others can only be identified or checked for the presence of dependency references.

See [Dependency coverage](DEPENDENCY_COVERAGE.md) for the complete list of detectors and the capabilities available for each source.

## Security scanning

We use [Snyk](https://snyk.io/) and [Socket](https://socket.dev/) to scan this repository's dependencies for security risks. We also use GitHub's [CodeQL](https://codeql.github.com/) to scan Go code and GitHub Actions workflows for security issues, and [Dependabot](https://docs.github.com/en/code-security/dependabot) to alert us to vulnerable dependencies and open pull requests for security and version updates.
