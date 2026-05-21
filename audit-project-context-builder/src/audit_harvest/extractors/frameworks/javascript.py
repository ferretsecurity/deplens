"""A2 JS/TS route extractor — tree-sitter implementation.

Covers: Express (any router variable), Next.js file-based API routes,
NestJS HTTP decorators.
"""
from __future__ import annotations
from pathlib import Path
from tree_sitter import Node
from audit_harvest.constants import SKIP_DIRS
from audit_harvest.extractors.ts_utils import parse_file, node_text, line_of, walk, unquote

_EXPRESS_METHODS = frozenset("get post put delete patch head options all".split())
_NESTJS_METHODS = frozenset("Get Post Put Delete Patch Head Options".split())
_SKIP_DIRS = SKIP_DIRS | frozenset({".next", ".nuxt"})
_JS_EXTS = frozenset((".js", ".mjs", ".cjs", ".ts", ".tsx"))


def _lang_for(path: Path) -> str:
    return "typescript" if path.suffix.lower() in (".ts", ".tsx") else "javascript"


def _first_string_arg_js(args_node: Node, source: bytes) -> str | None:
    for child in args_node.children:
        if child.type == "string":
            return unquote(node_text(child, source))
    return None


def _nextjs_path_from_file(js_file: Path, repo_path: Path) -> str | None:
    """Derive a Next.js API route path from the file's location, or None."""
    try:
        rel = js_file.relative_to(repo_path)
    except ValueError:
        return None
    parts = rel.parts
    if len(parts) >= 3 and parts[0] == "pages" and parts[1] == "api":
        segments = list(parts[2:])
        segments[-1] = Path(segments[-1]).stem
        if segments[-1] == "index":
            segments.pop()
        path = "/api/" + "/".join(segments)
        return path.replace("[", ":").replace("]", "")
    if len(parts) >= 3 and parts[0] == "app" and parts[1] == "api":
        if parts[-1].startswith("route."):
            segments = list(parts[2:-1])
            path = "/api/" + "/".join(segments)
            return path.replace("[", ":").replace("]", "")
    return None


def _extract_nestjs_routes(tree, source: bytes, js_file: Path, repo_path: Path, routes: list):
    # KNOWN GAP: tree_sitter_javascript may not emit class_declaration with decorator
    # children for TypeScript files. If nothing is found, NestJS routes are silently
    # skipped. Switch to tree_sitter_typescript.language_typescript() in ts_utils.py
    # to fully support NestJS.
    for class_node in walk(tree.root_node):
        if class_node.type != "class_declaration":
            continue
        controller_prefix = ""
        for child in class_node.children:
            if child.type == "decorator":
                for gc in child.children:
                    if gc.type == "call_expression":
                        fn = gc.child_by_field_name("function")
                        if fn and node_text(fn, source) == "Controller":
                            args = gc.child_by_field_name("arguments")
                            if args:
                                s = _first_string_arg_js(args, source)
                                if s:
                                    controller_prefix = s.rstrip("/")
        body = class_node.child_by_field_name("body")
        if body is None:
            continue
        for method_node in body.children:
            if method_node.type != "method_definition":
                continue
            method_name_node = method_node.child_by_field_name("name")
            handler = node_text(method_name_node, source) if method_name_node else "unknown"
            for child in method_node.children:
                if child.type != "decorator":
                    continue
                for gc in child.children:
                    if gc.type != "call_expression":
                        continue
                    fn = gc.child_by_field_name("function")
                    if fn is None:
                        continue
                    dec_name = node_text(fn, source)
                    if dec_name not in _NESTJS_METHODS:
                        continue
                    args = gc.child_by_field_name("arguments")
                    path_suffix = (_first_string_arg_js(args, source) or "").rstrip("/")
                    if path_suffix and not path_suffix.startswith("/"):
                        path_suffix = "/" + path_suffix
                    full_path = controller_prefix + path_suffix or "/"
                    routes.append({
                        "kind": "http",
                        "method": dec_name.upper(),
                        "path": full_path,
                        "handler": handler,
                        "file": str(js_file.relative_to(repo_path)),
                        "line": line_of(gc),
                        "framework": "nestjs",
                    })


def extract_js_routes(repo_path: Path) -> list[dict]:
    routes = []
    for js_file in repo_path.rglob("*"):
        if js_file.suffix.lower() not in _JS_EXTS:
            continue
        if any(p in js_file.parts for p in _SKIP_DIRS):
            continue

        nextjs_path = _nextjs_path_from_file(js_file, repo_path)
        if nextjs_path is not None:
            routes.append({
                "kind": "http",
                "method": "ANY",
                "path": nextjs_path,
                "handler": js_file.stem,
                "file": str(js_file.relative_to(repo_path)),
                "line": 1,
                "framework": "nextjs",
            })
            continue

        lang = _lang_for(js_file)
        try:
            tree, source = parse_file(js_file, lang)
        except Exception:
            continue

        for node in walk(tree.root_node):
            if node.type != "call_expression":
                continue
            fn = node.child_by_field_name("function")
            if fn is None or fn.type != "member_expression":
                continue
            prop = fn.child_by_field_name("property")
            if prop is None:
                continue
            prop_name = node_text(prop, source).lower()
            if prop_name not in _EXPRESS_METHODS or prop_name == "use":
                continue
            args = node.child_by_field_name("arguments")
            if args is None:
                continue
            path = _first_string_arg_js(args, source)
            if path is None or not path.startswith("/"):
                continue
            routes.append({
                "kind": "http",
                "method": prop_name.upper() if prop_name != "all" else "ANY",
                "path": path,
                "handler": "anonymous",
                "file": str(js_file.relative_to(repo_path)),
                "line": line_of(node),
                "framework": "express",
            })

        if js_file.suffix.lower() in (".ts", ".tsx"):
            _extract_nestjs_routes(tree, source, js_file, repo_path, routes)

    return routes
