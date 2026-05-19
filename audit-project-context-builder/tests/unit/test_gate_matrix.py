import json
from pathlib import Path
from unittest.mock import patch
import pytest
import jsonschema
from audit_harvest.producers.gate_matrix import produce_gate_matrix

_FIXTURES = Path(__file__).parent.parent / "fixtures"
FIXTURE_GO = _FIXTURES / "go_simple"
FIXTURE_PY = _FIXTURES / "python_flask"


def test_returns_13_cwe_classes():
    result = produce_gate_matrix(FIXTURE_GO, {"entry_points": []}, [])
    assert len(result["rules"]) == 13


def test_xss_not_applicable_when_no_web_framework(tmp_path):
    (tmp_path / "main.go").write_text("package main\nfunc main() {}")
    result = produce_gate_matrix(tmp_path, {"entry_points": []}, [])
    xss_rule = next(r for r in result["rules"] if r["cwe"] == "CWE-79")
    assert xss_rule["applicable"] is False
    assert len(xss_rule["evidence"]) >= 2


def test_sqli_needs_verification_with_driver_no_raw_sql():
    sbom_comps = [{"name": "sqlalchemy"}]
    result = produce_gate_matrix(FIXTURE_PY, {"entry_points": []}, sbom_comps)
    sqli = next(r for r in result["rules"] if r["cwe"] == "CWE-89")
    assert sqli["applicable"] == "needs_verification"


def test_llm_not_invoked_for_deterministic_rules():
    with patch("audit_harvest.producers.gate_matrix.LLMClient") as mock_llm:
        produce_gate_matrix(FIXTURE_GO, {"entry_points": []}, [])
    mock_llm.assert_not_called()


def test_output_matches_schema():
    schema_path = Path(__file__).parent.parent.parent / "interfaces" / "harvest-outputs.schema.json"
    full_schema = json.loads(schema_path.read_text())
    gate_schema = full_schema.get("$defs", full_schema.get("definitions", {})).get("gate_matrix_artifact")
    if gate_schema is None:
        pytest.skip("gate_matrix_artifact not defined in schema yet")
    gate_schema = {**gate_schema, "$defs": full_schema.get("$defs", full_schema.get("definitions", {}))}
    result = produce_gate_matrix(FIXTURE_GO, {"entry_points": []}, [])
    jsonschema.validate(result, gate_schema)
