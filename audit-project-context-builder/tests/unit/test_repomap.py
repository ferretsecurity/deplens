import json
from pathlib import Path

from audit_harvest.storage import ArtifactStore
from audit_harvest.producers.repomap.producer import produce_repomap


def test_produce_repomap_go_fixture(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    fixture = Path("tests/fixtures/go_simple")

    record = produce_repomap(fixture, store)

    assert record.name == "repomap"
    data = json.loads(Path(record.path).read_text())
    names = {s["name"] for s in data["symbols"]}
    assert "main" in names
    assert "listUsers" in names
    assert "createUser" in names
    assert "getUser" in names
    assert data["meta"]["total_symbols"] > 0


def test_produce_repomap_python_fixture(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    fixture = Path("tests/fixtures/python_flask")

    record = produce_repomap(fixture, store)

    data = json.loads(Path(record.path).read_text())
    names = {s["name"] for s in data["symbols"]}
    assert "list_users" in names
    assert "create_user" in names
    assert "get_user" in names
    assert data["meta"]["total_symbols"] > 0


def test_produce_repomap_excludes_vendor(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    repo = tmp_path / "repo"
    repo.mkdir()

    # File in vendor/ — should be excluded
    vendor_dir = repo / "vendor" / "pkg"
    vendor_dir.mkdir(parents=True)
    (vendor_dir / "main.go").write_text("package pkg\nfunc VendoredFunc() {}\n")

    # File in the main repo — should be included
    (repo / "main.go").write_text("package main\nfunc MyFunc() {}\n")

    record = produce_repomap(repo, store)
    content = Path(record.path).read_text()

    assert "MyFunc" in content
    assert "VendoredFunc" not in content


def test_produce_repomap_empty_repo(tmp_path):
    """A repo with no recognized source files produces an empty symbols list."""
    store = ArtifactStore(tmp_path / ".audit")
    record = produce_repomap(tmp_path, store)
    data = json.loads(Path(record.path).read_text())
    assert data["symbols"] == []
    assert data["meta"]["total_symbols"] == 0


def test_produce_repomap_multi_language(tmp_path):
    """Symbols from Go and Python files are both captured."""
    go_file = tmp_path / "main.go"
    go_file.write_text("package main\nfunc Foo() {}\n")
    py_file = tmp_path / "app.py"
    py_file.write_text("def bar():\n    pass\n")

    store = ArtifactStore(tmp_path / ".audit")
    record = produce_repomap(tmp_path, store)
    data = json.loads(Path(record.path).read_text())
    names = {s["name"] for s in data["symbols"]}
    assert "Foo" in names
    assert "bar" in names
