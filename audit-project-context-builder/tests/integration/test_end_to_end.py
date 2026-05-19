"""End-to-end test: run all Phase 1 producers against fixtures and verify outputs.

These tests require no external binaries (cdxgen, osv-scanner) -- only the pure-Python
producers (A1, A2, A4, A7, A14). Mark them @pytest.mark.integration for documentation
purposes but they run without real external tools.
"""
import json
from pathlib import Path

import pytest

from audit_harvest.storage import ArtifactStore
from audit_harvest.producers.entry_points import produce_entry_points
from audit_harvest.producers.gate_matrix import produce_gate_matrix
from audit_harvest.producers.index import produce_index
from audit_harvest.producers.repomap.producer import produce_repomap

_FIXTURES_DIR = Path(__file__).parent.parent / "fixtures"
FIXTURES = [
    _FIXTURES_DIR / "go_simple",
    _FIXTURES_DIR / "python_flask",
    _FIXTURES_DIR / "js_express",
    _FIXTURES_DIR / "java_spring",
]


@pytest.mark.integration
@pytest.mark.parametrize("fixture", FIXTURES, ids=[f.name for f in FIXTURES])
def test_entry_points_and_gate_matrix_pipeline(fixture, tmp_path):
    """A2 + A4 pipeline: extract routes, then evaluate gate matrix."""
    store = ArtifactStore(tmp_path)
    ep_record = produce_entry_points(fixture, store)
    ep_data = json.loads(Path(ep_record.path).read_text())
    assert "entry_points" in ep_data

    gm = produce_gate_matrix(fixture, ep_data, [])
    assert len(gm["rules"]) == 13
    store.write("gate_matrix", json.dumps(gm).encode(), source_hash="test")

    index = produce_index(store, max_age_days=7)
    assert len(index["artifacts"]) == 2
    assert all(not a["stale"] for a in index["artifacts"])


@pytest.mark.integration
@pytest.mark.parametrize("fixture", FIXTURES, ids=[f.name for f in FIXTURES])
def test_repomap_produces_symbols(fixture, tmp_path):
    """A7: repomap extracts at least some symbols from each fixture."""
    store = ArtifactStore(tmp_path)
    record = produce_repomap(fixture, store)
    data = json.loads(Path(record.path).read_text())
    assert "symbols" in data
    # All fixtures have at least one function/class definition
    assert data["meta"]["total_symbols"] > 0


@pytest.mark.integration
@pytest.mark.parametrize("fixture", FIXTURES, ids=[f.name for f in FIXTURES])
def test_second_run_uses_cache(fixture, tmp_path):
    """Running the same producer twice returns the same ArtifactRecord (cache hit)."""
    store = ArtifactStore(tmp_path)
    record1 = produce_entry_points(fixture, store)
    record2 = produce_entry_points(fixture, store)
    assert record1.content_hash == record2.content_hash
    assert record1.path == record2.path
