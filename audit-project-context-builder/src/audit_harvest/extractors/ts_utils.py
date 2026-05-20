"""Shared tree-sitter utilities for route extractors."""
from __future__ import annotations
from pathlib import Path
from typing import Generator
import tree_sitter_go
import tree_sitter_python
import tree_sitter_javascript
import tree_sitter_java
from tree_sitter import Language, Node, Parser

_LANG_MAP: dict[str, Language] = {
    "go": Language(tree_sitter_go.language()),
    "python": Language(tree_sitter_python.language()),
    "javascript": Language(tree_sitter_javascript.language()),
    "typescript": Language(tree_sitter_javascript.language()),  # shared grammar; see KNOWN GAP below
    "java": Language(tree_sitter_java.language()),
}
# KNOWN GAP: TypeScript decorator syntax (NestJS) may not parse fully under the JS grammar.
# If NestJS support is needed, swap "typescript" to use tree_sitter_typescript.language_typescript().


def get_parser(lang: str) -> Parser:
    return Parser(_LANG_MAP[lang])


def parse_file(path: Path, lang: str) -> tuple[object, bytes]:
    """Return (tree, source_bytes) for a source file."""
    source = path.read_bytes()
    return get_parser(lang).parse(source), source


def node_text(node: Node, source: bytes) -> str:
    return source[node.start_byte:node.end_byte].decode("utf-8", errors="replace")


def line_of(node: Node) -> int:
    return node.start_point[0] + 1  # tree-sitter rows are 0-indexed


def walk(node: Node) -> Generator[Node, None, None]:
    """Depth-first traversal of all nodes in the subtree."""
    yield node
    for child in node.children:
        yield from walk(child)


def unquote(text: str) -> str:
    """Strip enclosing quotes from a string literal's raw text."""
    for q in ('"""', "'''", '`'):
        if text.startswith(q) and text.endswith(q) and len(text) >= 2 * len(q):
            return text[len(q):-len(q)]
    if len(text) >= 2 and text[0] in ('"', "'") and text[-1] == text[0]:
        return text[1:-1]
    return text
