# Scan generated Python requirements in CI

This recipe keeps deplens independent of Socket and Snyk. Deplens writes requirements files and JSON metadata; CI owns vendor calls, installation, and resolution.

The inputs in [`examples/vendor-ci`](../examples/vendor-ci) cover custom YAML, TypeScript CDK Glue, and Python CDK Glue. Each source has independent groups with incompatible `urllib3` constraints. The YAML input also has empty and missing groups, which are valid zero-output cases.

## Generate and select files

Use a fresh checkout so an old output cannot affect the result:

```sh
go build -o ./deplens ./cmd/deplens
./deplens --extend-rules examples/vendor-ci/rules.yaml \
  --generate python-requirements --json examples/vendor-ci > generation.json

root=$(jq -r '.root' generation.json)
jq -r '.generation.paths[]' generation.json > generated-paths.txt
while IFS= read -r relative; do
  test -f "$root/$relative"
done < generated-paths.txt
```

Do not parse human output. Resolve each path as JSON `root` plus a relative `.generation.paths[]` entry. An empty path list is a successful zero-output run; do not invoke a vendor scanner with a nonexistent input.

The example produces these exact associations:

| Source and group | Generated JSON path |
| --- | --- |
| `workflow.yaml` / `current` | `workflow.yaml-current.generated-requirements.txt` |
| `workflow.yaml` / `legacy` | `workflow.yaml-legacy.generated-requirements.txt` |
| `jobs.ts` / `typescript-current` | `jobs.ts-typescript-current.generated-requirements.txt` |
| `jobs.ts` / `typescript-legacy` | `jobs.ts-typescript-legacy.generated-requirements.txt` |
| `jobs.py` / `python-current` | `jobs.py-python-current.generated-requirements.txt` |
| `jobs.py` / `python-legacy` | `jobs.py-python-legacy.generated-requirements.txt` |
| `workflow.yaml` / `empty` | no path; outcome status `empty` |
| `workflow.yaml` / `missing` | no path; outcome status `missing` |

## Socket acceptance job

Socket must discover the exact generated names when given the directory. Giving Socket explicit file paths would not verify automatic filename discovery. The acceptance run must confirm all six paths and both `idna` and `urllib3` in the completed report. Then add one exact path to `.gitignore`, repeat the scan, and confirm only that path disappears; `socket scan create` honors `.gitignore` and default ignores.

```yaml
jobs:
  socket-generated-requirements:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: npm install --global @socketsecurity/cli
      - name: Generate manifests
        run: |
          go build -o ./deplens ./cmd/deplens
          ./deplens --extend-rules examples/vendor-ci/rules.yaml \
            --generate python-requirements --json examples/vendor-ci > generation.json
      - name: Scan through Socket discovery
        env:
          SOCKET_SECURITY_API_KEY: ${{ secrets.SOCKET_SECURITY_API_KEY }}
        run: |
          count=$(jq '.generation.paths | length' generation.json)
          test "$count" -eq 0 && exit 0
          test "$count" -eq 6
          socket scan create --repo=deplens-generated-acceptance \
            --branch="${GITHUB_HEAD_REF:-$GITHUB_REF_NAME}" \
            --report --json examples/vendor-ci > socket-report.json
          # Assert in socket-report.json (or its completed linked report) that all
          # six exact paths above and both dependency names were ingested.
```

Do not silently rename files if discovery fails. Record the Socket CLI version, organization supported-files response, scan URL/ID, collected paths, reported packages, and ignore-test evidence.

## Snyk acceptance job

Snyk requires custom requirements filenames to use `--package-manager=pip`. Install every group into a separate virtual environment because constraints can conflict. A scan can exit nonzero because it found a vulnerability or policy violation; that differs from ingestion failure, so retain the JSON and assert the intended packages are present.

```yaml
jobs:
  snyk-generated-requirements:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: actions/setup-python@v5
        with:
          python-version: '3.12'
      - run: npm install --global snyk
      - name: Generate manifests
        run: |
          go build -o ./deplens ./cmd/deplens
          ./deplens --extend-rules examples/vendor-ci/rules.yaml \
            --generate python-requirements --json examples/vendor-ci > generation.json
      - name: Install and scan every group
        env:
          SNYK_TOKEN: ${{ secrets.SNYK_TOKEN }}
        run: |
          root=$(jq -r '.root' generation.json)
          jq -r '.generation.paths[]' generation.json > generated-paths.txt
          test -s generated-paths.txt || exit 0
          index=0
          while IFS= read -r relative; do
            index=$((index + 1))
            manifest="$root/$relative"
            python -m venv ".venv-snyk-$index"
            ".venv-snyk-$index/bin/python" -m pip install -r "$manifest"
            set +e
            snyk test --file="$manifest" --package-manager=pip \
              --command=".venv-snyk-$index/bin/python" --json > "snyk-$index.json"
            status=$?
            set -e
            jq -e '[.. | strings] as $values | \
              ($values | any(test("idna"))) and \
              ($values | any(test("urllib3")))' "snyk-$index.json" >/dev/null
            test "$status" -le 1
          done < generated-paths.txt
```

An unpinned declaration can install a version different from production. This workflow tests the generated declaration, not a resolved production inventory.

## Recorded acceptance status

On 2026-09-21, generation was exercised locally from a fresh temporary copy with Go 1.25.6. It produced the six paths above and two structured zero-output outcomes. Normal Go tests and vet remained offline.

Live ingestion remains incomplete:

- Snyk CLI 1.1300.0 was present, but no `SNYK_TOKEN` was available. No authenticated scan ran, so package reporting is not claimed.
- Socket CLI and credentials were unavailable. Exact filename discovery, package reporting, the organization supported-files response, and `.gitignore` behavior were not verified live.

Public vendor documentation is not evidence for these exact names. Run the jobs above with authorized test-organization credentials and attach the recorded evidence before marking live vendor acceptance complete.
