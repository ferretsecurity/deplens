from __future__ import annotations

import re
from pathlib import Path


# Express: app.get('/path', handler) or router.post('/path', handler)
_EXPRESS_ROUTE = re.compile(
    r'(?:app|router)\s*\.\s*(get|post|put|delete|patch|head|options|use)\s*\(\s*[\'"]([^\'"]+)[\'"]',
    re.IGNORECASE,
)


def extract_js_routes(repo_path: Path) -> list[dict]:
    routes = []
    for js_file in repo_path.rglob("*"):
        if js_file.suffix.lower() not in (".js", ".mjs", ".ts", ".tsx"):
            continue
        if any(part in js_file.parts for part in ("node_modules", ".next", "dist", "build")):
            continue
        try:
            source = js_file.read_text(errors="replace")
        except OSError:
            continue

        for match in _EXPRESS_ROUTE.finditer(source):
            method = match.group(1).upper()
            path = match.group(2)
            if method == "USE":
                continue  # middleware mount, not a route
            routes.append({
                "kind": "http",
                "method": method,
                "path": path,
                "handler": "anonymous",
                "file": str(js_file.relative_to(repo_path)),
                "line": source[:match.start()].count("\n") + 1,
                "framework": "express",
            })
    return routes
