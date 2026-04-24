# AGENTS.md

## Repository Instructions

- When adding a new detector to code, add information about it to `README.md`.
- When adding a new default rule, add an example in the `testdata` folder.
- When a change affects user-visible behavior and the agent creates a PR, add a concrete example that shows the change. For a new CLI argument, include how to call the CLI with the new argument, plus example output without it and with it. For a new default rule, include an example of files that were not identified before and are identified after the change. Apply the same standard to other user-facing behavior changes: show the old behavior and the new behavior with a concrete example.
- `testdata` is not a replacement for proper unit tests, however it can be used as an addition.
