"""
Shared constants for audit-harvest producers and utilities.
Single source of truth -- do not duplicate these in individual modules.
"""

# All manifest and lockfile names used for cache-key hashing and repo
# profiling. Add new entries here; every other module imports from here.
MANIFEST_NAMES: tuple[str, ...] = (
    # Go
    "go.mod", "go.sum", "go.work",
    # Node
    "package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
    # Python
    "requirements.txt", "setup.py", "setup.cfg", "pyproject.toml",
    "Pipfile", "Pipfile.lock",
    # Java / Kotlin / Scala
    "pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle",
    # Rust
    "Cargo.toml", "Cargo.lock",
    # Ruby
    "Gemfile", "Gemfile.lock",
    # PHP
    "composer.json", "composer.lock",
)

# Subset of MANIFEST_NAMES that indicates a sub-package root when
# found in a non-root directory. Used by monorepo detection in A1.
# If a manifest type is added to MANIFEST_NAMES and should also
# trigger monorepo detection, add it here too.
MONOREPO_MANIFEST_NAMES: frozenset[str] = frozenset({
    "go.mod",
    "package.json",
    "pom.xml",
    "pyproject.toml",
    "Cargo.toml",
})

# Directories to exclude from all in-process repo walks.
# Used by producers, extractors, and any rglob-based scan.
SKIP_DIRS: frozenset[str] = frozenset({
    ".git", "node_modules", "vendor", "__pycache__",
    ".venv", "testdata", "site-packages", "target", "build",
    "dist", ".next", ".nuxt", ".cache",
})

# ripgrep --glob exclusion patterns equivalent to SKIP_DIRS.
# Pass as: for g in SKIP_GLOBS: cmd += ["--glob", g]
SKIP_GLOBS: tuple[str, ...] = (
    "!.git", "!vendor", "!node_modules", "!testdata",
    "!*.lock",
)
