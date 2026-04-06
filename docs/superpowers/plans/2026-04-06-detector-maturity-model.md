# Detector Maturity Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Document a detector maturity model in the README for built-in and custom detectors without changing runtime behavior.

**Architecture:** Keep the change documentation-only. Add a general maturity-model section, extend the supported-detectors table with a maturity column, and add a short note describing how custom YAML rules map into the same framework.

**Tech Stack:** Markdown documentation, existing repository README structure

---

### Task 1: Update the README with the maturity model

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Review the current README sections that describe detector capabilities**

Run: `sed -n '1,260p' README.md`
Expected: the existing overview, `Supported Detectors` section, and detector table are visible so the new section can be inserted in the right place.

- [ ] **Step 2: Add the new maturity model section and extend the detector table**

Update `README.md` to add a `Detector Maturity Model` section before `Supported Detectors`, using this content:

```md
## Detector Maturity Model

`deplens` uses a detector maturity model to describe detector capability. The maturity level applies to the detector design itself, not to the outcome for an individual matched file.

Per-file outcomes are still represented by scan results such as `has_dependencies` and `dependencies`.

- Level 1: the detector can identify candidate files.
- Level 2: the detector can determine whether a matched file contains dependency declarations.
- Level 3: the detector can extract dependency data in a detector-specific form.
- Level 4: the detector can extract dependency data into a normalized cross-detector format.

No current detector is level 4. That level is reserved for future normalization work because `deplens` does not yet define a shared normalized dependency schema.
```

Then update the `Supported Detectors` table header and rows to include maturity values:

```md
| Detector | Matches | Extracts dependencies | Maturity |
| --- | --- | --- | --- |
| filename regex match | Built-in filename rules: `*requirements*.txt`, `*requirements*.in`, `uv.lock`, `poetry.lock`, `Pipfile.lock`, `pdm.lock`, `conda-lock.yml`, `package.json`, `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock`, `bun.lockb`, `deno.lock`, `pom.xml`, `gradle.lockfile`, `Gemfile`, `Gemfile.lock`, `composer.json`, `composer.lock`, `go.mod`, `go.sum`, `Cargo.toml`, `Cargo.lock`, `*.csproj`, `Gopkg.lock`, `glide.lock`, `conan.lock`, `Package.resolved`, `Podfile.lock`, `mix.lock` | No | 1 |
| path glob match | Built-in path-glob rules such as `python-requirements-dir` for `**/requirements/*.txt` | No | 1 |
| toml | TOML files matched by a rule such as built-in `python-pyproject` for `pyproject.toml`; extracts from `build-system.requires[]`, `project.dependencies[]`, `project.optional-dependencies.*[]`, `dependency-groups.*[]`, `tool.poetry.dependencies`, and `tool.poetry.group.*.dependencies` | Yes | 3 |
| pipfile | `Pipfile` matched by the built-in `python-pipfile` rule; reports only when the file contains at least one dependency-bearing package section such as `[packages]`, `[dev-packages]`, or a custom package category like `[docs]` | Yes | 3 |
| python call | Python files matched by a rule such as built-in `python-setup-py` for `setup.py`; detects imported function calls with specific keyword arguments, for example `setuptools.setup(..., install_requires=..., extras_require=...)`, and can extract from simple literal arrays in `install_requires=[...]` plus `extras_require={"group": [...]}` | Yes | 3 |
| ini | INI files matched by a rule such as built-in `python-setup-cfg` for `setup.cfg`; extracts from `[options]` keys `setup_requires` and `install_requires`, plus all keys under `[options.extras_require]`, when values are written as static multiline lists | Yes | 3 |
| banner regex | JavaScript files whose first 4096 bytes match a configured `banner-regex` with capture groups 1 and 2 for package name and version | Yes | 3 |
| yaml | Path expression such as `workflow.steps[].config.packages.pip[]` to extract data from yaml files, or a presence check such as `dependencies` to detect files that declare a dependency section without extracting it | Sometimes | 2 or 3, depending on rule configuration |
| html external scripts | HTML-like files (`.html`, `.htm`, `.xhtml`, `.tmpl`, `.gohtml`, `.mustache`, `.hbs`, `.njk`) matched by the built-in `html-external-scripts` rule; extracts remote URLs from external `<script src="https://...">` tags, `<script type="module">` imports, and `<script type="importmap">` `imports` entries | Yes | 3 |
| terraform | Terraform `.tf` files with parsing content. For example containing a `aws_glue_job` resource with `default_arguments.--job-language = "python"` and `default_arguments.--additional-python-modules` present | No | 2 |
| typescript cdk construct | TypeScript `.ts` files parsed with an AST. For example containing `new glue.CfnJob(..., { defaultArguments: { "--job-language": "python", "--additional-python-modules": "pandas==2.2.1" }})` imported from `aws-cdk-lib/aws-glue` | Yes | 3 |
| python cdk construct | Python `.py` files with statically-resolved CDK `CfnJob(...)` calls. For example containing `glue.CfnJob(..., default_arguments={"--job-language": "python", "--additional-python-modules": "pandas==2.2.1"})` imported from `aws_cdk.aws_glue` | Yes | 3 |
```

After the table, add this short note:

```md
The same maturity model applies to custom rules passed with `--rules`. In general, selector-only rules are level 1, presence-check rules are level 2, extraction rules are level 3, and level 4 is reserved for future normalized output.
```

- [ ] **Step 3: Review the README diff for wording consistency and scope**

Run: `git diff -- README.md`
Expected: only documentation changes in `README.md`, with no CLI, JSON, or detector behavior changes.

### Task 2: Verify the documentation change

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Re-read the updated README section in context**

Run: `sed -n '1,220p' README.md`
Expected: the new maturity-model section reads naturally before `Supported Detectors`, and the new `Maturity` column aligns with the documented detector capabilities.

- [ ] **Step 2: Confirm the worktree only contains the intended documentation files**

Run: `git status --short`
Expected: `README.md` is modified and the spec/plan docs remain present; no unexpected source-code or test changes appear.

- [ ] **Step 3: Commit the documentation update**

Run:

```bash
git add README.md docs/superpowers/specs/2026-04-06-detector-maturity-model-design.md docs/superpowers/plans/2026-04-06-detector-maturity-model.md
git commit -m "docs: document detector maturity model"
```

Expected: a single commit records the approved spec, implementation plan, and README documentation update.
