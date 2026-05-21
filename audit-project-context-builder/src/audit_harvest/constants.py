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
