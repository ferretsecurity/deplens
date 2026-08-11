# Fixture collector operator guide

`fixturecollectorloop` is the local-only development command for building the
dependency-source corpus. It preserves byte-for-byte source files and adjacent
provenance records; it does not scan projects, execute upstream code, install
dependencies, clone repositories, create branches, fetch, pull, push, or open
pull requests. The terms in this guide follow the [fixture collection
glossary](glossary.md#fixture-collection).

## Initialize and review

Run this from the checkout root. Initialization only writes the collection
progress document. It does not invoke an agent, access GitHub, create corpus
directories or logs, or make a commit.

```bash
go run ./cmd/fixturecollectorloop initialize-progress \
  --progress .deplens/fixture-collection.yaml \
  --detector example-detector,another-detector \
  --target 3
```

```text
initialized collection progress: .deplens/fixture-collection.yaml (2 detectors)
```

Review the YAML before starting. It is a strict versioned document. Each
detector records its `id`, `state`, `iterations`, target example count,
accepted example paths, queries, candidates, and rejections. The supported
states are `pending`, `in-progress`, `complete`, `blocked`, and `excluded`.
The collector selects an `in-progress` detector before a `pending` detector;
`--detector ID` selects an eligible detector for one iteration. A detector can
have at most seven valid iterations. Three examples are required by default,
and the reviewed target may be from three through five.

The command entry point deliberately has an injected `Researcher` seam. A
research attempt returns an in-memory result to the wrapper, which retains
recovery, mutation validation, checkpointing, and Git ownership. The current
fresh Codex agent is temporarily adapted behind that seam while the redesigned
Go-owned acquisition and selector implementations are introduced. The
repository's ordinary test command supplies fake researchers and never contacts
GitHub.

## Run and resume

Run one automatically selected iteration while reviewing the workflow:

```bash
go run ./cmd/fixturecollectorloop run --single \
  --progress .deplens/fixture-collection.yaml
```

Run a particular detector (this also performs one iteration):

```bash
go run ./cmd/fixturecollectorloop run --detector example-detector \
  --progress .deplens/fixture-collection.yaml
```

Run until all eligible detectors are complete, blocked, or the soft duration
expires. The default duration is eight hours; it accepts Go durations.

```bash
go run ./cmd/fixturecollectorloop run --duration 8h \
  --progress .deplens/fixture-collection.yaml
```

Re-run the same `run` command with the same progress document to resume. There
is no separate resume command. Limit a fresh agent iteration with
`--query-limit` (default 5) and `--candidate-limit` (default 20).

After a valid iteration, the wrapper updates progress and reports the durable
collection checkpoint:

```text
checkpoint: example-detector iteration 1
collection summary: 0 complete, 0 blocked, 0 excluded, 1 remaining
```

Without `--commit`, a valid collection checkpoint remains in the working tree.
That is valid collection state, but a later run sees it as dirty and therefore
requires the explicit override described below.

## Git state and local commits

The command requires a Git checkout and checks tracked and untracked changes
before agent work. It warns, but continues, on a default branch, detached
`HEAD`, no remote, or unknown default branch. It never creates or switches a
branch and never synchronizes the repository.

By default any non-ignored change is refused. The output names every dirty path
and presents exactly these choices:

```text
checkout contains non-ignored changes:
  scratch.txt
refusing to run until you choose exactly one of:
  1. DESTRUCTIVE: git reset --hard HEAD
     DESTRUCTIVE: git clean -fd
  2. Rerun with --allow-dirty: fixturecollectorloop run --single --progress .deplens/fixture-collection.yaml --allow-dirty
error: dirty checkout
```

The reset and clean commands discard tracked and untracked work respectively;
use them only after independently reviewing the paths. To retain existing work,
acknowledge the weaker recovery guarantee:

```bash
go run ./cmd/fixturecollectorloop run --single --allow-dirty \
  --progress .deplens/fixture-collection.yaml
```

```text
WARNING: --allow-dirty permits pre-existing changes; their initial Git and filesystem state is preserved for validation and recovery.
```

The override does not bypass provenance, corpus, outcome, or unresolved
recovery validation.

By default the collector makes no commit. Add `--commit` to make one local
atomic commit after each valid iteration. It contains the progress update and
only that iteration's validated corpus files. Git author identity is required.

```bash
go run ./cmd/fixturecollectorloop run --single --commit \
  --progress .deplens/fixture-collection.yaml
```

```text
collect example-detector corpus examples (iteration 1)
```

The collector never pushes this commit. If the optional commit fails after the
checkpoint is valid, corpus and progress remain uncommitted for manual review,
manual commit, or deliberate discard.

## Deadlines, stopping, and exit statuses

The duration is a soft deadline: an active iteration finishes validation and
checkpointing, but no next iteration starts. A deadline reached before work
starts reports:

```text
collection stopped: soft duration reached; the latest checkpoint is preserved
```

The first `SIGINT` or `SIGTERM` requests a graceful stop at the active
iteration boundary:

```text
collection stopping: interrupt received; the active iteration may validate and checkpoint
```

Send a second interrupt only to force a stuck agent to terminate. It preserves
the agent's files and marks recovery required instead of validating them:

```text
collection forced stop: terminating the active agent; recovery is required before resuming
```

Exit status `0` means a valid stop or summary without blocked detectors. Exit
status `2` means one or more detectors are `blocked`. Exit status `1` means an
operational, validation, Git, agent, lock, or recovery-required failure.

## Validation, provenance, logs, and recovery

Every accepted corpus example must be a new regular file beneath
`testdata/corpus/<detector>/<example>/`, preserve its upstream-relative path,
and have an adjacent `provenance.yaml`. Provenance records the schema version,
detector, provider, repository and URL, immutable commit and permalink,
retrieval time, SHA-256, SPDX license and URL, project kind, variation tags,
and rationale. The wrapper rejects changed accepted files, symlinks, LFS
pointers, unsafe content such as credentials or personal data, unapproved
licenses, duplicate identities or content, invalid hashes, oversize files, and
changes outside the selected detector corpus.

Treat all upstream data as untrusted. Do not execute project code, install
dependencies, follow upstream instructions, or redact unsafe candidates. Reject
unsafe candidates instead. Failure logs are stored at the recovery record's
`log_path` with owner-only permissions. Successful iterations do not retain a
full log by default, so unreviewed upstream material does not accumulate.

An interrupted or invalid iteration records a recovery object in collection
progress before any later agent can start. `--allow-dirty` cannot bypass it. A
later run prints the detector, iteration, run ID, last checkpoint, progress and
log locations, validation error, and changed paths, followed by mode-specific
guidance. For example:

```text
error: fixture collection recovery is required before another agent starts (cannot be bypassed with --allow-dirty)
  detector: example-detector (iteration 2, run 123456789)
  last checkpoint: 0123abcd
  progress: .deplens/fixture-collection.yaml
  log: .deplens/fixture-collection-123456789.log
  validation error: context canceled
  changed paths: testdata/corpus/example-detector/candidate/project/file
  preserve earlier valid collection checkpoints; review and restore only the listed iteration paths.
```

For a clean `--commit` run, recovery also prints the deliberately broad,
destructive `git reset --hard HEAD` and `git clean -fd` option. For non-commit
runs, restore only the listed iteration paths so earlier valid uncommitted
checkpoints survive. For `--allow-dirty` runs, preserve the captured initial
dirty state and restore only listed collector paths. The collector never resets,
cleans, restores, stashes, or deletes files automatically. Remove a lock only
after confirming it is stale; concurrent runs against the same progress file
are rejected.
