# Decision: Artifact Storage Location

**Choice:** In-repo — `<repo>/.audit/harvest/<git-sha>/`

Override via env var: `AUDIT_HARVEST_DIR=<path>`

**Rationale:**
- Artifacts travel with the repo they describe.
- Path is keyed on git SHA, making the cache naturally content-addressed.
- Dirty working tree uses `dirty-<unix-timestamp>` and does NOT update the `current` symlink.
- The `current` symlink at `<repo>/.audit/harvest/current` gives downstream stages a stable pointer.
