from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Optional

from audit_harvest.storage import ArtifactRecord, ArtifactStore


LANG_EXTENSIONS: dict[str, list[str]] = {
    "go": [".go"],
    "python": [".py"],
    "javascript": [".js", ".mjs"],
    "typescript": [".ts", ".tsx"],
    "java": [".java"],
}

EXCLUDE_DIRS = {"vendor", "node_modules", ".git", "testdata", "__pycache__", ".venv"}


def _get_parser(lang: str):
    """Return a tree-sitter Parser for the given language, or None if unavailable."""
    import tree_sitter_go
    import tree_sitter_python
    import tree_sitter_javascript
    import tree_sitter_typescript
    import tree_sitter_java
    from tree_sitter import Language, Parser

    lang_map = {
        "go": tree_sitter_go.language(),
        "python": tree_sitter_python.language(),
        "javascript": tree_sitter_javascript.language(),
        "typescript": tree_sitter_typescript.language_typescript(),
        "java": tree_sitter_java.language(),
    }
    if lang not in lang_map:
        return None
    return Parser(Language(lang_map[lang]))


def _extract_go(source: bytes) -> list[dict]:
    parser = _get_parser("go")
    if parser is None:
        return []
    tree = parser.parse(source)
    symbols = []
    for node in tree.root_node.children:
        if node.type == "function_declaration":
            name = node.child_by_field_name("name")
            if name:
                symbols.append({"name": name.text.decode(), "line": node.start_point[0] + 1, "kind": "function"})
        elif node.type == "method_declaration":
            name = node.child_by_field_name("name")
            if name:
                symbols.append({"name": name.text.decode(), "line": node.start_point[0] + 1, "kind": "method"})
    return symbols


def _extract_python(source: bytes) -> list[dict]:
    parser = _get_parser("python")
    if parser is None:
        return []
    tree = parser.parse(source)
    symbols: list[dict] = []

    def _walk(node) -> None:
        if node.type == "function_definition":
            name_node = node.child_by_field_name("name")
            if name_node:
                symbols.append({
                    "name": name_node.text.decode(),
                    "line": node.start_point[0] + 1,
                    "kind": "function",
                })
            # don't recurse into function bodies (nested defs are noise for repomap)
            return
        if node.type == "class_definition":
            name_node = node.child_by_field_name("name")
            if name_node:
                symbols.append({
                    "name": name_node.text.decode(),
                    "line": node.start_point[0] + 1,
                    "kind": "class",
                })
            # recurse into class body to capture methods
            body = node.child_by_field_name("body")
            if body:
                for child in body.children:
                    _walk(child)
            return
        for child in node.children:
            _walk(child)

    for child in tree.root_node.children:
        _walk(child)
    return symbols


def _extract_java(source: bytes) -> list[dict]:
    parser = _get_parser("java")
    if parser is None:
        return []
    tree = parser.parse(source)
    symbols: list[dict] = []

    def walk(node) -> None:
        if node.type in ("class_declaration", "interface_declaration", "enum_declaration"):
            name = node.child_by_field_name("name")
            if name:
                symbols.append({"name": name.text.decode(), "line": node.start_point[0] + 1, "kind": "class"})
        elif node.type == "method_declaration":
            name = node.child_by_field_name("name")
            if name:
                symbols.append({"name": name.text.decode(), "line": node.start_point[0] + 1, "kind": "method"})
        for child in node.children:
            walk(child)

    walk(tree.root_node)
    return symbols


def _extract_js(source: bytes, lang: str = "javascript") -> list[dict]:
    parser = _get_parser(lang)
    if parser is None:
        return []
    tree = parser.parse(source)
    symbols: list[dict] = []

    def walk(node) -> None:
        if node.type == "function_declaration":
            name = node.child_by_field_name("name")
            if name:
                symbols.append({"name": name.text.decode(), "line": node.start_point[0] + 1, "kind": "function"})
        elif node.type == "class_declaration":
            name = node.child_by_field_name("name")
            if name:
                symbols.append({"name": name.text.decode(), "line": node.start_point[0] + 1, "kind": "class"})
        for child in node.children:
            walk(child)

    walk(tree.root_node)
    return symbols


_EXTRACT: dict[str, object] = {
    "go": _extract_go,
    "python": _extract_python,
    "java": _extract_java,
    "javascript": _extract_js,
    "typescript": lambda s: _extract_js(s, "typescript"),
}


def _get_lang(path: Path) -> Optional[str]:
    ext = path.suffix.lower()
    for lang, exts in LANG_EXTENSIONS.items():
        if ext in exts:
            return lang
    return None


def _collect_symbols(repo_path: Path, max_files: int = 500) -> list[dict]:
    symbols: list[dict] = []
    count = 0
    for path in sorted(repo_path.rglob("*")):
        if count >= max_files:
            break
        if any(part in EXCLUDE_DIRS for part in path.parts):
            continue
        if not path.is_file():
            continue
        lang = _get_lang(path)
        if lang is None:
            continue
        fn = _EXTRACT.get(lang)
        if fn is None:
            continue
        try:
            source = path.read_bytes()
            syms = fn(source)  # type: ignore[operator]
        except Exception:
            syms = []
        rel = str(path.relative_to(repo_path))
        for sym in syms:
            symbols.append({"file": rel, "line": sym["line"], "name": sym["name"], "kind": sym["kind"]})
        count += 1
    return symbols


def _source_hash(repo_path: Path) -> str:
    all_exts = {ext for exts in LANG_EXTENSIONS.values() for ext in exts}
    parts = []
    for p in sorted(repo_path.rglob("*")):
        if not p.is_file():
            continue
        if any(part in EXCLUDE_DIRS for part in p.parts):
            continue
        if p.suffix not in all_exts:
            continue
        stat = p.stat()
        parts.append(f"{p}:{stat.st_mtime}:{stat.st_size}")
    return hashlib.sha256("\n".join(parts).encode()).hexdigest()


def produce_repomap(
    repo_path: Path,
    store: ArtifactStore,
) -> ArtifactRecord:
    src_hash = _source_hash(repo_path)
    if store.is_fresh("repomap", src_hash):
        return store.get("repomap")  # type: ignore[return-value]
    symbols = _collect_symbols(repo_path)
    output = {
        "meta": {"repo_path": str(repo_path), "total_symbols": len(symbols)},
        "symbols": symbols,
    }
    return store.write("repomap", json.dumps(output).encode(), source_hash=src_hash)
