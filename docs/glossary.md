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

The strict version 2 record adjacent to a corpus example. The redesigned collector accepts no earlier provenance version. It records the detector and stable candidate identity, provider, repository and URL, default branch, immutable commit, original path, immutable permalink, retrieval time, source SHA-256, governing license SPDX identifier/path/permalink/SHA-256, and model-authored selection rationale. Go derives and writes every field except the rationale.

### Collection progress

The strict new-format document containing reviewed plans and append-only valid iteration history. It records structured Go-observed accounting, candidate facts and outcomes, selection-state fingerprints, model decisions, and final validation results without storing upstream contents. Go reconstructs current state from this history so independent iterations can resume without hidden conversational context.

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

### Source candidate

A dependency source from a real public project that is being considered for the corpus but has not passed final validation. A source candidate is not a corpus example.

### Qualification

Deterministic checks performed before model selection to establish whether a source candidate is eligible to be considered. Qualification does not replace final wrapper validation.

### Qualified source candidate

A source candidate that passed qualification and is eligible for model selection. Omission from one selection packet does not change that eligibility.

### Model-presentable source candidate

A qualified source candidate whose exact, complete source is safe UTF-8 and can fit by itself in the encoded selection-packet budget. Aggregate packet omission is separate and does not make a candidate non-presentable.

### Stable candidate ID

A deterministic identifier derived from a source candidate's provider, repository, immutable commit, and path. It identifies the upstream source independently of its content hash.

### Selection packet

The bounded set of accepted corpus references, qualified source candidates, and supporting evidence presented to one model session for comparison. Its token budget includes both references and candidates.

### Accepted corpus reference

An existing corpus example included with exact content, immutable identity, and stored selection rationale as mandatory comparison material in a resumed selection packet. It is marked as already accepted and cannot be selected again.

### Selected source candidate

A qualified source candidate chosen by the model for materialization and final validation. The model identifies it by stable candidate ID and supplies a selection rationale; ordering among selected candidates has no domain meaning. It becomes a corpus example only after final validation succeeds.

### Selection nonselection

The contextual outcome that the model did not choose a qualified source candidate from a particular selection packet against a particular accepted corpus state. It is not a deterministic rejection; the candidate may be reconsidered when the comparison state changes, but the same packet and corpus state are not submitted again.

### Selection rationale

The model-authored, non-empty explanation of what a selected source candidate demonstrates and how it complements the final set. It is bounded to 1,000 UTF-8 characters, excludes control characters, is preserved verbatim in provenance, and is not semantically reinterpreted by deterministic validation.

### Query plan

The reviewed, ordered search expressions that Go may execute to discover source candidates for one detector. Go may generate an initial plan from detector signals, but the model neither proposes nor executes its queries.

### Needs query review

A non-blocking detector state indicating that Go could not generate a sufficiently precise query plan for safe automatic discovery. The collection loop skips the detector without treating the state as a search failure.

### Needs content review

A non-blocking detector state indicating that otherwise eligible sources required to reach the corpus minimum cannot be presented to the model as exact, complete, safe UTF-8 content within the selection-packet budget. The research outcome that establishes this state consumes an iteration; later collection runs skip the detector.

### Needs collection review

A non-blocking detector state indicating that automatic research ended with fewer than three accepted examples after seven valid iterations or because no distinct selection state remains. Later runs skip the detector; manual fulfillment is outside the collector.

### Collection accounting

The mandatory record of actual discovery, acquisition, qualification, and selection-packet activity. Accounting is produced by Go operations rather than model reports.

### Candidate inspection

The first detailed metadata or content request for one unique repository-and-path pair in one collection iteration. Duplicate search hits do not consume another inspection, but a failed request and deliberate reconsideration in a later iteration do.

### Candidate inspection target

The normal number of candidate inspections attempted in one collection iteration. It is not a hard ceiling: when fewer than five candidates qualify at the target, qualification continues until five qualify or another hard collection budget is exhausted.

### Outcome reason code

A closed, stage-specific Go enum identifying a durable candidate or infrastructure outcome. Optional bounded diagnostic detail may accompany it, but external error text and free-form wording never determine control flow.

### Collection budget

A configurable hard limit on resource-consuming collection activity, enforced by Go at the point of consumption. Queries, result pages, downloaded bytes, selection-packet tokens, and selector invocations are budgeted; derived outcomes and the candidate-inspection target are only accounted for.

### License evidence

Proof that an allowlisted license governs a source candidate, consisting of a detected SPDX identifier and the governing license file fetched and hashed at the candidate's immutable commit. Missing, disallowed, conflicting, or ambiguous evidence fails qualification.

### Acquisition snapshot

The source bytes and supporting evidence fetched consistently from the default-branch head resolved at a recorded acquisition time. Later movement of the branch does not invalidate this immutable snapshot.

### Valid research outcome

A completed, checkpointed collection attempt that found, rejected, selected, or exhausted candidates within its collection budgets. It consumes one collection iteration even when it accepts no corpus example.

### Collection infrastructure failure

A failure of authentication, a remote provider, the model protocol, materialization, or the collector itself that prevents a valid research outcome. It does not consume a collection iteration.

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
