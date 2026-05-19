"""A14: Index artifact -- hash-tracked summary of all Stage 1 artifacts."""
from __future__ import annotations

import time
from dataclasses import asdict

from audit_harvest.storage import ArtifactStore


def produce_index(store: ArtifactStore, max_age_days: int = 7) -> dict:
    now = time.time()
    threshold = max_age_days * 86400
    artifacts = []
    for record in store.list():
        age = now - record.last_built_at
        entry = asdict(record)
        entry["stale"] = age > threshold
        artifacts.append(entry)
    return {"built_at": now, "artifacts": artifacts}
