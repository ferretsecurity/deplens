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
--ignore dist,build,vendor     Replace the default ignored directories
--show-without-dependencies    Include sources confirmed to contain no dependencies
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

## Supported dependency sources

The 185 built-in detectors cover common package managers and languages as well as containers, CI systems, deployment tools, build systems, and infrastructure configuration. Some sources support full dependency extraction, while others can only be identified or checked for the presence of dependency references.

See [Dependency coverage](DEPENDENCY_COVERAGE.md) for the complete list of detectors and the capabilities available for each source.
