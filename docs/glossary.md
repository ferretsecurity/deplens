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

## Fixture collection

### Corpus

The authoritative collection of validated corpus examples and their provenance records. Files at corpus paths enter the authoritative collection only when collection progress records their successful wrapper validation.

### Corpus example

An exact, byte-for-byte dependency source retained from a real public project for future analyzer design and testing. Every corpus example has an adjacent provenance record.

### Provenance record

Structured evidence identifying a corpus example's immutable upstream revision, origin, license, content hash, project classification, variations, and selection rationale.

### Collection progress

The strict, versioned document that records the reviewed collection plan and durable outcomes so independent collection iterations can resume without hidden conversational context.

### Collection iteration

One bounded attempt by a fresh agent session to advance the example set for one detector. The wrapper validates the attempt as a unit before accepting its changes or optionally committing them.

### Collection run

One invocation that executes one or more collection iterations within the requested mode and time limit.

### Collection commit

An optional local Git commit created by the wrapper after one valid collection iteration. It contains only validated collector-owned changes and is never pushed by the collector.

### Collection checkpoint

The durable collection state after a valid iteration has updated collection progress. In commit mode it is represented by `HEAD`; without commits it remains valid collection state in the working tree. A resumed run continues from this checkpoint rather than from an interrupted agent session.

### Valid collection state

Collector-owned files that are fully described by collection progress and satisfy the collector's integrity rules. A later collection run may revalidate and resume from this state even when it has not been committed.

### Unvalidated collection changes

Collector-owned file changes left by an iteration that did not complete wrapper validation. They are preserved for diagnosis, but collection cannot resume until a human resolves them.

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
