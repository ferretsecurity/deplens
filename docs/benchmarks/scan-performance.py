"""Measure a built CLI three times per output mode and preserve its output.

Run from the repository root with an absolute binary path and a result label.
The 'after' label also compares output with an earlier 'baseline' run.
"""

import hashlib
import json
import pathlib
import subprocess
import sys
import time

base = pathlib.Path('/tmp/deplens-159-evidence')
base.mkdir(exist_ok=True)
ts = base / 'typescript'
ts.mkdir(exist_ok=True)
content = ''.join(
    f'function ordinary{i}(value: number): number {{ return value + {i}; }}\n'
    for i in range(1500)
)
for i in range(200):
    (ts / f'ordinary{i:03}.ts').write_text(content)

sizes = [1000, 10000, 30000]
for size in sizes:
    root = base / f'pnpm-{size}'
    root.mkdir(exist_ok=True)
    (root / 'pnpm-lock.yaml').write_text(
        "lockfileVersion: '9.0'\npackages:\n"
        + ''.join(f'  package-{i:05}@1.0.0: {{}}\n' for i in range(size))
    )

binary, label = sys.argv[1:]
workloads = [('fixtures', pathlib.Path.cwd() / 'testdata'), ('typescript', ts)]
workloads += [(f'pnpm-{size}', base / f'pnpm-{size}') for size in sizes]
results = {}
for name, root in workloads:
    results[name] = {}
    for mode, args in [('json', ['--json']), ('human', [])]:
        elapsed = []
        previous = None
        for _ in range(3):
            start = time.perf_counter()
            run = subprocess.run([binary, *args, str(root)], capture_output=True, check=True)
            elapsed.append(time.perf_counter() - start)
            if previous is not None and previous != run.stdout:
                raise RuntimeError(f'{name}/{mode}: output changed between repeats')
            previous = run.stdout
        (base / f'{label}-{name}-{mode}.out').write_bytes(run.stdout)
        if label == 'after':
            baseline = (base / f'baseline-{name}-{mode}.out').read_bytes()
            if baseline != run.stdout:
                raise RuntimeError(f'{name}/{mode}: output differs from baseline')
        measured = {
            'seconds': elapsed,
            'bytes': len(run.stdout),
            'sha256': hashlib.sha256(run.stdout).hexdigest(),
            'exit': run.returncode,
        }
        results[name][mode] = measured
        if mode == 'json':
            data = json.loads(run.stdout)
            measured['sources'] = len(data['sources'])
            measured['dependencies'] = sum(
                len(source.get('dependencies', [])) for source in data['sources']
            )
            if name == 'typescript' and data['sources']:
                raise RuntimeError('ordinary TypeScript unexpectedly produced sources')
            if name.startswith('pnpm-'):
                size = int(name.split('-')[1])
                if len(data['sources']) != 1 or measured['dependencies'] != size:
                    raise RuntimeError(f'{name}: wrong dependency count')
                for i, dep in enumerate(data['sources'][0]['dependencies']):
                    if dep['name'] != f'package-{i:05}' or dep['version'] != '1.0.0':
                        raise RuntimeError(f'{name}: wrong dependency value at {i}')
    (base / f'{label}-measurements.json').write_text(json.dumps(results, indent=2))
    print(label, name, results[name], flush=True)
