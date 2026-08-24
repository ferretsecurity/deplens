# Analyzer implementation loop

`cmd/analyzerloop` implements semantic analyzers from the verified fixture corpus without adding the original corpus sources to this repository.

## Workflow

Run `analyzerloop plan --corpus ../deplens-fixture-corpus` on a clean dedicated branch. Planning reads the corpus verification ledger and creates `.deplens/analyzer-implementation.yaml` only when it does not exist. An item is eligible only if its result is `OK`, it has exactly three candidates, all candidates are `valid`, the candidate files exist, and their SHA-256 hashes match. The planner also checks the frozen DepLens commit and default-rules hash. Items are sorted by detector ID and start in `pending` state.

Review and commit that ledger before `analyzerloop run`. The run command requires clean DepLens and corpus worktrees, a non-default branch, and configured Git identity. It rejects a changed commit or rules hash.

Each work item has two independent stages:

1. The implementer inspects all three originals, writes the semantic analyzer, tests, documentation, and exactly three minimized synthetic fixtures.
2. A fresh verifier independently checks the same originals and fixes gaps.

Only a successful verifier marks an item `completed`. An accepted implementer checkpoint is `in_progress`, so a later run resumes with verification. Each role receives at most two attempts per run. Rejected attempts remain in the runtime journal and do not change the ledger.

## Safety boundaries

Normal runs use a detached Git worktree for every attempt. The Harness formats changed Go files, runs focused analyzer tests, runs the full suite and `go vet` for verifier attempts, and checks that all corpus candidates are recognized by the intended detector. It rejects patches outside `internal/analyze/`, `testdata/`, `README.md`, and `DEPENDENCY_COVERAGE.md`, as well as changes to the loop, ledger, corpus copies, or Go modules.

Accepted patches are applied to the target branch and committed with `Ralph-Run`, `Ralph-Attempt`, `Ralph-Work-Item`, and `Ralph-Outcome` trailers. `--no-commit` is available for an explicitly uncommitted trial; it keeps both fresh sessions in the target worktree so the verifier can see the implementer changes.

The Codex adapter is pinned to `gpt-5.6-terra`, `high` reasoning effort, `workspace-write`, and no approvals. Its final result is a small JSON marker containing a summary and the three fixture paths. Raw attempt output and content-free JSONL outcome records live below `.ralph/runs/` with private permissions. They are never committed and are not automatically removed.

## Selection and recovery

`--select 1,3...7,12` selects ascending unique item numbers. Without it, `run` chooses the first unfinished item; `--once` limits an explicit selection to its first unfinished item. A failed validation consumes an attempt. Authentication, preflight, ledger, Git transaction, or journal errors stop the run without automatic repair. Inspect the runtime journal and working tree before retrying.
