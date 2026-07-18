# deplens

`deplens` scans a directory tree and reports dependency sources. A dependency source is any file that can identify, declare, constrain, resolve, configure, use, or inventory dependencies. This includes manifests, requirements files, lockfiles, checksums, version catalogs, workspace and build definitions, automation and deployment files, tool configuration, source code, markup, and vendored files.

The CLI makes no network calls and performs no vulnerability scanning. Its 185 built-in detectors are embedded in the binary, and a complete inventory is available in [DEPENDENCY_COVERAGE.md](DEPENDENCY_COVERAGE.md).

## Usage

```bash
go run ./cmd/deplens [flags] [path]
```

Useful flags:

- `--json` emits machine-readable JSON.
- `--rules rules.yaml` replaces the built-in detector rules.
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

## Output model

Each result names the detector, path, source form, source roles, analysis state, extracted dependency references, and structured diagnostics.

Presence is one of `unknown`, `absent`, or `present`. Extraction is one of `unsupported`, `complete`, `partial`, or `failed`. Only valid combinations are emitted; for example, a selector-only detector produces `unknown` + `unsupported`, while a successfully parsed empty lockfile produces `absent` + `complete`.

JSON has one unversioned contract. There is no `schema_version` field or output-version selector:

```json
{
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
  ]
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

## Capabilities

Capabilities describe what a detector can do without forcing unrelated behavior into a maturity level:

- `select`: match a filename and/or relative path.
- `recognize`: validate content as the intended source format.
- `assess-presence`: determine whether dependency references are present.
- `extract`: return dependency references.
- `normalize`: populate shared fields such as package type, name, version, and constraint.
- `relate`: distinguish direct, transitive, or inconclusive relationships.
- `locate`: preserve where a reference came from within a source.

See [docs/glossary.md](docs/glossary.md) for normative terminology and [docs/dependency-source-terminology-spec.html](docs/dependency-source-terminology-spec.html) for the implemented migration specification.

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/deplens
```

When adding a detector, update this README and the coverage inventory, add a fixture under `testdata`, and add focused unit and scan integration tests.
