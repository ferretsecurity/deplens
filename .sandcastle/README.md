# Sandcastle setup

This workflow uses the Codex CLI's ChatGPT OAuth session and the GitHub CLI's
existing WSL login. No OpenAI API key or GitHub token is copied into an env
file.

## One-time setup

Run these commands from WSL:

```bash
codex login
gh auth login
npm install
npm run sandcastle:build
npm run sandcastle:smoke
```

The Docker sandbox mounts:

- `$CODEX_HOME/auth.json` (falling back to `~/.codex/auth.json`) read-only at
  `/home/agent/host-codex/auth.json`, then copies it to the writable
  `/home/agent/.codex/auth.json` before an agent starts. This lets Codex persist
  sandbox-local OAuth refreshes without mutating or trying to atomically
  replace a bind-mounted host file.
- `~/.config/gh` read-only at `/home/agent/.config/gh`, allowing `gh` to reuse
  the host login without exposing it through `.sandcastle/.env`.

## Run the sequential reviewer

```bash
npm run sandcastle
```

Each iteration gives a Codex implementer and a Codex reviewer the same Docker
sandbox and named Git branch. Both use `gpt-5.6-terra` with medium reasoning.

This workflow leaves every reviewed ticket on its own local branch. It does not
merge those branches into the branch from which the command was started.

## Run the feature pipeline

```bash
npm run sandcastle:feature
```

Run this command with the feature branch that should receive completed tickets
checked out. The pipeline refuses to run from a detached `HEAD` or from the
repository's default branch. It otherwise preserves the existing host-worktree
behavior: local changes are the user's responsibility, and the checked-out
branch must not be switched or moved while an iteration is running.

Before every iteration, the orchestrator fetches the open issues carrying the
`Sandcastle` label, filters out issues with open blockers, and selects the
lowest-numbered eligible issue. Native GitHub issue dependencies are preferred;
the documented `Blocked by` body convention is used when native dependency
metadata is unavailable.

Each selected issue runs through these stages:

1. An implementer works on one timestamped ticket branch and commits the change.
2. A reviewer inspects the same branch and may commit corrections.
3. The orchestrator merges the reviewed branch into the checked-out feature
   branch with `git merge --no-ff`.
4. The orchestrator runs `go test ./...` and `go vet ./...` on the accumulated
   feature branch.
5. Only after both checks pass, the orchestrator closes the issue and deletes
   the merged local ticket branch.

The pipeline processes at most ten issues per invocation and refreshes the issue
queue after every successful integration so newly unblocked work can be picked
up. It makes no pushes and opens no pull requests.

If a merge conflicts, the orchestrator aborts the merge, comments on the issue,
retains the ticket branch, and stops. If post-merge verification fails, the
merge commit remains on the feature branch for manual recovery; the issue stays
open, the ticket branch is retained, and the pipeline stops.
