from __future__ import annotations

import json
import time
from pathlib import Path

from audit_harvest.storage import ArtifactRecord, ArtifactStore
from audit_harvest.extractors.frameworks.go import extract_go_routes
from audit_harvest.extractors.frameworks.python import extract_python_routes
from audit_harvest.extractors.frameworks.javascript import extract_js_routes
from audit_harvest.extractors.frameworks.java import extract_java_routes


def produce_entry_points(repo_path: Path, store: ArtifactStore) -> ArtifactRecord:
    all_routes: list[dict] = []
    all_routes.extend(extract_go_routes(repo_path))
    all_routes.extend(extract_python_routes(repo_path))
    all_routes.extend(extract_js_routes(repo_path))
    all_routes.extend(extract_java_routes(repo_path))

    output = {
        "meta": {
            "artifact_id": "entry_points",
            "built_at": time.time(),
            "producer_version": "0.1.0",
            "source_hash": str(len(all_routes)),
        },
        "entry_points": all_routes,
    }

    return store.write(
        "entry_points",
        json.dumps(output).encode(),
        source_hash=str(len(all_routes)),
    )
