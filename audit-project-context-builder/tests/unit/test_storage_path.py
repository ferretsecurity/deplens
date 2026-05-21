"""Tests for _get_store path computation in run_tools.py."""
from pathlib import Path

from audit_harvest.mcp_server.tools.run_tools import _get_store
from audit_harvest.storage import ArtifactStore


def test_get_store_default_path_is_in_repo(tmp_path):
    """Default path must be <repo>/.audit/harvest/."""
    store, repo = _get_store(str(tmp_path))
    expected = tmp_path / ".audit" / "harvest"
    assert store.root == expected


def test_get_store_respects_env_override(tmp_path, monkeypatch):
    """AUDIT_HARVEST_DIR env var overrides the default path."""
    override = tmp_path / "custom-override"
    monkeypatch.setenv("AUDIT_HARVEST_DIR", str(override))
    store, repo = _get_store(str(tmp_path))
    assert store.root == override


def test_get_store_does_not_use_cache_dir(tmp_path):
    """Path must NOT contain .cache or audit-agent."""
    store, repo = _get_store(str(tmp_path))
    assert ".cache" not in str(store.root), f"Path uses .cache: {store.root}"
    assert "audit-agent" not in str(store.root), f"Path uses audit-agent: {store.root}"


def test_get_store_returns_correct_types(tmp_path):
    """Return types must be (ArtifactStore, Path)."""
    store, repo = _get_store(str(tmp_path))
    assert isinstance(store, ArtifactStore)
    assert isinstance(repo, Path)


def test_get_store_returns_resolved_repo_path(tmp_path):
    """Second return value must be the resolved absolute repo path."""
    store, repo = _get_store(str(tmp_path))
    assert repo == tmp_path.resolve()


def test_get_store_env_override_clears_on_unset(tmp_path, monkeypatch):
    """Without AUDIT_HARVEST_DIR set, default path is used regardless of prior env state."""
    monkeypatch.delenv("AUDIT_HARVEST_DIR", raising=False)
    store, repo = _get_store(str(tmp_path))
    assert store.root == tmp_path / ".audit" / "harvest"
