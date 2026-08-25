# deplens

`deplens` scans a directory tree and reports dependency sources and policy findings. A dependency source is any file that can identify, declare, constrain, resolve, configure, use, or inventory dependencies. This includes manifests, requirements files, lockfiles, checksums, version catalogs, workspace and build definitions, automation and deployment files, tool configuration, source code, markup, and vendored files.

The CLI makes no network calls and performs no vulnerability scanning. Its 185 built-in detectors are embedded in the binary, and a complete inventory is available in [DEPENDENCY_COVERAGE.md](DEPENDENCY_COVERAGE.md).

Unless an example is demonstrating ownership behavior, its dependency sources are assumed to be covered by a catch-all CODEOWNERS rule.

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

### Analyzer implementation loop

`analyzerloop` turns the separately verified fixture corpus into a reviewed work ledger for semantic analyzer work. It never contacts a package registry or GitHub itself.

```bash
go run ./cmd/analyzerloop plan --corpus ../deplens-fixture-corpus
# Review and commit .deplens/analyzer-implementation.yaml.
go run ./cmd/analyzerloop run --select 1...3
```

Planning accepts only corpus work items whose result is `OK` and whose three candidates are all valid and hash-verified. A run uses fresh implementer and verifier sessions, creates three synthesized test fixtures per item, and commits every accepted checkpoint. Runtime logs are private local data under `.ralph/` and are ignored. See [the operator guide](docs/agents/analyzer-loop.md).

Human output is path-first and includes the source form plus its analysis state:

```text
Root: /work/example

Found 3 dependency sources:

frontend/package-lock.json [lockfile · 2 dependencies]
  dependencies:
    - react@18.3.1

package.json [manifest · 1 dependency]
  dependencies:
    - express@^5.1.0
requirements.txt [requirements · 1 dependency]
  - requests==2.32.3
```

Absent sources are counted by `Found N dependency sources` but hidden from the detailed list unless `--show-without-dependencies` is supplied.

### Semantic coverage for common formats

`package.json`, Maven POM, Cargo, Composer, .NET, Buf manifests and lockfiles, Rebar3, Clojure `deps.edn`, Clojure Boot, Chef Berksfile manifests and lockfiles, Chef Policyfiles and Policyfile lockfiles, and Chef cookbook metadata; Gradle builds, lockfiles, and version catalogs; Gemfiles and Bundler lockfiles; Dockerfiles; and Docker Compose files have built-in semantic analyzers. Buf lockfiles extract resolved Buf Schema Registry module names and commits from both v1 and v2 formats. Berksfiles extract Supermarket, GitHub, Git, and local cookbook declarations. Policyfiles extract static Git and local cookbook declarations, including version constraints and source tags. Berksfile lockfiles extract resolved cookbooks from legacy text and JSON formats, including Git and local sources. Policyfile lockfiles extract resolved cookbooks and their registry, Git, or local origins. Chef metadata extracts direct cookbook declarations and static cookbook lists. Rebar3 extracts regular, Git, Git-subdirectory, plugin, and profile dependency declarations. Clojure `deps.edn` extracts root and alias Maven, Git, and local dependency coordinates. Clojure Boot extracts static `set-env! :dependencies` declarations, including test-scoped dependencies. Static declarations are normalized into dependency records; executable or interpolated declarations that cannot be resolved without running external tools produce partial analysis warnings.

For example, `package.json` was previously limited to presence assessment:

```text
package.json [manifest · references present, not extracted]
```

Given:

```json
{
  "dependencies": {
    "react": "^19.0.0",
    "server": "npm:@acme/server@^3.2.0",
    "@acme/ui": "workspace:^"
  },
  "devDependencies": {
    "typescript": "~5.8.0"
  }
}
```

the dedicated analyzer now preserves the declarations and their source groups:

```text
package.json [manifest · 4 dependencies]
  dependencies:
    - @acme/ui@workspace:^
    - react@^19.0.0
    - server@npm:@acme/server@^3.2.0
  devDependencies:
    - typescript@~5.8.0
```

JSON additionally normalizes registry constraints, npm aliases, direct relationships, runtime/development/optional scopes, and registry, workspace, path, Git, or URL origins. Package-manager and workspace metadata remains available to policy evaluation; it is not emitted as a dependency.

For example, this Gradle build was previously identified only:

```text
build.gradle.kts [build-definition · identified only]
```

Given:

```kotlin
dependencies {
    implementation("org.springframework:spring-core:6.1.8")
    implementation(libs.jackson.databind)
}
```

the same source now returns the static Maven dependency and explains the unresolved alias:

```text
build.gradle.kts [build-definition · 1 dependency · partial]
  implementation:
    - org.springframework:spring-core[6.1.8]
  warning [dependency-extraction-incomplete]: version-catalog alias libs.jackson.databind could not be resolved from this build file
```

Dockerfile analysis extracts external images from `FROM` and external `COPY --from` instructions, while excluding `scratch` and previously declared stage aliases. Compose analysis extracts `services.*.image`. It does not treat packages installed by `RUN` or local build contexts as package dependencies.

Maven POMs, Cargo manifests, Composer manifests, and .NET project or central-package files also preserve source groups, constraints, scopes, and relationships. Consuming declarations are direct; non-consuming catalogs such as Maven `dependencyManagement`, Cargo `workspace.dependencies`, and .NET `PackageVersion` entries are inconclusive:

```text
pom.xml [manifest · 2 dependencies]
  dependencies:
    - org.slf4j:slf4j-api[2.0,3.0)
  dependencyManagement:
    - org.springframework.boot:spring-boot-dependencies[3.3.2]

Cargo.toml [manifest · 2 dependencies]
  dependencies:
    - serde^1.0.0
  workspace.dependencies:
    - anyhow^1.0
```

Maven and MSBuild properties declared unconditionally in the same file are resolved while their original expressions remain in JSON `raw`. Unresolved expressions preserve the dependency name and raw constraint, mark extraction partial, and emit `dependency-extraction-incomplete`. Composer platform requirements such as `php`, `ext-*`, and `lib-*` are not emitted as package dependencies.

### Dependency policy findings

The built-in rules include ten conservative dependency policy checks:

- `javascript-npm-lockfile-missing`
- `javascript-pnpm-lockfile-missing`
- `javascript-yarn-lockfile-missing`
- `javascript-conflicting-lockfiles`
- `python-uv-lockfile-missing`
- `rust-cargo-lockfile-missing-for-application`
- `go-sum-missing`
- `php-composer-lockfile-missing-for-application`
- `ruby-gemfile-lockfile-missing-for-application`
- `dependency-source-codeowners-missing`

JavaScript checks require explicit npm, pnpm, or Yarn evidence; package publishability (`private`) does not affect eligibility. The uv check requires uv-specific project configuration. The Cargo check requires a binary target such as `src/main.rs`. The Go check requires an external `require` entry; modules whose requirements are all locally replaced are skipped. Composer checks require `type: project` and ignore platform-only requirements. Ruby checks require a static `gem` declaration and skip Gemfiles with a `gemspec` directive. Dependency-free projects are not flagged. Explicit workspace ownership is honored where the ecosystem defines it, and an unrelated ancestor lockfile does not satisfy a nested independent project. Ambiguous package-manager or application-role cases are skipped rather than reported as findings.

The conflicting-lockfiles check reports a JavaScript project when its owned root contains lockfiles from at least two package-manager families: npm, pnpm, or Yarn. `package-lock.json` and `npm-shrinkwrap.json` are both npm lockfiles and do not conflict with each other. The check does not require dependency declarations because competing committed lockfiles are independently actionable.

The CODEOWNERS check reports every dependency source that does not resolve to at least one syntactically valid owner. It includes sources confirmed to contain no dependency references, because those files can gain dependencies later. A repository without a usable CODEOWNERS file receives one finding per source. The check operates offline: it validates owner syntax, but cannot verify that a user or team exists or has repository access.

GitHub repositories use `.github/CODEOWNERS`, root `CODEOWNERS`, then `docs/CODEOWNERS`; GitLab repositories use root `CODEOWNERS`, `docs/CODEOWNERS`, then `.gitlab/CODEOWNERS`. Candidates must be readable regular files; symlinks and other non-regular files fail the check. Auto mode recognizes provider-specific locations. It evaluates root and `docs` files with both dialects and accepts them only when source coverage agrees. If both providers are signaled or their interpretations differ, the check fails with `codeowners-platform-ambiguous` instead of emitting uncertain findings. Failed policy checks and their reasons are included in human output as well as JSON.

For example, this CODEOWNERS file does not cover the detected `package.json`:

```text
# CODEOWNERS
*.go @backend-team
```

The default scan reports the ownership gap:

```text
Found 1 dependency source:

package.json [manifest · 1 dependency]
  dependencies:
    - express@^5.1.0

Found 1 policy finding:

package.json [medium] Dependency source has no code owner
  check: dependency-source-codeowners-missing
  remediation: Create or update CODEOWNERS so the dependency source matches at least one valid owner.
```

Adding `/package.json @dependency-team` removes the policy finding without changing the dependency inventory.

For example, this project contains both npm and pnpm lockfiles:

```text
.
├── package.json
├── package-lock.json
└── pnpm-lock.yaml
```

Previously, the three missing-lockfile checks treated the package-manager evidence as ambiguous and emitted no finding:

```text
Found 3 dependency sources:

package-lock.json [lockfile · 1 dependency]
  dependencies:
    - left-pad@1.3.0

package.json [manifest · references present, not extracted]

pnpm-lock.yaml [lockfile · 1 dependency]
  dependencies:
    - left-pad@1.3.0
```

The same scan now reports the conflicting files:

```text
Found 3 dependency sources:

package-lock.json [lockfile · 1 dependency]
  dependencies:
    - left-pad@1.3.0

package.json [manifest · 1 dependency]
  dependencies:
    - left-pad@^1.3.0

pnpm-lock.yaml [lockfile · 1 dependency]
  dependencies:
    - left-pad@1.3.0

Found 1 policy finding:

package.json [medium] JavaScript project has conflicting package-manager lockfiles
  check: javascript-conflicting-lockfiles
  conflicting: package-lock.json, pnpm-lock.yaml
  remediation: Choose one package manager, remove lockfiles from the others, and declare packageManager in package.json.
```

For example, given this `package.json` without a lockfile:

```json
{
  "name": "api",
  "packageManager": "npm@11.4.2",
  "dependencies": { "express": "^5.1.0" }
}
```

Previously, the missing-lockfile evaluator skipped this project because `private: true` was absent, so output stopped after inventorying the manifest:

```text
Found 1 dependency source:

package.json [manifest · references present, not extracted]
```

The same scan now adds a concrete finding:

```text
Found 1 dependency source:

package.json [manifest · 1 dependency]
  dependencies:
    - express@^5.1.0

Found 1 policy finding:

package.json [medium] npm project has dependencies but no npm lockfile
  check: javascript-npm-lockfile-missing
  expected: package-lock.json or npm-shrinkwrap.json
  remediation: Run `npm install` and commit the generated lockfile.
```

Findings do not change the default exit status. A successful scan exits zero even when findings are present.

For uv workspaces, a dependency-free root such as the following is policy metadata rather than a dependency source:

```toml
[project]
dependencies = []

[tool.uv.workspace]
members = ["packages/*"]
```

Previously, `recognize-empty: true` caused that root to appear in `sources` with `presence: absent`. It is now omitted from dependency inventory, but the uv evaluator still uses it to attach dependency-bearing members to the workspace root and anchors any missing-`uv.lock` finding at the root `pyproject.toml`.

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
  "check_runs": [
    {
      "check_id": "dependency-source-codeowners-missing",
      "subject": { "project_root": "." },
      "status": "completed"
    }
  ],
  "findings": []
}
```

A finding identifies its logical project only by normalized root; concrete files belong in `locations`:

```json
{
  "subject": { "project_root": "." },
  "locations": [{ "path": "package.json" }]
}
```

Finding fingerprints have their own format version, independent of the JSON schema version.

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

The configuration-free semantic analyzer types `package-json`, `gradle-build`, `gradle-lock`, `gradle-version-catalog`, `gemfile`, `gemfile-lock`, `dockerfile`, `docker-compose`, `maven-pom`, `cargo-manifest`, `composer-manifest`, `dotnet-project`, `dotnet-central-packages`, `dotnet-packages-config`, `buf`, `buf-lock`, and `erlang-rebar-config` can also be used by custom rules. They reject analyzer fields other than `type`.

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
    type: package-json
```

Generic TOML rules recognize a source only when their configured queries establish dependency relevance. When the uv check is enabled, the scanner separately retains `pyproject.toml` bytes as policy inputs. The check layer parses uv and workspace facts from those inputs, so a dependency-free workspace root can own members without being emitted as a dependency source or changing generic TOML semantics.

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

Supported evaluator types are `npm-lockfile-missing`, `pnpm-lockfile-missing`, `yarn-lockfile-missing`, `javascript-conflicting-lockfiles`, `uv-lockfile-missing`, `cargo-application-lockfile-missing`, `go-sum-missing`, `composer-application-lockfile-missing`, `gemfile-application-lockfile-missing`, and `dependency-source-codeowners`. Unknown fields are rejected. Other evaluator semantics and safe ambiguity behavior remain built-in invariants.

The CODEOWNERS evaluator accepts one optional platform selector. `auto` is the default; use an explicit provider for mixed-host or otherwise ambiguous repositories:

```yaml
checks:
  - id: dependency-source-codeowners-missing
    summary: Dependency source has no code owner
    severity: medium
    evaluator:
      type: dependency-source-codeowners
      platform: gitlab # auto, github, or gitlab
    remediation: Create or update CODEOWNERS so the dependency source matches at least one valid owner.
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

See [docs/glossary.md](docs/glossary.md) for normative terminology.

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/deplens
```

When adding a detector, update this README and the coverage inventory, add a fixture under `testdata`, and add focused unit and scan integration tests.
