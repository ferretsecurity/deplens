from __future__ import annotations

import re
from pathlib import Path


# Flask: @app.route("/path", methods=["GET", "POST"])
_FLASK_ROUTE_WITH_METHODS = re.compile(
    r'@(?:app|blueprint|bp)\.route\(\s*[\'"]([^\'"]+)[\'"]\s*,\s*methods=\[([^\]]+)\]',
    re.IGNORECASE,
)
# Flask: @app.route("/path") with no methods keyword (defaults to GET)
_FLASK_ROUTE_NO_METHOD = re.compile(
    r'@\w+\.route\(\s*[\'"]([^\'"]+)[\'"]\s*\)',
)
# FastAPI: @router.get("/path") or @app.post("/path")
_FASTAPI_ROUTE = re.compile(
    r'@\w+\.(get|post|put|delete|patch|head|options)\s*\(\s*[\'"]([^\'"]+)[\'"]',
    re.IGNORECASE,
)


def _next_def_name(source: str, pos: int) -> str:
    rest = source[pos:]
    m = re.search(r"def\s+(\w+)", rest)
    return m.group(1) if m else "unknown"


def extract_python_routes(repo_path: Path) -> list[dict]:
    routes = []
    for py_file in repo_path.rglob("*.py"):
        if any(part in py_file.parts for part in ("__pycache__", ".venv", "site-packages")):
            continue
        try:
            source = py_file.read_text(errors="replace")
        except OSError:
            continue

        matched_spans: set[int] = set()

        # Flask routes with explicit methods
        for match in _FLASK_ROUTE_WITH_METHODS.finditer(source):
            matched_spans.add(match.start())
            path = match.group(1)
            methods_raw = match.group(2)
            methods = [m.strip().strip('"\'').upper() for m in methods_raw.split(",")]
            handler = _next_def_name(source, match.end())
            for method in methods:
                routes.append({
                    "kind": "http",
                    "method": method,
                    "path": path,
                    "handler": handler,
                    "file": str(py_file.relative_to(repo_path)),
                    "line": source[:match.start()].count("\n") + 1,
                    "framework": "flask",
                })

        # Flask routes without explicit methods (defaults to GET); skip already matched
        for match in _FLASK_ROUTE_NO_METHOD.finditer(source):
            if match.start() in matched_spans:
                continue
            path = match.group(1)
            handler = _next_def_name(source, match.end())
            routes.append({
                "kind": "http",
                "method": "GET",
                "path": path,
                "handler": handler,
                "file": str(py_file.relative_to(repo_path)),
                "line": source[:match.start()].count("\n") + 1,
                "framework": "flask",
            })

        # FastAPI routes
        for match in _FASTAPI_ROUTE.finditer(source):
            method = match.group(1).upper()
            path = match.group(2)
            handler = _next_def_name(source, match.end())
            routes.append({
                "kind": "http",
                "method": method,
                "path": path,
                "handler": handler,
                "file": str(py_file.relative_to(repo_path)),
                "line": source[:match.start()].count("\n") + 1,
                "framework": "fastapi",
            })

    return routes
