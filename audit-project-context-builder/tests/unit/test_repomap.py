from pathlib import Path

from audit_harvest.storage import ArtifactStore
from audit_harvest.producers.repomap.producer import produce_repomap


def test_produce_repomap_go_fixture(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    fixture = Path("tests/fixtures/go_simple")

    record = produce_repomap(fixture, store)

    assert record.name == "repomap"
    content = Path(record.path).read_text()
    assert "main.go" in content
    # Fixture defines main, listUsers, createUser, getUser
    assert "func" in content


def test_produce_repomap_python_fixture(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    fixture = Path("tests/fixtures/python_flask")

    record = produce_repomap(fixture, store)

    content = Path(record.path).read_text()
    assert "app.py" in content
    assert "def" in content


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
