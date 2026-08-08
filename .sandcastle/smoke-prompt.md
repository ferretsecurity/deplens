# Sandcastle authentication smoke test

Validate this sandbox without modifying the repository or any remote state.

Run these commands and inspect their exit codes and output:

1. `codex login status`
2. `gh auth status`
3. `gh repo view --json nameWithOwner --jq .nameWithOwner`
4. `go version`
5. `git status --short`

Requirements:

- Codex must report that it is logged in using ChatGPT.
- GitHub CLI authentication and repository access must succeed.
- Go must report version 1.25.6.
- Do not edit files, create commits, push branches, or modify GitHub issues.

If every check succeeds, output `<smoke>PASS</smoke>`.
If any check fails, output `<smoke>FAIL</smoke>` and explain the failure.
Finish with `<promise>COMPLETE</promise>`.
