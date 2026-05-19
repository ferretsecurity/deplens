import json
import time
from pathlib import Path

import pytest

from audit_harvest.storage import ArtifactStore
from audit_harvest.producers.index import produce_index


def test_index_lists_all_artifacts(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"content_a", source_hash="h1")
    store.write("entry_points", b"content_b", source_hash="h2")

    result = produce_index(store, max_age_days=7)
    names = {a["name"] for a in result["artifacts"]}
    assert names == {"repo_profile", "entry_points"}


def test_stale_artifact_is_flagged(tmp_path):
    store = ArtifactStore(tmp_path)
    record = store.write("repo_profile", b"old", source_hash="h1")

    # Backdate last_built_at by rewriting meta.json
    meta_path = Path(record.path).parent / "meta.json"
    data = json.loads(meta_path.read_text())
    data["last_built_at"] = time.time() - (8 * 86400)  # 8 days ago
    meta_path.write_text(json.dumps(data))

    result = produce_index(store, max_age_days=7)
    artifact = result["artifacts"][0]
    assert artifact["stale"] is True


def test_fresh_artifact_is_not_stale(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"fresh", source_hash="h1")

    result = produce_index(store, max_age_days=7)
    assert result["artifacts"][0]["stale"] is False


def test_empty_store_produces_empty_index(tmp_path):
    store = ArtifactStore(tmp_path)
    result = produce_index(store, max_age_days=7)
    assert result["artifacts"] == []
    assert "built_at" in result


def test_index_schema_valid(tmp_path):
    schema_path = Path(__file__).parent.parent.parent / "interfaces" / "harvest-outputs.schema.json"
    full_schema = json.loads(schema_path.read_text())
    defs = full_schema.get("$defs", full_schema.get("definitions", {}))
    index_schema = defs.get("index_artifact")
    if index_schema is None:
        pytest.skip("index_artifact not defined in schema yet")
    index_schema = {**index_schema, "$defs": defs}

    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"x", source_hash="h1")
    result = produce_index(store, max_age_days=7)
    import jsonschema
    jsonschema.validate(result, index_schema)
