import json
from pathlib import Path

import pytest

from audit_harvest.llm.client import GroundedOutput, LLMClient, drop_unverified_claims


def test_verified_claim_is_kept(tmp_path):
    src = tmp_path / "handler.go"
    src.write_text("func getUser(w http.ResponseWriter, r *http.Request) {\n}\n")

    claims = [
        {
            "claim": "getUser is a handler",
            "file": str(src),
            "lines": [1, 1],
            "evidence": "func getUser(",
        }
    ]
    verified = drop_unverified_claims(claims, tmp_path)
    assert len(verified) == 1


def test_fabricated_file_claim_is_dropped(tmp_path):
    claims = [
        {
            "claim": "fakeFunc exists",
            "file": "nonexistent/fake.go",
            "lines": [1, 1],
            "evidence": "func fakeFunc(",
        }
    ]
    verified = drop_unverified_claims(claims, tmp_path)
    assert verified == []


def test_out_of_range_line_claim_is_dropped(tmp_path):
    src = tmp_path / "handler.go"
    src.write_text("line one\n")

    claims = [
        {
            "claim": "something on line 100",
            "file": str(src),
            "lines": [100, 100],
            "evidence": "something",
        }
    ]
    verified = drop_unverified_claims(claims, tmp_path)
    assert verified == []


def test_grounded_output_filters_claims(tmp_path):
    src = tmp_path / "app.py"
    src.write_text("@app.route('/users')\ndef list_users():\n    pass\n")

    raw = GroundedOutput(
        claims=[
            {
                "claim": "list_users is a route handler",
                "file": str(src),
                "lines": [2, 2],
                "evidence": "def list_users",
            },
            {
                "claim": "invented handler",
                "file": "ghost.py",
                "lines": [1, 1],
                "evidence": "def ghost",
            },
        ]
    )
    filtered = raw.verified(tmp_path)
    assert len(filtered.claims) == 1
    assert filtered.claims[0]["claim"] == "list_users is a route handler"


def test_grounding_failure_is_logged_to_file(tmp_path):
    log_path = tmp_path / "llm-grounding-failures.jsonl"
    claims = [
        {
            "claim": "bad",
            "file": "ghost.py",
            "lines": [1, 1],
            "evidence": "nope",
        }
    ]
    drop_unverified_claims(claims, tmp_path, failure_log=log_path)

    assert log_path.exists()
    entry = json.loads(log_path.read_text().strip())
    assert entry["_reason"] == "file_not_found"


def test_multiple_dropped_claims_all_logged(tmp_path):
    log_path = tmp_path / "failures.jsonl"
    claims = [
        {"claim": "bad1", "file": "ghost1.py", "lines": [1, 1], "evidence": "x"},
        {"claim": "bad2", "file": "ghost2.py", "lines": [1, 1], "evidence": "y"},
    ]
    drop_unverified_claims(claims, tmp_path, failure_log=log_path)

    lines = log_path.read_text().strip().splitlines()
    assert len(lines) == 2
    reasons = [json.loads(l)["_reason"] for l in lines]
    assert all(r == "file_not_found" for r in reasons)


def test_relative_file_path_resolved_against_repo_path(tmp_path):
    src = tmp_path / "pkg" / "handler.go"
    src.parent.mkdir(parents=True)
    src.write_text("func handler() {}\n")

    claims = [
        {
            "claim": "handler exists",
            "file": "pkg/handler.go",  # relative path
            "lines": [1, 1],
            "evidence": "func handler",
        }
    ]
    verified = drop_unverified_claims(claims, tmp_path)
    assert len(verified) == 1
