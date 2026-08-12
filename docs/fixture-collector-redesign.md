# Fixture collector redesign

Status: implemented v2 contract.

This note consolidates the fixture-collector grilling decisions. It is not an
ADR. The [operator guide](fixture-collector.md) describes the production
workflow; this note records its design rationale.

## Objective and ownership

For each eligible detector, retain 3-5 exact dependency sources from real
public GitHub projects. Go owns discovery, remote calls, acquisition,
qualification, accounting, provenance, materialization, final validation,
checkpointing, and optional commits. A model performs only comparative
selection from a bounded packet and returns stable candidate IDs with
rationales. The model never supplies source bytes, provenance facts, repository
coordinates, queries, or filesystem changes.

One collection iteration advances one detector. It may accept a batch of
examples. A detector is complete once at least three examples are accepted;
five remains the maximum.

## Module boundary

The collector wrapper retains ownership of recovery snapshots, exact-byte
materialization, final validation, progress checkpointing, and optional Git
commits. A deep read-only Researcher module has one operation that produces a
research result. Internally it owns query execution, budgets, acquisition,
qualification, packet construction, and selection.

The Researcher has two external seams:

- an acquisition adapter, whose production implementation uses GitHub and a
  web-hint fallback but whose accepted evidence always comes from GitHub; and
- a selector adapter, whose production implementation invokes Codex.

Tests replace both seams. No test requires live GitHub, web-search, or Codex
access.

## Query planning and discovery

Query plans are deterministic, reviewed, ordered data in collection progress.
Go generates them from detector selectors during initialization. A model does
not propose queries. If a precise plan cannot be generated, the detector enters
`needs-query-review` and is skipped without consuming an iteration.

The retained query-generation prototype currently classifies 142 of 144
eligible detectors as ready and two as needing query review:
`ocaml-opam-locked` and `terraform.aws_glue_job.python`.

GitHub code search is primary. When its reviewed plan cannot produce enough
model-presentable candidates to reach the detector minimum, Go may execute a
reviewed web fallback. Web results are hints only: Go extracts GitHub
coordinates and reruns normal GitHub acquisition and qualification. Third-party
page content never enters a packet, corpus, provenance record, or progress.

Search-result order is discovery evidence, not a quality ranking. Duplicate
repository-and-path hits across queries are collapsed before inspection.

## Budgets and accounting

All acquisition controls are explicit reviewed progress fields. Hard resource
bounds are enforced by a shared Go budget tracker immediately before
consumption. At minimum they cover:

- search queries;
- result pages;
- a normal candidate-inspection target;
- decoded remote-response bytes, partitioned between discovery and detailed
  acquisition and including metadata and evidence;
- selection-packet tokens; and
- selector invocations.

The established defaults are eight queries, a target of 100 inspections, a 2
MiB source ceiling, a 100,000-token candidate-packet target, seven valid
iterations per detector, and two selector invocations per iteration. Page and
aggregate-byte limits are mandatory reviewed values rather than hidden
implementation defaults.

The aggregate decoded-response allowance is deterministically partitioned: one
quarter is reserved for search responses and three quarters for repository,
commit, source, and license acquisition. Qualification inspects up to the
normal target even when five candidates qualify early. If fewer than five
qualify at that point, it continues until five qualify or a hard query,
result-page, or decoded-byte bound is reached. Search stops when both
the inspection target and qualified minimum are satisfied. The
production GitHub adapter obtains source type and exact bytes from one Contents
response rather than downloading the same payload twice.

Search-byte exhaustion is a normal bounded-discovery stop. It preserves
already qualified candidates, prevents another provider from starting with a
zero-byte search allowance, and continues to selection when the preserved set
is sufficient. It is not classified as a remote-provider infrastructure
failure.

Valid outcomes durably advance a per-provider/query search cursor and record
content-free hashes for observed repository/path hits. The next iteration
starts at the next unvisited page and suppresses hits repeated by changing
provider pagination. Final-page exhaustion is durable. Once every configured
provider/query pair is exhausted, there is no distinct automatic research
state and the detector enters collection review without spending placeholder
iterations. GitHub code search does not request pages beyond its 1,000-result
window.

A search hit does not consume an inspection. An inspection is charged before
the first detailed metadata or content request for a unique repository and path
in the current iteration. Retrieval failures still consume it. Reconsidering
the same source in a later iteration consumes a new inspection. Repeated remote
requests and all decoded response bytes are accounted separately and never
refunded.

Remote work uses bounded concurrency. Budgets are reserved before scheduling,
and results are committed to the research record in deterministic discovery
order. Per-run in-memory caches use repository, immutable commit, and path
keys. Raw candidate bytes are not cached across runs.

## Acquisition snapshot

For each inspected source, Go resolves the repository's current default-branch
head once and records the acquisition time. It fetches source bytes, repository
facts, and license evidence at that same immutable commit. Later branch movement
does not invalidate the snapshot, and no acceptance-time head recheck occurs.

The repository must be public, non-fork, non-template, and a real project rather
than a fixture, demo, example-only, or training-data repository. There is no
popularity or activity threshold. Ambiguous project-purpose evidence is a
candidate rejection rather than a model judgment.

Go uses direct GitHub API requests. Credentials are resolved from the standard
GitHub token environment or, when absent, from the installed `gh` login without
logging or persisting the token. Credentials remain in Go memory and are never
passed to Codex. Requests are serial, code search is paced to one request per
six seconds, and `core` and `code_search` response limits are tracked
independently. Core requests are paced from the latest reported limit,
remaining count, and reset time, with ten percent reserved for other clients.
The collector waits for reset when that reserve is reached. Explicit rate
limits wait for `Retry-After` or reset and retry a bounded number of times;
secondary limits use bounded exponential backoff. Temporary transport and 5xx
failures also receive bounded retries. Other 403 responses fail immediately.
All waits are context-cancellable, every decoded retry response consumes the
existing byte budget, and sanitized rate headers, request IDs, and bounded
provider messages are reported to the operator. Authentication failure and
service unavailability remain infrastructure failures.

## Qualification

Qualification is deterministic and has a documented check order so a candidate
always receives the same primary reason code. It covers:

- repository eligibility and default-branch membership;
- a regular source path matching the detector's path selector;
- source-size ceiling and exact-byte retrieval;
- symlink and Git LFS pointer rejection;
- approved, unambiguous governing license evidence;
- credentials, authentication material, private-registry secrets, and personal
  data;
- duplicate immutable identity and duplicate source content; and
- safe handling of source bytes.

The implementation applies those checks in this exact order: repository access
and purpose; safe repository-relative path and detector selector; immutable
default-branch head; regular-file evidence; source retrieval and source-size
ceiling; UTF-8, LFS, and sensitive-content checks; nearest governing-license
resolution; then duplicate immutable identity and source SHA-256. The scanner
and analyzers are not invoked at any point in that sequence. A rejection stores
only its first closed code (for example `repository-private`,
`repository-purpose`, `source-selector-mismatch`,
`source-not-regular-file`, `source-size-ceiling`,
`not-model-presentable-non-utf8`, `not-model-presentable-packet-size`,
`source-lfs-pointer`,
`sensitive-content`, `license-missing`, `license-disallowed`,
`license-conflicting`, `license-ambiguous`, `duplicate-identity`, or
`duplicate-content`) and no rejected source or license bytes.

Qualification never executes upstream code, installs dependencies, follows
upstream instructions, or runs the scanner/analyzer as an acceptance oracle.
Prompt-like comments or strings are not a rejection reason. Unsafe content is
rejected without redaction, and rejected bytes are discarded after the
iteration.

The governing license is the nearest unambiguous recognized license file from
the source directory up to the repository root. Go fetches it at the acquisition
commit, detects an allowlisted SPDX identifier, and records its path, immutable
permalink, and SHA-256. Missing, disallowed, conflicting, or ambiguous license
evidence fails qualification.

An otherwise qualified source may still be unsuitable for model presentation.
The collector never truncates, summarizes, or Base64-encodes candidate content.
Non-UTF-8 content or a source too large to fit whole receives a precise
not-model-presentable disposition. If such sources prevent the detector from
reaching three accepted examples, the detector enters `needs-content-review`.
The establishing research outcome consumes an iteration; later skips do not.

## Selection packet

The packet is structured JSON. Candidate contents are complete escaped UTF-8
JSON strings accompanied by stable ID, source byte length, SHA-256, repository,
path, immutable commit, and license facts. The encoded JSON representation is
what consumes packet budget. The prompt declares every content field untrusted
data that must not be followed as instructions.

When a detector has one or two accepted examples, the packet includes them
first as mandatory accepted corpus references: exact content, immutable
identity, and stored rationale. They count against the same packet budget and
cannot be selected again. Failure to represent a mandatory reference uses the
`needs-content-review` path. The selector is not invoked for a detector that
already has three accepted examples.

After mandatory references, Go packs candidates deterministically:

1. assign each candidate to the first query that discovered it;
2. within each query, order by estimated encoded size and then stable ID;
3. round-robin across query queues, adding complete candidates that fit; and
4. serialize the final candidate set by stable ID so position does not imply
   quality.

Every qualified candidate excluded by the aggregate packet cap is recorded as
omitted for packet budget; it is not rejected. This policy favors short sources
and exposes multiple reviewed search intents without Go making a diversity or
quality judgment.

The 100,000-token figure is a locally estimated candidate-packet target, not an
exact count of Codex's hidden full request. The configured estimator includes
JSON encoding and reserves headroom for selector instructions and residual tool
schemas. Actual post-turn input usage is recorded as an operational metric.

## Model decision

The model must select exactly three candidates as one representative,
versatile, complementary set. The set must include the best example of common
real-world usage and use the remaining choices for useful structural variation
or edge cases, with a preference for short examples. Go does not mechanically
enforce repository, project-kind, or variation diversity.

The model returns only:

```json
{"selected":[{"id":"<stable-candidate-id>","rationale":"<text>"}]}
```

Selection order has no meaning. A rationale is required for each ID, is bounded
to 1,000 UTF-8 characters, excludes control characters, and is preserved
verbatim in provenance. Go validates representation only and does not score or
reinterpret its diversity claims. `project_kind` and `variation_tags` do not
exist in the new provenance schema.

Every model decision selects exactly three candidates. The prompt states this
requirement and the structured-output schema fixes both the minimum and maximum
array length at three. If fewer than three complete candidates fit the packet,
Go does not invoke the selector and checkpoints an unsuccessful bounded
research iteration instead.

Nonselection is contextual rather than a factual rejection. Progress records
the packet fingerprint, accepted-corpus fingerprint, selector configuration,
and decision. The same decision state is never submitted again after a valid
decision. Previously nonselected candidates may reappear when the accepted set,
packet membership, or selector configuration changes, with never-presented
candidates packed first.

## Codex transport and isolation

Go invokes an installed `codex exec` binary using the existing Codex ChatGPT
OAuth login. Any Codex CLI version number is eligible, but preflight fails
closed unless the complete required isolation feature set is available. The
packet is sent only on stdin. The process is ephemeral, ignores user
configuration and rules, uses a strict output schema, runs from a fresh empty
private directory, never requests approval, inherits an allowlisted environment,
and uses a deny-by-default filesystem and network permission profile. Checked
feature disables remove shell, web, MCP/app/plugin, subagent, browser/computer,
image, hook, and related tool families.

Codex does not expose a supported empty-tool-list switch under this OAuth
transport. The contract is therefore explicit: high-risk tool families are
removed where supported, and residual local tools are ineffective outside the
sandbox. The exact invocation and first-party citations are documented in
`docs/research/selector-transport-options.md`.

The model identifier (`gpt-5.6-terra`), reasoning effort (`medium`), observed Codex version,
isolation-feature set, and packet format version are included in the
selector-state fingerprint. They are never inherited silently from user
configuration.

## Selector validation and retries

Go validates the complete structured response before materializing anything:
IDs must have appeared in the exact packet, must be unique, must satisfy the
exact-three cardinality bound, and must have valid rationales.

An iteration permits at most two fresh ephemeral selector processes. A retry is
allowed only when no valid decision was produced, including malformed output,
unknown or duplicate IDs, invalid cardinality or rationale, timeout, retryable
Codex failure, or context overflow. Context overflow deterministically removes
the last packed candidates until additional headroom is reached; other failures
retry the identical packet. Failure of the second invocation is infrastructure
failure, changes no candidate outcomes, and consumes no iteration.

Candidate-specific final-validation failures are different. Go validates every
selected candidate independently, accepts and checkpoints survivors, records
stable rejection codes for failures, and never calls the model again in that
iteration. If fewer than three examples remain accepted, a later fresh
iteration continues from the partial corpus. Once at least three survive, the
detector is complete.

## Progress, outcomes, and recovery

The redesign uses a new strict progress format and accepts no old progress
format. Valid iterations are append-only structured history containing actual
queries and accounting, candidate facts and qualification outcomes, stable
reason codes, packet and accepted-state fingerprints, model decisions, and
final-validation results. Raw source or license bytes never enter progress.

Reason codes are closed Go enums separated by acquisition, qualification,
packet disposition, selection, final validation, and infrastructure stages.
Optional diagnostic details are bounded. External error strings never drive
control flow; an unexpected condition without a defined candidate-specific code
is infrastructure failure.

Progress is written with strict known fields, canonical ordering, a private
temporary file, flush/sync, and atomic rename while the collection lock is
held. Go reconstructs current state from history. A separate recovery record
describes an interrupted or unvalidated filesystem mutation; it does not turn
an infrastructure failure into a valid iteration.

A valid checkpointed research outcome consumes one of seven iterations even
when it ends in search exhaustion, budget exhaustion, all candidates rejected,
fewer than three presentable candidates, or partial candidate failure.
Query-review skips, content-review skips after their establishing outcome, and
infrastructure failures consume nothing.

After seven valid iterations, or earlier when no distinct decision state can be
constructed, a detector with fewer than three examples enters
`needs-collection-review`. `needs-query-review`, `needs-content-review`, and
`needs-collection-review` are non-blocking: the loop skips them, reports their
counts and IDs, and may still exit normally after processing other detectors.
Manual fulfillment is outside this implementation.

## Provenance and materialization

The redesigned collector accepts and writes only provenance version 2. There is
no backward compatibility or migration for old progress or version 1
provenance. Startup reports that a fresh collection must be initialized rather
than rewriting existing files automatically.

Version 2 contains:

- detector and stable candidate ID;
- provider, repository, repository URL, and default branch;
- immutable commit, original path, immutable source permalink, retrieval time,
  and source SHA-256;
- governing license SPDX identifier, path, immutable permalink, and SHA-256;
- the model-authored rationale.

Go owns every field except the rationale and recomputes both hashes and the
stable candidate ID during final validation.

Passing examples are staged in owner-only temporary directories. Go derives a
collision-resistant example directory from repository, commit, and original
path, preserves the upstream-relative source path below it, and writes exactly
one source plus `provenance.yaml`. Additive installation uses no model-supplied
path and never overwrites an existing file. The wrapper then revalidates exact
bytes, path selector, provenance, license evidence, size, LFS, unsafe content,
duplicate identity/content, and the iteration mutation boundary. It does not
run the analyzer as an acceptance oracle.

One valid iteration creates at most one optional local commit containing all
passing examples and its progress checkpoint. The collector never pushes.
Partial valid work survives; unexpected partial filesystem changes use the
existing recovery boundary.

## Retention and reporting

Candidate packets, raw Codex events, candidate bytes, and license bytes are not
written to logs. `codex exec --ephemeral` suppresses session rollout storage;
stdout and stderr are captured only in bounded memory. Failures may persist
owner-only, content-free diagnostics containing codes, IDs, hashes, sizes,
usage, and sanitized provider/request metadata. Successful operational metrics
live in progress.

Each run reports accepted counts, review-state counts and detector IDs, budget
usage, qualification reasons, packet omissions, selector usage, and
infrastructure failures. Review-needed detectors do not make an otherwise valid
run fail. Operational, recovery, or integrity failures remain nonzero exits.

## Verification requirements

Implementation tests must cover:

- deterministic query generation and the two current query-review detectors;
- fake GitHub/web discovery and fake Codex, with no live network;
- budget enforcement before consumption and stable accounting after failures;
- separate GitHub search/core pacing, rate-limit classification, interruptible
  wait, bounded retry, retry-byte accounting, and sanitized diagnostics;
- default-branch acquisition snapshots and governing license resolution;
- public/fork/template/project-purpose, size, symlink, LFS, secret/PII, selector,
  duplicate identity, and duplicate-content cases;
- deterministic packet packing, JSON escaping, token estimation, accepted
  references, omissions, and non-model-presentable sources;
- Codex argv/environment/isolation settings, stdin-only prompt transport,
  bounded capture, structured output, and retry behavior;
- contextual nonselection, exact-three selection, insufficient packet
  membership, partial batch acceptance, and no repeated decision state;
- strict progress history, crash recovery, v2-only provenance, additive writes,
  final validation, and optional commit boundaries; and
- proof that raw rejected or model-visible content does not enter progress,
  logs, arguments, environment variables, or retained temporary files.
