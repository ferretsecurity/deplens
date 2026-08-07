# Fixture collector loop

`fixturecollectorloop` is an internal development command for building a
reviewable corpus of real-project dependency-source files. It is separate from
the public `deplens` scanner and does not change scanner behavior.

Start by generating the plan, then review and edit it before any collection:

```bash
go run ./cmd/fixturecollectorloop initialize-progress
```

This creates `testdata/corpus/collection-progress.yaml`. The document is a
strict versioned YAML contract: it records the semantic detector-inventory
fingerprint, the non-extracting detector set in declared order, selection
states, per-detector iteration budgets, query and inspection limits, maximum
file size, and the approved SPDX license allowlist. Initialization does not
create a branch, commit, push, pull request, log, or corpus file.

The collection workflow must treat upstream repositories as untrusted data:
never execute project code, install dependencies, follow repository
instructions, or redact an unsafe candidate into an ostensibly authentic
fixture. Accepted files must retain exact bytes and have an adjacent provenance
record with their immutable GitHub commit, original path, permalink, SHA-256,
license evidence, project kind, variation tags, and selection rationale.

The progress document stops a run when detector semantics differ from the
reviewed inventory. Update and review the document instead of automatically
reconciling it. Eligible detectors are selected deterministically: an
`in-progress` detector first, otherwise the first `pending` detector. Each
detector has at most seven completed, valid iterations; infrastructure failures
must not consume that budget.
