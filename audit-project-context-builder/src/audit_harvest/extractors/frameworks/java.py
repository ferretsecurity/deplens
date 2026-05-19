from __future__ import annotations

import re
from pathlib import Path


# Method-level mapping annotations with optional path argument
_MAPPING_PATTERN = re.compile(
    r'@(GetMapping|PostMapping|PutMapping|DeleteMapping|PatchMapping|RequestMapping)'
    r'\s*(?:\(\s*(?:value\s*=\s*)?[\'"]([^\'"]+)[\'"])?',
)
# Class-level base path from @RequestMapping("/base")
_CLASS_MAPPING = re.compile(
    r'@RequestMapping\s*\(\s*[\'"]([^\'"]+)[\'"]',
)
_METHOD_NAME = re.compile(r'(?:public|protected|private)\s+\S+\s+(\w+)\s*\(')

_HTTP_METHOD_MAP = {
    "GetMapping": "GET",
    "PostMapping": "POST",
    "PutMapping": "PUT",
    "DeleteMapping": "DELETE",
    "PatchMapping": "PATCH",
    "RequestMapping": "ANY",
}


def extract_java_routes(repo_path: Path) -> list[dict]:
    routes = []
    for java_file in repo_path.rglob("*.java"):
        try:
            source = java_file.read_text(errors="replace")
        except OSError:
            continue

        # Find class-level base path (first @RequestMapping with a string arg)
        class_base = ""
        cm = _CLASS_MAPPING.search(source)
        if cm:
            class_base = cm.group(1).rstrip("/")

        class_mapping_pos = cm.start() if cm else -1

        for match in _MAPPING_PATTERN.finditer(source):
            # Skip the class-level @RequestMapping itself to avoid duplicate
            if match.start() == class_mapping_pos:
                continue

            annotation = match.group(1)
            path_suffix = match.group(2) or ""
            http_method = _HTTP_METHOD_MAP.get(annotation, "ANY")

            # Find the next method name after this annotation
            rest = source[match.end():]
            mn = _METHOD_NAME.search(rest)
            handler = mn.group(1) if mn else "unknown"

            if path_suffix and not path_suffix.startswith("/"):
                full_path = class_base + "/" + path_suffix
            else:
                full_path = class_base + path_suffix

            if not full_path:
                full_path = "/"

            routes.append({
                "kind": "http",
                "method": http_method,
                "path": full_path,
                "handler": handler,
                "file": str(java_file.relative_to(repo_path)),
                "line": source[:match.start()].count("\n") + 1,
                "framework": "spring",
            })

    return routes
