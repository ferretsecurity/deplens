"""A2 Python route extractor — tree-sitter implementation.

Covers: Flask (@app.route), FastAPI (@router.get/post/…), Django (urlpatterns).
Variable-name-agnostic: any object name works as the decorator receiver.
"""
from __future__ import annotations
from pathlib import Path
from tree_sitter import Node
from audit_harvest.extractors.ts_utils import parse_file, node_text, line_of, walk, unquote

_HTTP_METHOD_ATTRS = frozenset(
    "get post put delete patch head options".split()
)
_SKIP_DIRS = frozenset(("__pycache__", ".venv", "site-packages", "node_modules"))


def _first_string_arg(args_node: Node, source: bytes) -> str | None:
    for child in args_node.children:
        if child.type == "string":
            return unquote(node_text(child, source))
    return None


def _methods_kwarg(args_node: Node, source: bytes) -> list[str] | None:
    """Return explicit methods list from methods=[...] kwarg, or None if absent/dynamic."""
    for child in args_node.children:
        if child.type != "keyword_argument":
            continue
        key = None
        val_node = None
        for c in child.children:
            if c.type == "identifier" and key is None:
                key = node_text(c, source)
            elif c.type == "list":
                val_node = c
        if key == "methods" and val_node is not None:
            result = []
            for item in val_node.children:
                if item.type == "string":
                    result.append(unquote(node_text(item, source)).upper())
            return result if result else None
    return None


def _handler_from_def(def_node: Node, source: bytes) -> str:
    name = def_node.child_by_field_name("name")
    return node_text(name, source) if name else "unknown"


def _process_decorator(dec_node: Node, source: bytes) -> tuple[str | None, list[str] | None, str | None]:
    """Return (path, methods, framework) or (None, None, None) if not a route decorator."""
    call_node = next((c for c in dec_node.children if c.type == "call"), None)
    if call_node is None:
        return None, None, None

    fn = call_node.child_by_field_name("function")
    if fn is None:
        return None, None, None

    if fn.type == "attribute":
        attr_node = fn.child_by_field_name("attribute")
        attr_name = node_text(attr_node, source).lower() if attr_node else None
    elif fn.type == "identifier":
        attr_name = node_text(fn, source).lower()
    else:
        return None, None, None

    if attr_name is None:
        return None, None, None

    args = call_node.child_by_field_name("arguments")
    if args is None:
        return None, None, None

    path = _first_string_arg(args, source)
    if path is None:
        return None, None, None

    if attr_name == "route":
        methods = _methods_kwarg(args, source) or ["GET"]
        return path, methods, "flask"

    if attr_name in _HTTP_METHOD_ATTRS:
        return path, [attr_name.upper()], "fastapi"

    return None, None, None


def _handle_urlpatterns(assign_node: Node, source: bytes, py_file: Path, repo_path: Path, routes: list):
    left = assign_node.child_by_field_name("left")
    right = assign_node.child_by_field_name("right")
    if left is None or right is None:
        return
    if node_text(left, source).strip() != "urlpatterns":
        return
    if right.type != "list":
        return
    for item in walk(right):
        if item.type != "call":
            continue
        fn = item.child_by_field_name("function")
        if fn is None:
            continue
        fn_name = node_text(fn, source)
        if fn_name not in ("path", "re_path", "url"):
            continue
        args = item.child_by_field_name("arguments")
        if args is None:
            continue
        path = _first_string_arg(args, source)
        if path is None:
            continue
        handler = "unknown"
        string_seen = False
        for c in args.children:
            if c.type == "string":
                string_seen = True
            elif string_seen and c.type in ("identifier", "attribute"):
                handler = node_text(c, source)
                break
        routes.append({
            "kind": "http",
            "method": None,  # Django does not encode HTTP method in urlpatterns
            "path": path,
            "handler": handler,
            "file": str(py_file.relative_to(repo_path)),
            "line": line_of(item),
            "framework": "django",
        })


def extract_python_routes(repo_path: Path) -> list[dict]:
    routes = []
    for py_file in repo_path.rglob("*.py"):
        if any(p in py_file.parts for p in _SKIP_DIRS):
            continue
        try:
            tree, source = parse_file(py_file, "python")
        except Exception:
            continue

        for node in walk(tree.root_node):
            if node.type == "decorated_definition":
                handler = None
                for child in node.children:
                    if child.type == "function_definition":
                        handler = _handler_from_def(child, source)
                if handler is None:
                    continue
                for child in node.children:
                    if child.type != "decorator":
                        continue
                    path, methods, framework = _process_decorator(child, source)
                    if path is None:
                        continue
                    for method in (methods or [None]):
                        routes.append({
                            "kind": "http",
                            "method": method,
                            "path": path,
                            "handler": handler,
                            "file": str(py_file.relative_to(repo_path)),
                            "line": line_of(child),
                            "framework": framework,
                        })
            elif node.type == "assignment":
                _handle_urlpatterns(node, source, py_file, repo_path, routes)

    return routes
