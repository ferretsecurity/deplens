import hashlib
import json
import os
import time
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Optional


@dataclass
class ArtifactRecord:
    name: str
    path: str
    content_hash: str
    source_hash: str
    last_built_at: float
    producer_version: str


class ArtifactStore:
    def __init__(self, root_dir: Path) -> None:
        self.root = Path(root_dir)
        self.root.mkdir(parents=True, exist_ok=True)

    def write(
        self,
        name: str,
        content: bytes,
        source_hash: str,
        producer_version: str = "0.1.0",
    ) -> ArtifactRecord:
        content_hash = hashlib.sha256(content).hexdigest()
        artifact_dir = self.root / name / content_hash
        artifact_dir.mkdir(parents=True, exist_ok=True)

        artifact_path = artifact_dir / "artifact"
        tmp_path = artifact_path.with_suffix(".tmp")
        tmp_path.write_bytes(content)
        tmp_path.replace(artifact_path)

        record = ArtifactRecord(
            name=name,
            path=str(artifact_path),
            content_hash=content_hash,
            source_hash=source_hash,
            last_built_at=time.time(),
            producer_version=producer_version,
        )

        meta_path = artifact_dir / "meta.json"
        tmp_meta = meta_path.with_suffix(".tmp")
        tmp_meta.write_text(json.dumps(asdict(record)))
        tmp_meta.replace(meta_path)

        self._update_current(name, artifact_dir)
        return record

    def _update_current(self, name: str, target_dir: Path) -> None:
        import uuid
        name_dir = self.root / name
        current = name_dir / "current"
        tmp_link = name_dir / f"current.new.{uuid.uuid4().hex}"
        try:
            os.symlink(str(target_dir), str(tmp_link))
            os.replace(str(tmp_link), str(current))
        except Exception:
            try:
                tmp_link.unlink()
            except FileNotFoundError:
                pass
            raise

    def get(self, name: str) -> Optional[ArtifactRecord]:
        current = self.root / name / "current"
        if not current.exists():
            return None
        meta = current / "meta.json"
        if not meta.exists():
            return None
        return ArtifactRecord(**json.loads(meta.read_text()))

    def is_fresh(self, name: str, source_hash: str) -> bool:
        record = self.get(name)
        if record is None:
            return False
        return record.source_hash == source_hash

    def list(self) -> list[ArtifactRecord]:
        records = []
        for item in sorted(self.root.iterdir()):
            if item.is_dir():
                record = self.get(item.name)
                if record:
                    records.append(record)
        return records
