# Scan performance measurements

Issue [#159](https://github.com/ferretsecurity/deplens/issues/159) targets avoidable TypeScript parsing and quadratic YAML duplicate-key validation. These synthetic workloads measure those costs. They do not predict the runtime of every repository or explain the unavailable project reported to take 30 minutes.

Measurements use Go 1.25.6, Linux/amd64 under WSL, and an AMD Ryzen 5 7600X. Baseline commit is `614140c9abc0e0f9e0c96aba8d7efa129e014208`. Both versions use the same compiler, build options, scan roots, and workloads. Timings are medians of three runs. Go benchmarks report Go allocations; they do not include tree-sitter's native allocations or total process memory.

## Public Scan benchmarks

The benchmarks load default rules and call `Scan`, including extraction, normalization, project relations, and policy checks. Fixture preparation and correctness assertions run outside the measured interval. The TypeScript workload has 200 files of about 100 KB with valid ordinary functions and no relevant imports. Positive CDK correctness tests also cover aliases, escaped module names, diagnostics, and generation. Pnpm lockfile version 9 workloads contain unique package keys with resolution metadata. Benchmarks verify dependency counts and values.

| Workload | Baseline | Optimized | Speedup | Baseline Go bytes/op | Optimized Go bytes/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| TypeScript, 200 × 100 KB | 4.159 s | 14.02 ms | 296.5× | 247 MB | 22.2 MB |
| TypeScript, 1 × 1 KB | 291.9 µs | 25.84 µs | 11.3× | 17.8 KB | 4.7–5.2 KB |
| Pnpm, 1,000 packages | 4.648 ms | 3.187 ms | 1.46× | 2.47 MB | See repeated benchmark output |
| Pnpm, 10,000 packages | 218.501 ms | 30.906 ms | 7.07× | 27.9 MB | See repeated benchmark output |
| Pnpm, 30,000 packages | 2.374 s | 96.897 ms | 24.5× | 89.9 MB | 84.3 MB |

The large TypeScript runs range from 4.133–4.312 s before to 13.58–14.52 ms after. Go allocation counts fall from about 8.17 million to 9,500. The large pnpm allocation count falls from 810,699 to 720,692. Memory costs remain proportional to the file content and number of extracted references.

Run the committed benchmarks with:

```bash
go test ./internal/analyze -run '^$' -bench 'BenchmarkScan(OrdinaryTypeScript|PNPMLock)' -benchmem -benchtime=3x -count=3
```

For baseline measurements, copy the committed benchmark files into a checkout of the baseline commit. They use the existing public `Scan` interface and need no production changes.

## YAML implementation decision

Keep `gopkg.in/yaml.v3 v3.0.1`. The maintained v3 successor also contains the quadratic validation loop, so changing dependencies would not solve this cost. A custom decoder for pnpm's `packages` mapping checks string-key uniqueness with a map and retains each value as a `yaml.Node`, matching the existing representation. Only mappings of at least 64 entries with unique scalar string keys use this path.

Small mappings, duplicate keys, merge keys, aliases used as keys, and non-string keys use the original decoder. Large mappings with those unusual keys retain their previous performance. Compatibility tests compare decoded nodes and exact error text against the original decoder for duplicates, tagged keys, malformed input, nulls, anchors, aliases, merges, scalars, and non-mappings. Existing importer and old-format extraction tests remain in place. Rules, extension validation, and other YAML analyzers continue using the unchanged shared decoder.

## Full CLI comparisons

`scan-performance.py` creates the ordinary TypeScript project and pnpm projects with 1,000, 10,000, and 30,000 unique package keys, then runs each three times in human and JSON modes. The CLI includes default checks and rendering. It also scans the existing fixture tree. Each pnpm JSON result must contain the expected references. The second run compares every output byte with the baseline and rejects differences. Successful subprocesses must exit with status zero.

Build the baseline and changed versions, then run from the same repository root:

```bash
python3 docs/benchmarks/scan-performance.py /absolute/path/to/baseline-deplens baseline
python3 docs/benchmarks/scan-performance.py /absolute/path/to/changed-deplens after
```

Outputs and machine-readable timings are saved under `/tmp/deplens-159-evidence`. Use an otherwise idle machine when collecting comparative measurements.

## Special files

A static matching FIFO with no writer blocks the baseline CLI. The changed scanner skips FIFOs, sockets, devices, and symlinks to non-regular targets before reading or collecting project evidence. A matching FIFO scan completes with no sources. Symlinks to regular files retain their scan-relative paths, and broken matching symlinks retain read-failure diagnostics. Symlink directories are not traversed. CODEOWNERS policy-input validation remains separate.

Linux special-file regression tests run in a subprocess with a deadline and parent-owned cleanup, so restoring the old blocking implementation cannot hang the whole test runner. Tests also check Cargo application evidence, regular and broken symlinks, directory traversal, and unreadable-file diagnostics.
