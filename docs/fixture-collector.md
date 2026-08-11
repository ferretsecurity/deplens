# Fixture collector operator guide

`fixturecollectorloop` is a local-only development command for building the
dependency-source corpus. Go owns every remote request and every filesystem
effect. The isolated model receives a bounded packet of qualified source data
and returns only stable candidate IDs and rationales. It cannot author paths,
source bytes, provenance facts, queries, or workspace changes.

The command never executes upstream code, installs dependencies, clones
repositories, creates branches, fetches, pulls, pushes, or opens pull
requests. Terms follow the [fixture collection glossary](glossary.md#fixture-collection).

## Initialize, limits, and states

Run initialization from the checkout root. It writes only strict version-2
progress: reviewed query plans, resource limits, detector state, and no raw
upstream content.

```bash
go run ./cmd/fixturecollectorloop initialize-progress \
  --progress .deplens/fixture-collection.yaml \
  --detector example-detector,another-detector --target 3
```

Review progress before running. Its defaults bound eight queries, ten result
pages, 40 candidate inspections, 16 MiB of decoded remote responses, 2 MiB per
source, an approximately 50,000-token selection packet, two selector calls,
and seven valid iterations. The packet figure is a conservative local
approximation, not exact counting of the complete model request.

States are `pending`, `in-progress`, `complete`, `needs-query-review`,
`needs-content-review`, and `needs-collection-review`. Review-needed states are
reported but do not make an otherwise valid run fail. A detector completes with
three accepted examples (the reviewed target may be three through five).
Initialization marks detectors without a precise plan `needs-query-review`.

Old progress and provenance version 1 are never migrated or accepted. Start a
fresh collection and handle old artifacts manually.

## Acquisition and selection

`run` uses GitHub code search first and uses web search only as a Go-owned hint
fallback. Every hinted candidate is reverified through GitHub. Go resolves one
default-branch commit, fetches exact source and license evidence at that
immutable commit, qualifies the candidate, and writes provenance version 2
only after selection and final validation.

Qualification rejects non-public, forked, template, fixture/demo/training
repositories; unsafe, oversized, symlink, LFS, secret, or personal-data
sources; and missing, ambiguous, or unapproved licenses. Source and license
bytes are treated as data, not instructions. Go alone derives the stable
directory, original path, hashes, URLs, license evidence, and provenance. The
model's rationale is the sole model-authored provenance field.

The selector requires an installed Codex CLI with the complete isolation feature
set and an existing Codex OAuth login. Any Codex CLI version number is eligible;
preflight fails closed when a required feature is unavailable, and the observed
version is included in the selector-state fingerprint. GitHub credentials come
from standard token environment variables or an existing `gh` login and remain
in Go memory; they are never passed to Codex.
The packet is supplied only on stdin. Configurable tool families are disabled;
the residual-tool sandbox has an empty network policy and only a disposable
read-only work directory. This is a residual-tool sandbox guarantee, not a
claim that the tool list is empty. Stdout and stderr are bounded in memory;
candidate packets, raw events, source bytes, and license bytes are not retained
in logs or progress.

## Run, stop, and resume

```bash
go run ./cmd/fixturecollectorloop run --single \
  --progress .deplens/fixture-collection.yaml
go run ./cmd/fixturecollectorloop run --detector example-detector \
  --progress .deplens/fixture-collection.yaml
go run ./cmd/fixturecollectorloop run --duration 8h \
  --progress .deplens/fixture-collection.yaml
```

`--single` runs one selected detector iteration. A duration is a soft limit:
an active iteration may validate and checkpoint, but no subsequent iteration
starts. Re-run the same command to resume a valid checkpoint. The first
`SIGINT`/`SIGTERM` requests that behavior; a second cancels active research and
records recovery-required state. Exit 0 is a valid completion or stop; exit 1
is an operational, integrity, lock, Git, selector, or recovery failure.

## Git, recovery, and commits

The collector requires a Git checkout and refuses any non-ignored dirty state
unless `--allow-dirty` is explicit. The override preserves the starting state;
it does not bypass validation or recovery. A progress lock prevents concurrent
runs. Interrupted or invalid work records the detector, iteration, checkpoint,
changed paths, and sanitized failure diagnostic in progress. Resolve only the
listed collector paths, then rerun with `--allow-dirty`; the collector never
resets, cleans, restores, stashes, or deletes files automatically.

Without `--commit`, a valid checkpoint stays as working-tree state. `--commit`
creates at most one local commit after each valid iteration, containing exactly
the validated corpus changes and progress checkpoint. It requires Git author
identity and never pushes. If commit creation fails, the valid checkpoint stays
uncommitted for manual review.

## Diagnostics and manual handoff

Progress history is append-only, structured, and content-free: it records
queries, candidate dispositions, packet omissions, decision fingerprints,
accepted IDs, and stable reason codes. A summary names review-needed detector
IDs for manual fulfillment. Investigate provider authentication, budgets,
qualification reasons, selector failures, recovery details, and Git status
locally; do not retain upstream payloads as diagnostics.
