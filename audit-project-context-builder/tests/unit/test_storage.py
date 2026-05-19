import hashlib
import multiprocessing
from pathlib import Path

from audit_harvest.storage import ArtifactRecord, ArtifactStore


def test_write_and_read_roundtrip(tmp_path):
    store = ArtifactStore(tmp_path)
    content = b"hello world"
    record = store.write("repo_profile", content, source_hash="abc123")

    assert record.name == "repo_profile"
    assert record.source_hash == "abc123"
    assert Path(record.path).read_bytes() == content


def test_content_hash_is_sha256(tmp_path):
    store = ArtifactStore(tmp_path)
    content = b"hello world"
    record = store.write("repo_profile", content, source_hash="abc123")

    expected = hashlib.sha256(content).hexdigest()
    assert record.content_hash == expected


def test_get_returns_none_when_missing(tmp_path):
    store = ArtifactStore(tmp_path)
    assert store.get("nonexistent") is None


def test_is_fresh_true_when_source_hash_matches(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"content", source_hash="hash_v1")
    assert store.is_fresh("repo_profile", "hash_v1") is True


def test_is_fresh_false_when_source_hash_changes(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"content", source_hash="hash_v1")
    assert store.is_fresh("repo_profile", "hash_v2") is False


def test_is_fresh_false_when_artifact_missing(tmp_path):
    store = ArtifactStore(tmp_path)
    assert store.is_fresh("repo_profile", "any_hash") is False


def test_atomic_write_does_not_leave_tmp_file(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"data", source_hash="h1")

    tmp_files = list(tmp_path.rglob("*.tmp"))
    assert tmp_files == []


def test_list_returns_all_artifacts(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"a", source_hash="h1")
    store.write("entry_points", b"b", source_hash="h2")

    names = {r.name for r in store.list()}
    assert names == {"repo_profile", "entry_points"}


def test_overwrite_updates_content(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"version1", source_hash="h1")
    store.write("repo_profile", b"version2", source_hash="h2")

    record = store.get("repo_profile")
    assert Path(record.path).read_bytes() == b"version2"
    assert record.source_hash == "h2"


def _write_worker(args):
    root, content, source_hash = args
    store = ArtifactStore(Path(root))
    store.write("repo_profile", content.encode(), source_hash=source_hash)


def test_concurrent_writes_do_not_corrupt(tmp_path):
    args = [
        (str(tmp_path), f"content_{i}", f"hash_{i}")
        for i in range(4)
    ]
    with multiprocessing.Pool(4) as pool:
        pool.map(_write_worker, args)

    # current pointer must resolve to a valid record
    store = ArtifactStore(tmp_path)
    record = store.get("repo_profile")
    assert record is not None
    assert Path(record.path).exists()


def test_concurrent_writes_content_consistent(tmp_path):
    args = [
        (str(tmp_path), f"content_{i}", f"hash_{i}")
        for i in range(4)
    ]
    with multiprocessing.Pool(4) as pool:
        pool.map(_write_worker, args)

    store = ArtifactStore(tmp_path)
    record = store.get("repo_profile")
    assert record is not None
    # Content must be one of the written payloads
    content = Path(record.path).read_bytes()
    assert content in {f"content_{i}".encode() for i in range(4)}
    # source_hash must match one of the written hashes
    assert record.source_hash in {f"hash_{i}" for i in range(4)}
