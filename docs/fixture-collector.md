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
pages, 16 MiB of decoded remote responses, 2 MiB per source, an approximately
100,000-token selection packet, two selector calls, and seven valid iterations.
The normal candidate-inspection target is 100. One quarter of the
decoded-response allowance is reserved for search and three quarters for
detailed acquisition, so result pages cannot consume source and license
capacity. The packet figure is a conservative local approximation, not exact
counting of the complete model request. Initialization stores these reviewed
limits in the progress file; changing a software default does not silently
rewrite an existing collection's limits.

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

Provider search qualifiers are discovery hints and may return loose path
matches. Go applies every detector's reviewed selector locally before a search
hit consumes the candidate-inspection budget or triggers a repository-specific
request. This keeps the same bounded acquisition behavior across all detector
query shapes without trusting provider search semantics. Filtered search noise
is checkpointed as bounded reason counts; it is not recorded as one candidate
rejection per provider hit.

Qualification normally inspects 100 candidates. If fewer than five qualify,
it continues through later result pages until five qualify or a hard query,
result-page, or decoded-byte bound stops the work. Once both the
inspection target and five qualified candidates are reached, no unused result
pages are downloaded. Production source inspection obtains the file
type and exact bytes from one GitHub Contents response. Search and acquisition
charge independent portions of the reviewed aggregate byte allowance.

The selector requires an installed Codex CLI with the complete isolation feature
set and an existing Codex OAuth login. Any Codex CLI version number is eligible;
preflight fails closed when a required feature is unavailable, and the observed
version is included in the selector-state fingerprint. Selection is pinned to
`gpt-5.6-terra` with `medium` reasoning rather than inheriting user model
defaults. GitHub credentials come
from standard token environment variables or an existing `gh` login and remain
in Go memory; they are never passed to Codex.

The packet is supplied only on stdin. Configurable tool families are disabled;
the residual-tool sandbox has an empty network policy and only a disposable
read-only work directory. This is a residual-tool sandbox guarantee, not a
claim that the tool list is empty. Stdout and stderr are bounded in memory;
candidate packets, raw events, source bytes, and license bytes are not retained
in logs or progress.

Codex must select exactly three candidates as one versatile set. The set must
include the best representative of common real-world usage, while the other
choices add useful structural variation or edge cases. If fewer than three
complete candidates fit the approximately 100,000-token packet, Go does not
invoke Codex for that iteration.

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

`run` presents top-level run information without indentation and encloses every
detector iteration in a `┌─`/`│`/`└─` block. Search, qualification, selection,
checkpoint, and selected-candidate paths are indented sections inside that
block. After every finished detector it prints detectors attempted, finished,
and remaining in this run, the iteration count, elapsed time, average time per
finished detector, and an estimated time for the remaining detectors. The same
figures appear in a structured run summary on normal completion or a soft stop.
The estimate is `n/a` until one detector finishes.

While acquisition is active, the search section reports each provider result
page and the qualification section reports approximately ten evenly spaced
tallies containing inspected, the normal inspection target, qualified progress
toward five, rejected, and cheaply filtered counts. Search lines show the
search-byte allowance; qualification lines show the acquisition-byte allowance,
including downloaded and remaining amounts. The selection section brackets the
isolated model call. After a valid accepted checkpoint, the selected-candidates
section prints absolute local paths for every selected source and its provenance
file so terminals can render them as links. Progress is content-free: it never
prints upstream source or license bytes.

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
