# Vendored from https://github.com/Aider-AI/aider (tag v0.62.0)
# License: Apache License 2.0
# Modifications: removed aider-internal dependencies (io, dump, Model, diskcache, tqdm, Spinner)
# Adapted to use tree-sitter 0.25.x API (QueryCursor) and tree_sitter_language_pack-independent
# rendering in place of grep_ast.TreeContext (which is incompatible with tree_sitter 0.25.x).

import importlib
import importlib.resources as resources
import math
import os
import warnings
from collections import Counter, defaultdict, namedtuple
from pathlib import Path

import tiktoken
from grep_ast import filename_to_lang
from pygments.lexers import guess_lexer_for_filename
from pygments.token import Token

from audit_harvest.producers.repomap.vendor.special import filter_important_files

# tree_sitter is throwing a FutureWarning
warnings.simplefilter("ignore", category=FutureWarning)

Tag = namedtuple("Tag", "rel_fname fname line name kind".split())


def _token_count(text: str) -> int:
    enc = tiktoken.get_encoding("cl100k_base")
    return len(enc.encode(text))


class _NoOpIO:
    def tool_warning(self, *a, **kw):
        pass

    def tool_output(self, *a, **kw):
        pass

    def tool_error(self, *a, **kw):
        pass

    def read_text(self, fname):
        try:
            with open(fname, "r", errors="replace") as f:
                return f.read()
        except OSError:
            return None


class _NoOpSpinner:
    def step(self):
        pass

    def end(self):
        pass


# ---------------------------------------------------------------------------
# Language support using the project's per-language tree_sitter packages
# ---------------------------------------------------------------------------

_LANG_TO_PACKAGE: dict[str, str] = {
    "python": "tree_sitter_python",
    "go": "tree_sitter_go",
    "javascript": "tree_sitter_javascript",
    "typescript": "tree_sitter_typescript",
    "java": "tree_sitter_java",
    "ruby": "tree_sitter_ruby",
    "rust": "tree_sitter_rust",
    "c": "tree_sitter_c",
    "cpp": "tree_sitter_cpp",
    "c_sharp": "tree_sitter_c_sharp",
}


def _get_lang_and_parser(lang: str):
    """Return (Language, Parser) for the given language name, or None if unavailable."""
    from tree_sitter import Language, Parser

    pkg_name = _LANG_TO_PACKAGE.get(lang)
    if not pkg_name:
        return None, None
    try:
        pkg = importlib.import_module(pkg_name)
        language = Language(pkg.language())
        parser = Parser(language)
        return language, parser
    except Exception:
        return None, None


def get_scm_fname(lang: str):
    """Return a path-like to the tree-sitter tags SCM for the given language."""
    pkg_name = _LANG_TO_PACKAGE.get(lang)
    if not pkg_name:
        return _NonExistentPath()
    try:
        pkg = importlib.import_module(pkg_name)
        return resources.files(pkg).joinpath("queries/tags.scm")
    except Exception:
        return _NonExistentPath()


class _NonExistentPath:
    def exists(self):
        return False

    def read_text(self, *a, **kw):
        return ""


# ---------------------------------------------------------------------------
# Simple tree-rendering that does NOT require grep_ast.TreeContext
# ---------------------------------------------------------------------------

def _render_lines(abs_fname: str, lois: list[int], context: int = 2) -> str:
    """Return a small source excerpt showing lines of interest with surrounding context."""
    try:
        with open(abs_fname, "r", errors="replace") as fh:
            lines = fh.readlines()
    except OSError:
        return ""

    show: set[int] = set()
    for loi in lois:
        for i in range(max(0, loi - context), min(len(lines), loi + context + 1)):
            show.add(i)

    parts = []
    prev = None
    for i in sorted(show):
        if prev is not None and i > prev + 1:
            parts.append("...\n")
        parts.append(lines[i])
        prev = i
    return "".join(parts)


# ---------------------------------------------------------------------------
# RepoMap
# ---------------------------------------------------------------------------

class RepoMap:
    CACHE_VERSION = 3
    TAGS_CACHE_DIR = f".aider.tags.cache.v{CACHE_VERSION}"

    warned_files = set()

    def __init__(
        self,
        map_tokens=1024,
        root=None,
        main_model=None,
        io=None,
        repo_content_prefix=None,
        verbose=False,
        max_context_window=None,
        map_mul_no_files=8,
        refresh="auto",
    ):
        self.io = io if io is not None else _NoOpIO()
        self.verbose = verbose
        self.refresh = refresh

        if not root:
            root = os.getcwd()
        self.root = root

        self.TAGS_CACHE: dict = {}
        self.cache_threshold = 0.95

        self.max_map_tokens = map_tokens
        self.map_mul_no_files = map_mul_no_files
        self.max_context_window = max_context_window

        self.repo_content_prefix = repo_content_prefix
        self.main_model = main_model

        self.tree_cache: dict = {}
        self.tree_context_cache: dict = {}
        self.map_cache: dict = {}
        self.map_processing_time = 0
        self.last_map = None

    def token_count(self, text: str) -> float:
        len_text = len(text)
        if len_text < 200:
            return _token_count(text)

        lines = text.splitlines(keepends=True)
        num_lines = len(lines)
        step = num_lines // 100 or 1
        lines = lines[::step]
        sample_text = "".join(lines)
        sample_tokens = _token_count(sample_text)
        if not sample_text:
            return 0
        est_tokens = sample_tokens / len(sample_text) * len_text
        return est_tokens

    def get_repo_map(
        self,
        chat_files,
        other_files,
        mentioned_fnames=None,
        mentioned_idents=None,
        force_refresh=False,
    ):
        if self.max_map_tokens <= 0:
            return
        if not other_files:
            return
        if not mentioned_fnames:
            mentioned_fnames = set()
        if not mentioned_idents:
            mentioned_idents = set()

        max_map_tokens = self.max_map_tokens

        # With no files in the chat, give a bigger view of the entire repo
        padding = 4096
        if max_map_tokens and self.max_context_window:
            target = min(
                int(max_map_tokens * self.map_mul_no_files),
                self.max_context_window - padding,
            )
        else:
            target = 0
        if not chat_files and self.max_context_window and target > 0:
            max_map_tokens = target

        try:
            files_listing = self.get_ranked_tags_map(
                chat_files,
                other_files,
                max_map_tokens,
                mentioned_fnames,
                mentioned_idents,
                force_refresh,
            )
        except RecursionError:
            self.io.tool_error("Disabling repo map, git repo too large?")
            self.max_map_tokens = 0
            return

        if not files_listing:
            return

        if self.repo_content_prefix:
            repo_content = self.repo_content_prefix.format(other="")
        else:
            repo_content = ""

        repo_content += files_listing
        return repo_content

    def get_rel_fname(self, fname):
        try:
            return os.path.relpath(fname, self.root)
        except ValueError:
            return fname

    def get_mtime(self, fname):
        try:
            return os.path.getmtime(fname)
        except FileNotFoundError:
            self.io.tool_warning(f"File not found error: {fname}")

    def get_tags(self, fname, rel_fname):
        file_mtime = self.get_mtime(fname)
        if file_mtime is None:
            return []

        cache_key = fname
        val = self.TAGS_CACHE.get(cache_key)
        if val is not None and val.get("mtime") == file_mtime:
            return val["data"]

        data = list(self.get_tags_raw(fname, rel_fname))
        self.TAGS_CACHE[cache_key] = {"mtime": file_mtime, "data": data}
        return data

    def get_tags_raw(self, fname, rel_fname):
        lang = filename_to_lang(fname)
        if not lang:
            return

        language, parser = _get_lang_and_parser(lang)
        if language is None or parser is None:
            return

        query_scm = get_scm_fname(lang)
        if not query_scm.exists():
            return
        query_scm_text = query_scm.read_text()

        code = self.io.read_text(fname)
        if not code:
            return

        tree = parser.parse(bytes(code, "utf-8"))

        from tree_sitter import Query, QueryCursor
        query = Query(language, query_scm_text)
        cursor = QueryCursor(query)

        # Use matches() to get per-pattern results with all captures grouped.
        # Each match is (pattern_index, {capture_name: [Node, ...]}).
        # We look for matches that have both a "name" capture and a "definition.*"
        # or "reference.*" capture to classify each identifier as def or ref.
        saw = set()
        for _pat_idx, cap_dict in cursor.matches(tree.root_node):
            name_nodes = cap_dict.get("name", [])
            if not name_nodes:
                continue

            is_def = any(k.startswith("definition.") for k in cap_dict)
            is_ref = any(k.startswith("reference.") for k in cap_dict)

            if is_def:
                kind = "def"
            elif is_ref:
                kind = "ref"
            else:
                continue

            saw.add(kind)
            for node in name_nodes:
                yield Tag(
                    rel_fname=rel_fname,
                    fname=fname,
                    name=node.text.decode("utf-8"),
                    kind=kind,
                    line=node.start_point[0],
                )

        if "ref" in saw:
            return
        if "def" not in saw:
            return

        # Seen defs without refs — backfill refs via pygments
        try:
            lexer = guess_lexer_for_filename(fname, code)
        except Exception:
            return

        tokens = list(lexer.get_tokens(code))
        tokens = [token[1] for token in tokens if token[0] in Token.Name]

        for token in tokens:
            yield Tag(
                rel_fname=rel_fname,
                fname=fname,
                name=token,
                kind="ref",
                line=-1,
            )

    def get_ranked_tags(
        self, chat_fnames, other_fnames, mentioned_fnames, mentioned_idents, progress=None
    ):
        import networkx as nx

        defines: dict = defaultdict(set)
        references: dict = defaultdict(list)
        definitions: dict = defaultdict(set)
        personalization: dict = {}

        fnames = sorted(set(chat_fnames).union(set(other_fnames)))
        chat_rel_fnames: set = set()

        personalize = 100 / len(fnames) if fnames else 1

        for fname in fnames:
            if progress:
                progress()

            try:
                file_ok = Path(fname).is_file()
            except OSError:
                file_ok = False

            if not file_ok:
                if fname not in self.warned_files:
                    self.io.tool_warning(f"Repo-map can't include {fname}")
                    self.warned_files.add(fname)
                continue

            rel_fname = self.get_rel_fname(fname)

            if fname in chat_fnames:
                personalization[rel_fname] = personalize
                chat_rel_fnames.add(rel_fname)

            if rel_fname in mentioned_fnames:
                personalization[rel_fname] = personalize

            tags = list(self.get_tags(fname, rel_fname))
            if tags is None:
                continue

            for tag in tags:
                if tag.kind == "def":
                    defines[tag.name].add(rel_fname)
                    definitions[(rel_fname, tag.name)].add(tag)
                elif tag.kind == "ref":
                    references[tag.name].append(rel_fname)

        if not references:
            references = {k: list(v) for k, v in defines.items()}

        idents = set(defines.keys()).intersection(set(references.keys()))

        G = nx.MultiDiGraph()

        for ident in idents:
            if progress:
                progress()

            definers = defines[ident]
            if ident in mentioned_idents:
                mul = 10
            elif ident.startswith("_"):
                mul = 0.1
            else:
                mul = 1

            for referencer, num_refs in Counter(references[ident]).items():
                for definer in definers:
                    G.add_edge(referencer, definer, weight=mul * math.sqrt(num_refs), ident=ident)

        if personalization:
            pers_args = dict(personalization=personalization, dangling=personalization)
        else:
            pers_args = {}

        try:
            ranked = nx.pagerank(G, weight="weight", **pers_args)
        except ZeroDivisionError:
            try:
                ranked = nx.pagerank(G, weight="weight")
            except ZeroDivisionError:
                return []

        ranked_definitions: dict = defaultdict(float)
        for src in G.nodes:
            if progress:
                progress()
            src_rank = ranked[src]
            total_weight = sum(d["weight"] for _, _, d in G.out_edges(src, data=True))
            if total_weight == 0:
                continue
            for _, dst, data in G.out_edges(src, data=True):
                data["rank"] = src_rank * data["weight"] / total_weight
                ranked_definitions[(dst, data["ident"])] += data["rank"]

        ranked_tags = []
        for (fname, ident), _rank in sorted(
            ranked_definitions.items(), reverse=True, key=lambda x: (x[1], x[0])
        ):
            if fname in chat_rel_fnames:
                continue
            ranked_tags += list(definitions.get((fname, ident), []))

        rel_other_fnames_without_tags = set(self.get_rel_fname(f) for f in other_fnames)
        fnames_already_included = {rt[0] for rt in ranked_tags}

        for _rank, fname in sorted(
            [(rank, node) for node, rank in ranked.items()], reverse=True
        ):
            rel_other_fnames_without_tags.discard(fname)
            if fname not in fnames_already_included:
                ranked_tags.append((fname,))

        for fname in rel_other_fnames_without_tags:
            ranked_tags.append((fname,))

        return ranked_tags

    def get_ranked_tags_map(
        self,
        chat_fnames,
        other_fnames=None,
        max_map_tokens=None,
        mentioned_fnames=None,
        mentioned_idents=None,
        force_refresh=False,
    ):
        cache_key = (
            tuple(sorted(chat_fnames)) if chat_fnames else None,
            tuple(sorted(other_fnames)) if other_fnames else None,
            max_map_tokens,
            tuple(sorted(mentioned_fnames)) if mentioned_fnames else None,
            tuple(sorted(mentioned_idents)) if mentioned_idents else None,
        )

        use_cache = False
        if not force_refresh:
            if self.refresh == "manual" and self.last_map:
                return self.last_map
            if self.refresh == "files":
                use_cache = True
            elif self.refresh == "auto":
                use_cache = self.map_processing_time > 1.0
            if use_cache and cache_key in self.map_cache:
                return self.map_cache[cache_key]

        import time
        start_time = time.time()
        result = self.get_ranked_tags_map_uncached(
            chat_fnames, other_fnames, max_map_tokens, mentioned_fnames, mentioned_idents
        )
        self.map_processing_time = time.time() - start_time
        self.map_cache[cache_key] = result
        self.last_map = result
        return result

    def get_ranked_tags_map_uncached(
        self,
        chat_fnames,
        other_fnames=None,
        max_map_tokens=None,
        mentioned_fnames=None,
        mentioned_idents=None,
    ):
        if not other_fnames:
            other_fnames = []
        if not max_map_tokens:
            max_map_tokens = self.max_map_tokens
        if not mentioned_fnames:
            mentioned_fnames = set()
        if not mentioned_idents:
            mentioned_idents = set()

        spin = _NoOpSpinner()

        ranked_tags = self.get_ranked_tags(
            chat_fnames,
            other_fnames,
            mentioned_fnames,
            mentioned_idents,
            progress=spin.step,
        )

        other_rel_fnames = sorted(set(self.get_rel_fname(f) for f in other_fnames))
        special_fnames = filter_important_files(other_rel_fnames)
        ranked_tags_fnames = {tag[0] for tag in ranked_tags}
        special_fnames = [(fn,) for fn in special_fnames if fn not in ranked_tags_fnames]

        ranked_tags = special_fnames + ranked_tags

        num_tags = len(ranked_tags)
        lower_bound = 0
        upper_bound = num_tags
        best_tree = None
        best_tree_tokens = 0

        chat_rel_fnames = {self.get_rel_fname(f) for f in chat_fnames}
        self.tree_cache = {}

        middle = min(max_map_tokens // 25, num_tags)
        while lower_bound <= upper_bound:
            tree = self.to_tree(ranked_tags[:middle], chat_rel_fnames)
            num_tokens = self.token_count(tree)

            pct_err = abs(num_tokens - max_map_tokens) / max_map_tokens
            ok_err = 0.15
            if (num_tokens <= max_map_tokens and num_tokens > best_tree_tokens) or pct_err < ok_err:
                best_tree = tree
                best_tree_tokens = num_tokens
                if pct_err < ok_err:
                    break

            if num_tokens < max_map_tokens:
                lower_bound = middle + 1
            else:
                upper_bound = middle - 1

            middle = (lower_bound + upper_bound) // 2

        return best_tree

    def render_tree(self, abs_fname, rel_fname, lois):
        mtime = self.get_mtime(abs_fname)
        key = (rel_fname, tuple(sorted(lois)), mtime)
        if key in self.tree_cache:
            return self.tree_cache[key]

        res = _render_lines(abs_fname, lois)
        self.tree_cache[key] = res
        return res

    def to_tree(self, tags, chat_rel_fnames):
        if not tags:
            return ""

        cur_fname = None
        cur_abs_fname = None
        lois = None
        output = ""

        dummy_tag = (None,)
        for tag in sorted(tags) + [dummy_tag]:
            this_rel_fname = tag[0]
            if this_rel_fname in chat_rel_fnames:
                continue

            if this_rel_fname != cur_fname:
                if lois is not None:
                    output += "\n"
                    output += cur_fname + ":\n"
                    output += self.render_tree(cur_abs_fname, cur_fname, lois)
                    lois = None
                elif cur_fname:
                    output += "\n" + cur_fname + "\n"
                if type(tag) is Tag:
                    lois = []
                    cur_abs_fname = tag.fname
                cur_fname = this_rel_fname

            if lois is not None:
                lois.append(tag.line)

        # truncate long lines in case of minified js or similar
        output = "\n".join([line[:100] for line in output.splitlines()]) + "\n"
        return output
