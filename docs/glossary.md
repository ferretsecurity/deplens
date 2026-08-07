# Dependency source glossary

This glossary defines the vocabulary used by rules, Go APIs, JSON, CLI text, tests, and documentation.

## Core terms

### Dependency source

A file that can identify, declare, constrain, resolve, configure, use, or inventory dependencies. This is the umbrella term. A manifest and a lockfile are forms of dependency source; they are not synonyms for the umbrella term.

### Detector

A rule that selects candidate files and may analyze their content. A detector has a stable `id`, exactly one `form`, one or more `roles`, selectors, and optionally an analyzer.

### Selector

A cheap path-based test: `filename-regex`, `path-glob`, or both. Both selectors use AND semantics when present together. Selection does not imply content recognition.

### Analyzer

Content-aware logic that may recognize a source, assess dependency presence, extract references, and emit diagnostics. Syntax parsers are implementation details used by analyzers.

### Dependency reference

One dependency value obtained from a source. `raw` preserves the source value. Shared fields include `package_type`, `name`, `version`, `version_constraint`, `vers`, `source_group`, `origin_kind`, `relationship`, `scope`, and `attributes`.

### Diagnostic

A structured warning or error with `severity`, stable kebab-case `code`, and human-readable `message`.

## Fixture corpus

### Corpus example

An exact, byte-for-byte dependency-source file collected from an identifiable revision of a real project for future analyzer development and realistic detector testing.

### Example set

The deliberately varied corpus examples collected for one detector. An example set represents typical and edge-case dependency-relevant structures or facts across relevant project kinds rather than merely satisfying a numeric quota.

### Provenance record

The immutable origin and selection evidence associated with a corpus example, including its upstream repository, commit, path, license, retrieval date, and content hash.

### Collection run

One time-bounded invocation that performs collection iterations until its global stop condition is reached. Repeated collection runs continue the same reviewable body of work rather than publishing independently.

### Collection iteration

One resumable unit of a collection run, focused on advancing the example set for one detector and recording the resulting progress transition.

### Collection PR

The long-lived pull request through which corpus examples and their provenance records are reviewed before entering the main branch.

### Collection progress

The human-reviewable, machine-readable record that defines eligible detectors, example-set targets, iteration budgets, completed work, and the next unfinished collection iteration.

## Source forms

A source has exactly one physical or syntactic form:

- `manifest`
- `requirements`
- `lockfile`
- `constraint-file`
- `checksum-file`
- `version-catalog`
- `workspace-definition`
- `build-definition`
- `automation-definition`
- `deployment-definition`
- `tool-config`
- `source-code`
- `markup`
- `vendored-file`
- `other`

## Source roles

A source has one or more semantic roles:

- `declaration`: declares dependency references.
- `constraint`: limits acceptable versions or origins.
- `resolution`: records selected dependency versions or revisions.
- `integrity`: records hashes, checksums, or integrity metadata.
- `configuration`: configures dependency tooling or behavior.
- `workspace`: defines project or package membership.
- `usage`: directly refers to a dependency in code, markup, automation, or deployment content.
- `inventory`: records observed, installed, or vendored dependencies.

Form and role are independent. For example, `go.mod` is a `manifest` with declaration, constraint, and resolution roles.

## Analysis state

### Presence

- `unknown`: the analyzer cannot determine whether references exist.
- `absent`: the analyzer determined that no references exist.
- `present`: references are known to exist.

### Extraction

- `unsupported`: extraction was not attempted or is not implemented.
- `complete`: extraction completed for the analyzer's defined scope.
- `partial`: useful references were returned, but analysis was incomplete.
- `failed`: analysis failed and no usable references were returned.

Valid pairs are: unknown/unsupported, unknown/failed, absent/unsupported, absent/complete, present/unsupported, present/complete, and present/partial.

## Dependency fields

### `version`

The selected version or revision, commonly obtained from a lockfile. This project does not use `resolved_version`.

### `version_constraint`

A declaration range or exact declaration specifier from the source.

### `source_group`

The logical source section or group, such as `dependencies`, `devDependencies`, or `project.optional-dependencies.dev`.

### `origin_kind`

How a dependency is obtained: `registry`, `git`, `path`, `url`, or `workspace`.

### `relationship`

Whether a reference is `direct`, `transitive`, or `inconclusive`.

### `scope`

The dependency scope when known: `runtime`, `development`, `test`, `build`, or `optional`.

## CLI

### `--show-without-dependencies`

Includes sources whose presence is `absent` in detailed human output. It does not alter scanning or JSON. The former `--show-empty` name is removed.
