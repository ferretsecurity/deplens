"""A2 Java route extractor — tree-sitter implementation.

Covers Spring MVC: @GetMapping, @PostMapping, @PutMapping, @DeleteMapping,
@PatchMapping, and @RequestMapping (class-level prefix + method-level).
"""
from __future__ import annotations
from pathlib import Path
from tree_sitter import Node
from audit_harvest.extractors.ts_utils import parse_file, node_text, line_of, walk, unquote

_MAPPING_HTTP_METHOD: dict[str, str] = {
    "GetMapping": "GET",
    "PostMapping": "POST",
    "PutMapping": "PUT",
    "DeleteMapping": "DELETE",
    "PatchMapping": "PATCH",
    "RequestMapping": "ANY",
}


def _annotation_name(ann: Node, source: bytes) -> str:
    for child in ann.children:
        if child.type == "identifier":
            return node_text(child, source)
    return ""


def _annotation_path(ann: Node, source: bytes) -> str | None:
    """Return path string from annotation args, or None for marker annotations."""
    if ann.type == "marker_annotation":
        return None
    for child in ann.children:
        if child.type == "annotation_argument_list":
            for item in child.children:
                if item.type == "string_literal":
                    return unquote(node_text(item, source))
                if item.type == "element_value_pair":
                    pair_children = list(item.children)
                    key = node_text(pair_children[0], source) if pair_children else ""
                    if key in ("value", "path"):
                        for c in pair_children[1:]:
                            if c.type == "string_literal":
                                return unquote(node_text(c, source))
    return None


def _full_path(class_prefix: str, method_suffix: str | None) -> str:
    prefix = class_prefix.rstrip("/")
    suffix = (method_suffix or "").rstrip("/")
    if not suffix:
        return prefix or "/"
    if not suffix.startswith("/"):
        suffix = "/" + suffix
    return prefix + suffix


def _get_modifiers(node: Node) -> Node | None:
    """Find the modifiers child of a class_declaration or method_declaration."""
    return next((c for c in node.children if c.type == "modifiers"), None)


def _process_class(class_node: Node, source: bytes, java_file: Path, repo_path: Path, routes: list):
    # Extract class-level @RequestMapping prefix from modifiers
    class_prefix = ""
    modifiers = _get_modifiers(class_node)
    if modifiers:
        for ann in modifiers.children:
            if ann.type in ("annotation", "marker_annotation"):
                if _annotation_name(ann, source) == "RequestMapping":
                    p = _annotation_path(ann, source)
                    if p is not None:
                        class_prefix = p
                    break

    body = class_node.child_by_field_name("body")
    if body is None:
        return

    for method_node in body.children:
        if method_node.type != "method_declaration":
            continue
        name_node = method_node.child_by_field_name("name")
        handler = node_text(name_node, source) if name_node else "unknown"

        method_mods = _get_modifiers(method_node)
        if method_mods is None:
            continue
        for ann in method_mods.children:
            if ann.type not in ("annotation", "marker_annotation"):
                continue
            ann_name = _annotation_name(ann, source)
            http_method = _MAPPING_HTTP_METHOD.get(ann_name)
            if http_method is None:
                continue
            method_suffix = _annotation_path(ann, source)
            path = _full_path(class_prefix, method_suffix)
            routes.append({
                "kind": "http",
                "method": http_method,
                "path": path,
                "handler": handler,
                "file": str(java_file.relative_to(repo_path)),
                "line": line_of(ann),
                "framework": "spring",
            })


def extract_java_routes(repo_path: Path) -> list[dict]:
    routes = []
    for java_file in repo_path.rglob("*.java"):
        try:
            tree, source = parse_file(java_file, "java")
        except Exception:
            continue
        for node in walk(tree.root_node):
            if node.type == "class_declaration":
                _process_class(node, source, java_file, repo_path, routes)
    return routes
