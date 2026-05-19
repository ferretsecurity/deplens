import json
from pathlib import Path

import pytest

from audit_harvest.storage import ArtifactStore
from audit_harvest.producers.repomap.producer import produce_repomap, _source_hash


FIXTURE_PYTHON_FLASK = Path("tests/fixtures/python_flask")


@pytest.mark.integration
def test_produce_repomap_returns_record(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    record = produce_repomap(FIXTURE_PYTHON_FLASK, store)

    assert record.name == "repomap"
    content = Path(record.path).read_text()
    data = json.loads(content)
    assert isinstance(data["repo_map"], str)
    assert len(data["repo_map"]) > 0


@pytest.mark.integration
def test_produce_repomap_budget_respected(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    record = produce_repomap(FIXTURE_PYTHON_FLASK, store, budget_tokens=200)

    content = Path(record.path).read_text()
    data = json.loads(content)
    repo_map = data["repo_map"]
    # Word count should be well under 600 for a 200-token budget
    assert len(repo_map.split()) < 600


@pytest.mark.integration
def test_produce_repomap_cache_hit(tmp_path):
    store = ArtifactStore(tmp_path / "store")

    record1 = produce_repomap(FIXTURE_PYTHON_FLASK, store)

    src_hash = _source_hash(FIXTURE_PYTHON_FLASK)
    assert store.is_fresh("repomap", src_hash), "Cache should be fresh after first call"

    record2 = produce_repomap(FIXTURE_PYTHON_FLASK, store)
    assert record1.content_hash == record2.content_hash, "Cache hit should return same content"
