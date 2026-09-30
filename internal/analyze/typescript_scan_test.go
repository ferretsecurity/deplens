package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// BenchmarkScanOrdinaryTypeScript200x100KB includes rule selection, file reads,
// analysis, and policy checks through the same public Scan used by the CLI.
func BenchmarkScanOrdinaryTypeScript200x100KB(b *testing.B) {
	benchmarkScanOrdinaryTypeScript(b, 200, 100*1024)
}

func BenchmarkScanOrdinaryTypeScript1x1KB(b *testing.B) {
	benchmarkScanOrdinaryTypeScript(b, 1, 1024)
}

func benchmarkScanOrdinaryTypeScript(b *testing.B, files, size int) {
	ruleset, err := LoadDefaultRules()
	if err != nil {
		b.Fatal(err)
	}
	root := b.TempDir()
	var code strings.Builder
	for i := 0; code.Len() < size; i++ {
		fmt.Fprintf(&code, "export function ordinary%d(value: number): number { return value + %d; }\n", i, i)
	}
	content := []byte(code.String())
	for i := 0; i < files; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("ordinary%d.ts", i)), content, 0600); err != nil {
			b.Fatal(err)
		}
	}
	want := ScanResult{SchemaVersion: 1, Root: root, Sources: []DependencySourceResult{}, CheckRuns: []CheckRun{{CheckID: "dependency-source-codeowners-missing", Subject: FindingSubject{ProjectRoot: "."}, Status: CheckCompleted}}, Findings: []Finding{}}
	b.SetBytes(int64(files * len(content)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := Scan(root, nil, ruleset)
		if err != nil {
			b.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			b.Fatalf("unexpected scan result: %+v", got)
		}
	}
}

func TestScanTypeScriptDetectorPrecedence(t *testing.T) {
	typed := `  - id: custom-typescript
    filename-regex: '\.ts$'
    form: source-code
    roles: [inventory]
    analyzer:
      type: typescript
      cdk_construct:
        module: custom/jobs
        construct: Job
        props_argument_index: 2
        conditions:
          - key: modules
            present: true
        extract:
          key: modules
          split: comma
`
	fallback := `  - id: fallback
    filename-regex: '\.ts$'
    form: source-code
    roles: [inventory]
`
	for _, tc := range []struct {
		name, rules, content string
		detector             DetectorID
	}{
		{"earlier detector", fallback + typed, `import { Job } from "custom/jobs"; new Job(this, "Job", {modules: "pandas"});`, "fallback"},
		{"later detector after rejection", typed + fallback, `export function ordinary() { return 42; }`, "fallback"},
		{"configured module recognized", typed + fallback, `import { Job } from "custom/jobs"; new Job(this, "Job", {modules: "pandas"});`, "custom-typescript"},
		{"escaped configured module recognized", typed + fallback, `import { Job } from "\u0063ustom/jobs"; new Job(this, "Job", {modules: "pandas"});`, "custom-typescript"},
		{"misleading comment falls through", typed + fallback, `// custom/jobs
new Job(this, "Job", {modules: "pandas"});`, "fallback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ruleset, err := loadRules("rules.yaml", []byte("rules:\n"+tc.rules))
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			mustWriteFile(t, filepath.Join(root, "job.ts"), tc.content)
			got, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Sources) != 1 || got.Sources[0].Detector != tc.detector {
				t.Fatalf("unexpected selected detector: %+v", got.Sources)
			}
			if tc.detector == "custom-typescript" && (len(got.Sources[0].Dependencies) != 1 || got.Sources[0].Dependencies[0].Name != "pandas") {
				t.Fatalf("missing dependencies: %+v", got.Sources[0])
			}
		})
	}
}

func TestScanEscapedTypeScriptGenerationAndPreflight(t *testing.T) {
	code := `import { CfnJob as Job } from "\u0061ws-cdk-lib/aws-glue";
new Job(this, "First", { defaultArguments: { "--job-language": "python", "--additional-python-modules": "pandas==2.2.1" } });
new Job(this, "Second", { defaultArguments: { "--job-language": "python", "--additional-python-modules": "requests==2.32.0" } });`
	for _, dynamic := range []bool{false, true} {
		t.Run(fmt.Sprintf("dynamic=%v", dynamic), func(t *testing.T) {
			root := t.TempDir()
			source := code
			if dynamic {
				source = strings.Replace(source, `"requests==2.32.0"`, "getModules()", 1)
			}
			mustWriteFile(t, filepath.Join(root, "job.ts"), source)
			mustWriteFile(t, filepath.Join(root, "ordinary.ts"), "export function ordinary() { return 42; }")
			got, err := Scan(root, nil, mustLoadDefaultRules(t))
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Sources) != 1 {
				t.Fatalf("unexpected sources: %+v", got.Sources)
			}
			err = GeneratePythonRequirements(&got, false)
			if dynamic {
				if err == nil || !strings.Contains(err.Error(), "cannot be evaluated statically") {
					t.Fatalf("expected preflight failure, got %v", err)
				}
				matches, err := filepath.Glob(filepath.Join(root, "*.generated-requirements.txt"))
				if err != nil {
					t.Fatal(err)
				}
				if len(matches) != 0 {
					t.Fatalf("preflight wrote output: %v", matches)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Generation == nil || len(got.Generation.Paths) != 2 || len(got.Generation.Outcomes) != 2 {
				t.Fatalf("missing independent generated groups: %+v", got.Generation)
			}
			for path, want := range map[string]string{"job.ts-First.generated-requirements.txt": "pandas==2.2.1\n", "job.ts-Second.generated-requirements.txt": "requests==2.32.0\n"} {
				content, err := os.ReadFile(filepath.Join(root, path))
				if err != nil {
					t.Fatal(err)
				}
				if string(content) != want {
					t.Fatalf("%s: got %q want %q", path, content, want)
				}
			}
		})
	}
}
