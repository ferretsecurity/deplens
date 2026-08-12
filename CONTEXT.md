# Dependency Source Analysis

Deplens identifies and analyzes dependency sources. Fixture acquisition supplies authentic dependency sources for the corpus without making unvalidated material part of that corpus.

## Fixture acquisition

**Collection progress**:
The strict new-format document containing reviewed plans and append-only valid iteration history. It records structured Go-observed accounting, candidate facts and outcomes, selection-state fingerprints, model decisions, and final validation results without storing upstream contents. Go reconstructs current state from this history.
_Avoid_: Agent report, mutable candidate list

**Provenance record**:
The strict version 2 record adjacent to a corpus example. The redesigned collector accepts no earlier provenance version. It records the detector and stable candidate identity, immutable repository/default-branch snapshot, source path/permalink/hash, hashed immutable license evidence, retrieval time, and selection rationale. Go derives and writes every field except the model-authored rationale.
_Avoid_: Model metadata, variation profile

**Source candidate**:
A dependency source from a real public project that is being considered for the corpus but has not passed final validation.
_Avoid_: Candidate, example

**Qualification**:
Deterministic checks performed before model selection to establish whether a source candidate is eligible to be considered. Qualification does not make the candidate a corpus example.
_Avoid_: Validation, withdrawal

**Qualified source candidate**:
A source candidate that passed qualification and is eligible for model selection. Omission from one selection packet does not change that eligibility.
_Avoid_: Valid candidate, corpus example

**Model-presentable source candidate**:
A qualified source candidate whose exact, complete source is safe UTF-8 and can fit by itself in the encoded selection-packet budget. Aggregate packet omission is separate and does not make a candidate non-presentable.
_Avoid_: Truncated candidate, summarized candidate

**Stable candidate ID**:
A deterministic identifier derived from a source candidate's provider, repository, immutable commit, and path. It identifies the upstream source independently of its content hash.
_Avoid_: Repository path, content hash

**Selection packet**:
The bounded set of accepted corpus references, qualified source candidates, and supporting evidence presented to one model session for comparison. Its token budget includes both references and candidates.
_Avoid_: Context, candidate dump

**Accepted corpus reference**:
An existing corpus example included with exact content, immutable identity, and stored selection rationale as mandatory comparison material in a resumed selection packet. It is marked as already accepted and cannot be selected again.
_Avoid_: Source candidate, selected source candidate

**Selected source candidate**:
A qualified source candidate chosen by the model for materialization and final validation. The model identifies it by stable candidate ID and supplies a selection rationale; ordering among selected candidates has no domain meaning. It does not become a corpus example until validation succeeds.
_Avoid_: Accepted example, corpus example

**Selection nonselection**:
The contextual outcome that the model did not choose a qualified source candidate from a particular selection packet against a particular accepted corpus state. It is not a deterministic rejection; the candidate may be reconsidered when the comparison state changes, but the same packet and corpus state are not submitted again.
_Avoid_: Rejection, disqualification

**Selection rationale**:
The model-authored, non-empty explanation of what a selected source candidate demonstrates and how it complements the final set. It is bounded to 1,000 UTF-8 characters, excludes control characters, is preserved verbatim in provenance, and is not semantically reinterpreted by deterministic validation.
_Avoid_: Variation tags, project kind

**Query plan**:
The reviewed, ordered search expressions that Go may execute to discover source candidates for one detector. Go may generate an initial plan from detector signals, but the model neither proposes nor executes its queries.
_Avoid_: Search prompt, model query

**Search cursor**:
The durable next unvisited result page, or exhausted status, for one provider and query. A valid iteration checkpoints cursor advancement so later iterations continue discovery instead of restarting at page one.
_Avoid_: Query history, candidate offset

**Search hit ID**:
A content-free hash of a provider search hit's normalized repository and path. It prevents a hit repeated by shifting provider pagination from being filtered or inspected again; it is not an immutable source-candidate identity.
_Avoid_: Stable candidate ID, source path

**Needs query review**:
A non-blocking detector state indicating that Go could not generate a sufficiently precise query plan for safe automatic discovery. The collection loop skips the detector without treating the state as a search failure.
_Avoid_: Blocked, no candidates

**Needs content review**:
A non-blocking detector state indicating that otherwise eligible sources required to reach the corpus minimum cannot be presented to the model as exact, complete, safe UTF-8 content within the selection-packet budget. The research outcome that establishes this state consumes an iteration; later collection runs skip the detector.
_Avoid_: Rejected candidate, infrastructure failure

**Needs collection review**:
A non-blocking detector state indicating that automatic research ended with fewer than three accepted examples after seven valid iterations or because no distinct selection state remains. Later runs skip the detector; manual fulfillment is outside the collector.
_Avoid_: Infrastructure failure, blocked run

**Collection accounting**:
The mandatory record of actual discovery, acquisition, qualification, and selection-packet activity. Accounting is produced by Go operations rather than model reports.
_Avoid_: Agent report

**Candidate inspection**:
The first detailed metadata or content request for one unique repository-and-path pair in one collection iteration. Duplicate search hits do not consume another inspection, but a failed request and deliberate reconsideration in a later iteration do.
_Avoid_: Search hit, accepted candidate

**Outcome reason code**:
A closed, stage-specific Go enum identifying a durable candidate or infrastructure outcome. Optional bounded diagnostic detail may accompany it, but external error text and free-form wording never determine control flow.
_Avoid_: Error-message matching, ad hoc rejection

**Collection budget**:
A configurable hard limit on resource-consuming collection activity, enforced by Go at the point of consumption. Queries, result pages, candidate inspections, downloaded bytes, selection-packet tokens, and selector invocations are budgeted; derived outcomes are only accounted for.
_Avoid_: Qualification limit, model limit

**License evidence**:
Proof that an allowlisted license governs a source candidate, consisting of a detected SPDX identifier and the governing license file fetched and hashed at the candidate's immutable commit. Missing, disallowed, conflicting, or ambiguous evidence fails qualification.
_Avoid_: Repository license label

**Acquisition snapshot**:
The source bytes and supporting evidence fetched consistently from the default-branch head resolved at a recorded acquisition time. Later movement of the branch does not invalidate this immutable snapshot.
_Avoid_: Latest revision, acceptance-time head

**Valid research outcome**:
A completed, checkpointed collection attempt that found, rejected, selected, or exhausted candidates within its collection budgets. It consumes one collection iteration even when it accepts no corpus example.
_Avoid_: Successful collection

**Collection infrastructure failure**:
A failure of authentication, a remote provider, the model protocol, materialization, or the collector itself that prevents a valid research outcome. It does not consume a collection iteration.
_Avoid_: Candidate rejection, budget exhaustion
