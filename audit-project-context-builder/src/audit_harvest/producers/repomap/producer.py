from __future__ import annotations

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


def _extract_go(source: bytes) -> list[str]:
    parser = _get_parser("go")
    if parser is None:
        return []
    tree = parser.parse(source)
    symbols = []
    for node in tree.root_node.children:
        if node.type == "function_declaration":
            name = node.child_by_field_name("name")
            if name:
                symbols.append(f"func {name.text.decode()}")
        elif node.type == "method_declaration":
            name = node.child_by_field_name("name")
            if name:
                symbols.append(f"func (receiver) {name.text.decode()}")
    return symbols


def _extract_python(source: bytes) -> list[str]:
    parser = _get_parser("python")
    if parser is None:
        return []
    tree = parser.parse(source)
    symbols = []
    for node in tree.root_node.children:
        if node.type == "function_definition":
            name = node.child_by_field_name("name")
            if name:
                symbols.append(f"def {name.text.decode()}")
        elif node.type == "class_definition":
            name = node.child_by_field_name("name")
            if name:
                symbols.append(f"class {name.text.decode()}")
        elif node.type == "decorated_definition":
            # Decorated functions/classes at module level
            inner = node.child_by_field_name("definition")
            if inner and inner.type == "function_definition":
                name = inner.child_by_field_name("name")
                if name:
                    symbols.append(f"def {name.text.decode()}")
            elif inner and inner.type == "class_definition":
                name = inner.child_by_field_name("name")
                if name:
                    symbols.append(f"class {name.text.decode()}")
    return symbols


def _extract_java(source: bytes) -> list[str]:
    parser = _get_parser("java")
    if parser is None:
        return []
    tree = parser.parse(source)
    symbols: list[str] = []

    def walk(node) -> None:
        if node.type in ("class_declaration", "interface_declaration", "enum_declaration"):
            name = node.child_by_field_name("name")
            if name:
                symbols.append(f"class {name.text.decode()}")
        elif node.type == "method_declaration":
            name = node.child_by_field_name("name")
            if name:
                symbols.append(f"method {name.text.decode()}")
        for child in node.children:
            walk(child)

    walk(tree.root_node)
    return symbols


def _extract_js(source: bytes, lang: str = "javascript") -> list[str]:
    parser = _get_parser(lang)
    if parser is None:
        return []
    tree = parser.parse(source)
    symbols: list[str] = []

    def walk(node) -> None:
        if node.type == "function_declaration":
            name = node.child_by_field_name("name")
            if name:
                symbols.append(f"function {name.text.decode()}")
        elif node.type == "class_declaration":
            name = node.child_by_field_name("name")
            if name:
                symbols.append(f"class {name.text.decode()}")
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


def _collect_symbols(repo_path: Path, max_files: int = 500) -> dict[str, list[str]]:
    file_symbols: dict[str, list[str]] = {}
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
        if syms:
            rel = str(path.relative_to(repo_path))
            file_symbols[rel] = syms
        count += 1
    return file_symbols


def _build_repomap_md(file_symbols: dict[str, list[str]]) -> str:
    total = sum(len(v) for v in file_symbols.values())
    lines = [
        "# Repository Symbol Map",
        "",
        f"*{total} symbols across {len(file_symbols)} files*",
    ]
    for file_path, symbols in sorted(file_symbols.items()):
        lines.append("")
        lines.append(f"## `{file_path}`")
        for sym in symbols[:20]:  # cap per file
            lines.append(f"- {sym}")
    return "\n".join(lines)


def produce_repomap(
    repo_path: Path,
    store: ArtifactStore,
) -> ArtifactRecord:
    file_symbols = _collect_symbols(repo_path)
    markdown = _build_repomap_md(file_symbols)
    # Source hash is the symbol count — changes whenever code is added or removed.
    src_hash = str(sum(len(v) for v in file_symbols.values()))
    return store.write("repomap", markdown.encode(), source_hash=src_hash)
