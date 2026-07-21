# deplens

`deplens` scans a directory tree and reports dependency sources and policy findings. A dependency source is any file that can identify, declare, constrain, resolve, configure, use, or inventory dependencies. This includes manifests, requirements files, lockfiles, checksums, version catalogs, workspace and build definitions, automation and deployment files, tool configuration, source code, markup, and vendored files.

The CLI makes no network calls and performs no vulnerability scanning. Its 185 built-in detectors are embedded in the binary, and a complete inventory is available in [DEPENDENCY_COVERAGE.md](DEPENDENCY_COVERAGE.md).

## Usage

```bash
go run ./cmd/deplens [flags] [path]
```

Useful flags:

- `--json` emits machine-readable JSON.
- `--rules rules.yaml` replaces the built-in detector and check rules.
- `--ignore dist,build,vendor` replaces the default ignored directory list.
- `--show-without-dependencies` includes sources whose analysis found no dependency references.

The removed `--show-empty` flag is not accepted.

Human output is path-first and includes the source form plus its analysis state:

```text
Root: /work/example

Found 3 dependency sources:

frontend/package-lock.json [lockfile · 2 dependencies]
  dependencies:
    - react@18.3.1

package.json [manifest · references present, not extracted]
requirements.txt [requirements · 1 dependency]
  - requests==2.32.3
```

Absent sources are counted by `Found N dependency sources` but hidden from the detailed list unless `--show-without-dependencies` is supplied.

### Missing-lockfile findings

The built-in rules include five conservative missing-lockfile checks:

- `javascript-npm-lockfile-missing`
- `javascript-pnpm-lockfile-missing`
- `javascript-yarn-lockfile-missing`
- `python-uv-lockfile-missing`
- `rust-cargo-lockfile-missing-for-application`

JavaScript checks require an explicit npm, pnpm, or Yarn signal and a proven application signal such as `private: true`. The uv check requires uv-specific project configuration. The Cargo check requires a binary target such as `src/main.rs`. Dependency-free manifests are not flagged. Explicit workspace ownership is honored, and an unrelated ancestor lockfile does not satisfy a nested independent project. Ambiguous package-manager, workspace, or project-role cases are skipped rather than reported as findings.

For example, given this `package.json` without a lockfile:

```json
{
  "name": "api",
  "private": true,
  "packageManager": "npm@11.4.2",
  "dependencies": { "express": "^5.1.0" }
}
```

Previous output stopped after inventorying the manifest:

```text
Found 1 dependency source:

package.json [manifest · references present, not extracted]
```

The same scan now adds a concrete finding:

```text
Found 1 dependency source:

package.json [manifest · references present, not extracted]

Found 1 policy finding:

package.json [medium] npm project has dependencies but no npm lockfile
  check: javascript-npm-lockfile-missing
  expected: package-lock.json or npm-shrinkwrap.json
  remediation: Run `npm install` and commit the generated lockfile.
```

Findings do not change the default exit status. A successful scan exits zero even when findings are present.

## Output model

Each result names the detector, path, source form, source roles, analysis state, extracted dependency references, and structured diagnostics.

Presence is one of `unknown`, `absent`, or `present`. Extraction is one of `unsupported`, `complete`, `partial`, or `failed`. Only valid combinations are emitted; for example, a selector-only detector produces `unknown` + `unsupported`, while a successfully parsed empty lockfile produces `absent` + `complete`.

JSON uses schema version 1. In addition to dependency sources, it exposes check execution coverage and findings:

```json
{
  "schema_version": 1,
  "root": "/work/example",
  "sources": [
    {
      "detector": "js-npm-lock",
      "path": "package-lock.json",
      "form": "lockfile",
      "roles": ["resolution", "integrity"],
      "analysis": {
        "presence": "present",
        "extraction": "complete"
      },
      "dependencies": [
        {
          "package_type": "npm",
          "raw": "react@18.3.1",
          "name": "react",
          "version": "18.3.1",
          "source_group": "dependencies"
        }
      ]
    }
  ],
  "check_runs": [],
  "findings": []
}
```

`version` is the selected version. `version_constraint` is a declaration range or exact declaration specifier. The output does not use `resolved_version`.

## Detector rules

Rules have one stable `id`, one `form`, one or more `roles`, at least one selector, and optionally one nested analyzer. Unknown YAML fields are rejected at every level.

```yaml
rules:
  - id: go-mod
    package-type: golang
    form: manifest
    roles: [declaration, constraint, resolution]
    filename-regex: '^go\.mod$'
    analyzer:
      type: go-mod

  - id: project-requirements
    package-type: pypi
    form: requirements
    roles: [declaration, constraint]
    path-glob: '**/requirements/*.txt'
    analyzer:
      type: py-requirements
```

`filename-regex` matches the basename. `path-glob` matches the normalized root-relative path. If both are supplied, both must match. Rules are evaluated in order, and the first recognized source wins.

Analyzer-specific fields live beside `type` inside `analyzer`:

```yaml
analyzer:
  type: json
  exists-any:
    - dependencies
    - devDependencies
```

The old rule shape is rejected. The migration is intentionally atomic:

```yaml
# Before — rejected
- name: js
  dependency-type: npm
  filename-regex: '^package\.json$'
  json:
    exists-any: [dependencies]

# After
- id: js
  package-type: npm
  form: manifest
  roles: [declaration, constraint]
  filename-regex: '^package\.json$'
  analyzer:
    type: json
    exists-any: [dependencies]
```

The TOML analyzer supports `recognize-empty: true` for manifests that should remain visible even when configured dependency queries are empty. The built-in `python-pyproject` detector uses this so uv workspace roots can participate in repository relationships without being misclassified as having dependencies.

Checks live beside detectors in the same strict document. The MVP evaluator types have no configuration beyond their type discriminator:

```yaml
checks:
  - id: javascript-npm-lockfile-missing
    summary: npm project has dependencies but no npm lockfile
    severity: medium
    evaluator:
      type: npm-lockfile-missing
    remediation: Run `npm install` and commit the generated lockfile.
```

Supported evaluator types are `npm-lockfile-missing`, `pnpm-lockfile-missing`, `yarn-lockfile-missing`, `uv-lockfile-missing`, and `cargo-application-lockfile-missing`. Unknown fields are rejected. Ecosystem semantics and safe ambiguity behavior are built into each evaluator rather than exposed as YAML switches.

## Capabilities

Capabilities describe what a detector can do without forcing unrelated behavior into a maturity level:

- `select`: match a filename and/or relative path.
- `recognize`: validate content as the intended source format.
- `assess-presence`: determine whether dependency references are present.
- `extract`: return dependency references.
- `normalize`: populate shared fields such as package type, name, version, and constraint.
- `relate`: distinguish direct, transitive, or inconclusive relationships.
- `locate`: preserve where a reference came from within a source.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the implemented scan, analysis, relationship, and check pipeline.

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/deplens
```

When adding a detector, update this README and the coverage inventory, add a fixture under `testdata`, and add focused unit and scan integration tests.
