"""A2 Go route extractor — tree-sitter implementation.

Covers: net/http, gin, gorilla/mux, chi, echo, fiber.
Variable-name-agnostic: matches any selector_expression whose field name
is a recognized HTTP method or handler registration name.
"""
from __future__ import annotations
from pathlib import Path
from audit_harvest.extractors.ts_utils import parse_file, node_text, line_of, walk, unquote

_HTTP_METHODS = frozenset(
    "get post put delete patch head options handle handlefunc method any all".split()
)
_METHOD_NORMALIZE: dict[str, str] = {
    "handle": "ANY",
    "handlefunc": "ANY",
    "any": "ANY",
    "all": "ANY",
}


def _collect_args(call_node, source: bytes) -> tuple[list[str], list[str]]:
    """Return (string_values, ident_texts) from a call_expression's argument_list."""
    args_node = next(
        (c for c in call_node.children if c.type == "argument_list"), None
    )
    if args_node is None:
        return [], []
    strings, idents = [], []
    for child in args_node.children:
        if child.type == "interpreted_string_literal":
            strings.append(unquote(node_text(child, source)))
        elif child.type == "raw_string_literal":
            strings.append(unquote(node_text(child, source)))
        elif child.type in ("identifier", "selector_expression"):
            idents.append(node_text(child, source))
    return strings, idents


def extract_go_routes(repo_path: Path) -> list[dict]:
    routes = []
    for go_file in repo_path.rglob("*.go"):
        if "vendor" in go_file.parts or go_file.name.endswith("_test.go"):
            continue
        try:
            tree, source = parse_file(go_file, "go")
        except Exception:
            continue

        for node in walk(tree.root_node):
            if node.type != "call_expression":
                continue
            fn = node.child_by_field_name("function")
            if fn is None or fn.type != "selector_expression":
                continue
            field_node = fn.child_by_field_name("field")
            if field_node is None:
                continue
            field_name = node_text(field_node, source).lower()
            if field_name not in _HTTP_METHODS:
                continue

            strings, idents = _collect_args(node, source)

            if field_name == "method":
                # chi: r.Method("GET", "/path", handler)
                if len(strings) < 2:
                    continue
                http_method = strings[0].upper()
                path = strings[1]
                handler = idents[0] if idents else "unknown"
            else:
                if not strings:
                    continue
                path = strings[0]
                if not path.startswith("/"):
                    continue
                http_method = _METHOD_NORMALIZE.get(field_name, field_name.upper())
                handler = idents[0] if idents else "unknown"

            if not path.startswith("/"):
                continue

            routes.append({
                "kind": "http",
                "method": http_method,
                "path": path,
                "handler": handler,
                "file": str(go_file.relative_to(repo_path)),
                "line": line_of(node),
                "framework": "go-http",
            })
    return routes
