package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatabricksBundleAnalyzerExtractsBaseAndTargetTaskLibraries(t *testing.T) {
	content := []byte(`resources:
  jobs:
    first:
      tasks:
        - task_key: shared
          libraries:
            - pypi: {package: "requests>=2.32"}
            - maven: {coordinates: "org.example:tool:1.0"}
        - task_key: empty
          libraries: []
        - task_key: missing
targets:
  dev:
    resources:
      jobs:
        second:
          tasks:
            - task_key: shared
              libraries:
                - pypi: {package: "urllib3<3"}
`)
	result, err := (databricksBundleAnalyzer{}).Analyze("renamed.yml", content)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
		t.Fatalf("analysis = %+v, recognized = %v", result.Analysis, result.Recognized)
	}
	if len(result.Dependencies) != 3 || result.Dependencies[0].Raw != "requests>=2.32" || result.Dependencies[1].Raw != "org.example:tool:1.0" || result.Dependencies[2].Raw != "urllib3<3" {
		t.Fatalf("dependencies = %+v", result.Dependencies)
	}
	if len(result.Groups) != 8 {
		t.Fatalf("groups = %+v", result.Groups)
	}
	states := map[string]DependencyGroupState{}
	for _, group := range result.Groups {
		states[group.Location] = group.State
	}
	if states[".resources.jobs.first.tasks[1]"] != GroupEmpty || states[".resources.jobs.first.tasks[2]"] != GroupMissing {
		t.Fatalf("group states = %+v", states)
	}
}

func TestDatabricksBundleAnalyzerRejectsUnrelatedYAML(t *testing.T) {
	for _, content := range []string{
		"jobs:\n  build:\n    tasks: [test]\n",
		"resources:\n  jobs:\n    example:\n      description: libraries and tasks\n",
		"resources:\n  jobs: not-a-map\n",
	} {
		result, err := (databricksBundleAnalyzer{}).Analyze("anything.yaml", []byte(content))
		if err != nil {
			t.Fatal(err)
		}
		if result.Recognized {
			t.Fatalf("unrelated YAML recognized: %s", content)
		}
	}
}

func TestDatabricksBundleAnalyzerReportsMalformedNestedFields(t *testing.T) {
	result, err := (databricksBundleAnalyzer{}).Analyze("bundle.yml", []byte(`resources:
  jobs:
    first:
      tasks: not-a-list
targets: []
`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Recognized || result.Analysis.Extraction != ExtractionFailed || result.Analysis.Presence != PresenceAbsent {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, ".resources.jobs.first.tasks") || !strings.Contains(result.Diagnostics[0].Message, ".targets") {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestDatabricksBundleAnalyzerReportsUnsupportedDeclarations(t *testing.T) {
	content := []byte(`resources:
  jobs:
    job:
      tasks:
        - task_key: mixed
          libraries:
            - maven: {coordinates: 42}
            - pypi: {package: "requests==2.32.3"}
            - pypi: {package: "git+https://example.test/repo.git"}
            - pypi: {package: "https://example.test/archive.whl"}
            - pypi: {package: "./local-package"}
            - pypi: {package: "-r requirements.txt"}
            - pypi: {package: "${var.package}"}
            - pypi: {package: "idna", repo: "https://packages.example.test/simple"}
            - whl: ./dist/private.whl
`)
	result, err := (databricksBundleAnalyzer{}).Analyze("bundle.yaml", content)
	if err != nil {
		t.Fatal(err)
	}
	if result.Analysis.Extraction != ExtractionPartial || len(result.Dependencies) != 1 {
		t.Fatalf("result = %+v", result)
	}
	message := result.Diagnostics[0].Message
	for _, want := range []string{"git+https", "archive.whl", "local-package", "requirements.txt", "var.package", "custom repositories", "wheel declarations"} {
		if !strings.Contains(message, want) {
			t.Errorf("diagnostic %q does not contain %q", message, want)
		}
	}
	if !strings.Contains(message, "maven.coordinates") {
		t.Fatalf("Maven diagnostic missing: %s", message)
	}
}

func TestDatabricksBundleBuiltInRuleScansAndGeneratesSeparateFiles(t *testing.T) {
	ruleset, err := LoadDefaultRules()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join("..", "..", "testdata", "databricks", "bundle-renamed")
	root := t.TempDir()
	content, err := os.ReadFile(filepath.Join(fixture, "config", "anything.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "anything.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != 1 || result.Sources[0].Detector != "databricks.bundle.task.python" {
		t.Fatalf("sources = %+v", result.Sources)
	}
	if err := GeneratePythonRequirements(&result, false); err != nil {
		t.Fatal(err)
	}
	if len(result.Generation.Paths) != 2 {
		t.Fatalf("generation = %+v", result.Generation)
	}
	for _, generated := range result.Generation.Paths {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(generated)))
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "requests>=2.32\n" && string(body) != "urllib3<3\n" {
			t.Errorf("unexpected %s contents: %q", generated, body)
		}
	}
	if result.Generation.Paths[0] == result.Generation.Paths[1] || !strings.Contains(result.Generation.Paths[0], "at-") || !strings.Contains(result.Generation.Paths[1], "at-") {
		t.Fatalf("repeated task names were not disambiguated: %v", result.Generation.Paths)
	}
}

func TestDatabricksBundleGenerationDoesNotWriteOnIncompleteSource(t *testing.T) {
	ruleset, err := LoadDefaultRules()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	content := []byte("resources:\n  jobs:\n    job:\n      tasks:\n        - task_key: good\n          libraries:\n            - pypi: {package: 'requests==2.32.3'}\n        - task_key: bad\n          libraries:\n            - pypi: {package: '${var.package}'}\n")
	if err := os.WriteFile(filepath.Join(root, "renamed.yml"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatal(err)
	}
	if err := GeneratePythonRequirements(&result, false); err == nil {
		t.Fatal("expected incomplete extraction to prevent generation")
	}
	matches, err := filepath.Glob(filepath.Join(root, "*.generated-requirements.txt"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("generated files after failed preflight: %v, %v", matches, err)
	}
}

func TestDatabricksBundleMavenCoordinatesAndExclusions(t *testing.T) {
	result, err := (databricksBundleAnalyzer{}).Analyze("bundle.yaml", []byte(`resources:
  jobs:
    ingest:
      tasks:
        - task_key: load
          libraries:
            - maven:
                coordinates: org.apache.spark:spark-sql_2.12:3.5.1
                exclusions: [org.slf4j:slf4j-api, "commons-logging:commons-logging"]
`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Analysis.Extraction != ExtractionComplete || len(result.Dependencies) != 1 {
		t.Fatalf("result = %+v", result)
	}
	dep := result.Dependencies[0]
	if dep.PackageType != "maven" || dep.Name != "org.apache.spark:spark-sql_2.12" || dep.VersionConstraint != "3.5.1" || len(dep.MavenExclusions) != 2 {
		t.Fatalf("dependency = %+v", dep)
	}
}

func TestDatabricksBundleFormatDiagnosticsAreIsolated(t *testing.T) {
	result, err := (databricksBundleAnalyzer{}).Analyze("bundle.yaml", []byte(`resources:
  jobs:
    mixed:
      tasks:
        - task_key: task
          libraries:
            - pypi: {package: "${var.python}"}
            - maven: {coordinates: "org.example:valid:1.0"}
`))
	if err != nil {
		t.Fatal(err)
	}
	var python, maven DependencyGroup
	for _, group := range result.Groups {
		if group.Format == GenerationPythonRequirements {
			python = group
		} else if group.Format == GenerationMavenPOM {
			maven = group
		}
	}
	if len(python.Diagnostics) != 1 || len(maven.Diagnostics) != 0 || len(maven.Dependencies) != 1 {
		t.Fatalf("python=%+v maven=%+v", python, maven)
	}
}
