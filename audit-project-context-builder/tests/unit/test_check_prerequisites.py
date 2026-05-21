from unittest.mock import MagicMock, patch


def test_check_prerequisites_all_present():
    """When all Bucket A tools are on PATH, ok=True and no blocking missing."""
    from audit_harvest.mcp_server.tools.run_tools import harvest_check_prerequisites

    with patch("shutil.which", return_value="/usr/bin/tool"):
        result = harvest_check_prerequisites()
    assert result["ok"] is True
    assert result["missing_bucket_a"] == []


def test_check_prerequisites_missing_git_is_blocking():
    """Missing git (Bucket A) must appear in missing_bucket_a and set ok=False."""
    from audit_harvest.mcp_server.tools.run_tools import harvest_check_prerequisites

    def fake_which(name):
        return None if name == "git" else "/usr/bin/tool"

    with patch("shutil.which", side_effect=fake_which):
        result = harvest_check_prerequisites()

    assert result["ok"] is False
    assert "git" in result["missing_bucket_a"]


def test_check_prerequisites_missing_rg_is_blocking():
    """Missing ripgrep (Bucket A) must appear in missing_bucket_a."""
    from audit_harvest.mcp_server.tools.run_tools import harvest_check_prerequisites

    def fake_which(name):
        return None if name == "rg" else "/usr/bin/tool"

    with patch("shutil.which", side_effect=fake_which):
        result = harvest_check_prerequisites()

    assert result["ok"] is False
    assert "rg" in result["missing_bucket_a"]


def test_check_prerequisites_missing_enry_is_blocking():
    """Missing enry (Bucket A) must appear in missing_bucket_a."""
    from audit_harvest.mcp_server.tools.run_tools import harvest_check_prerequisites

    def fake_which(name):
        return None if name == "enry" else "/usr/bin/tool"

    with patch("shutil.which", side_effect=fake_which):
        result = harvest_check_prerequisites()

    assert result["ok"] is False
    assert "enry" in result["missing_bucket_a"]


def test_check_prerequisites_missing_cdxgen_is_non_blocking():
    """Missing cdxgen (Bucket B) must not block the run."""
    from audit_harvest.mcp_server.tools.run_tools import harvest_check_prerequisites

    def fake_which(name):
        return None if name == "cdxgen" else "/usr/bin/tool"

    with patch("shutil.which", side_effect=fake_which):
        result = harvest_check_prerequisites()

    assert result["ok"] is True
    assert "cdxgen" in result["missing_bucket_b"]
    assert "cdxgen" not in result["missing_bucket_a"]


def test_check_prerequisites_missing_osv_scanner_is_non_blocking():
    """Missing osv-scanner (Bucket B) must not block the run."""
    from audit_harvest.mcp_server.tools.run_tools import harvest_check_prerequisites

    def fake_which(name):
        return None if name == "osv-scanner" else "/usr/bin/tool"

    with patch("shutil.which", side_effect=fake_which):
        result = harvest_check_prerequisites()

    assert result["ok"] is True
    assert "osv-scanner" in result["missing_bucket_b"]


def test_check_prerequisites_tool_entry_has_required_fields():
    """Every tool entry must have status, path, version fields."""
    from audit_harvest.mcp_server.tools.run_tools import harvest_check_prerequisites

    mock_proc = MagicMock()
    mock_proc.stdout = "1.0.0\n"
    mock_proc.stderr = ""

    with patch("shutil.which", return_value="/usr/bin/tool"):
        with patch("subprocess.run", return_value=mock_proc):
            result = harvest_check_prerequisites()

    for tool_name, tool_info in result["tools"].items():
        assert "status" in tool_info, f"{tool_name} missing 'status'"
        assert "path" in tool_info, f"{tool_name} missing 'path'"
        assert "version" in tool_info, f"{tool_name} missing 'version'"
