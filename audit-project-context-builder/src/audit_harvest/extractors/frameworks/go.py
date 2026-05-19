from __future__ import annotations

import re
from pathlib import Path


# Matches gin/chi/echo/gorilla route registration calls
_ROUTE_PATTERN = re.compile(
    r'(?:r|router|mux|e|g)\s*\.\s*(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|Any|Handle)\s*\(\s*"([^"]+)"\s*,\s*(\w+)',
    re.IGNORECASE,
)


def extract_go_routes(repo_path: Path) -> list[dict]:
    routes = []
    for go_file in repo_path.rglob("*.go"):
        if "vendor" in go_file.parts or "_test.go" in go_file.name:
            continue
        try:
            source = go_file.read_text(errors="replace")
        except OSError:
            continue
        for match in _ROUTE_PATTERN.finditer(source):
            method = match.group(1).upper()
            path = match.group(2)
            handler = match.group(3)
            routes.append({
                "kind": "http",
                "method": method,
                "path": path,
                "handler": handler,
                "file": str(go_file.relative_to(repo_path)),
                "line": source[:match.start()].count("\n") + 1,
                "framework": "go-http",
            })
    return routes
