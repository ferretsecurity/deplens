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
