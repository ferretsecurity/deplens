package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ferretsecurity/deplens/internal/analyze"
)

const groupedRules = `rules:
  - id: company-workflows
    package-type: pypi
    form: automation-definition
    roles: [declaration, constraint]
    filename-regex: '^workflow.*\.yaml$'
    generate: python-requirements
    analyzer:
      type: yaml
      groups:
        query: '.workflows[]'
        name-query: '.name'
        dependencies-query: '.configuration.python.dependencies'
`

func TestGeneratePythonRequirementsFromYAMLGroups(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	rules := filepath.Join(dir, "rules.yaml")
	writeFile(t, rules, groupedRules)
	writeFile(t, filepath.Join(project, "workflow.yaml"), `workflows:
  - name: daily
    configuration: {python: {dependencies: [paramiko, "pandas==1.4.4", "requests[security]>=2.28; python_version >= '3.10'"]}}
  - name: legacy
    configuration: {python: {dependencies: ["pandas<1"]}}
  - name: empty
    configuration: {python: {dependencies: []}}
  - name: missing
    configuration: {python: {}}
`)
	var out, stderr bytes.Buffer
	code := run([]string{"--rules", rules, "--generate", "python-requirements", project}, &out, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
	for name, want := range map[string]string{"daily": "paramiko\npandas==1.4.4\nrequests[security]>=2.28; python_version >= '3.10'\n", "legacy": "pandas<1\n"} {
		data, err := os.ReadFile(filepath.Join(project, "workflow.yaml-"+name+".generated-requirements.txt"))
		if err != nil || string(data) != want {
			t.Fatalf("%s: %q, %v", name, data, err)
		}
	}
	if !strings.Contains(out.String(), "Generated 2 requirements files") || !strings.Contains(out.String(), "skipped: empty dependencies") || !strings.Contains(out.String(), "skipped: missing dependencies") {
		t.Fatalf("output: %s", &out)
	}
}

func TestGeneratePythonRequirementsFromEveryTypeScriptGlueJob(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "jobs.ts"), `import { CfnJob } from "aws-cdk-lib/aws-glue";
new CfnJob(this, "daily", { defaultArguments: {"--job-language": "python", "--additional-python-modules": "pandas==1.4.4,paramiko"} });
new CfnJob(this, "legacy", { defaultArguments: {"--job-language": "python", "--additional-python-modules": "pandas==0.25.3"} });
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--json", "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatalf("run failed: code=%d stderr=%s", code, stderr.String())
	}
	for name, want := range map[string]string{"daily": "pandas==1.4.4\nparamiko\n", "legacy": "pandas==0.25.3\n"} {
		data, err := os.ReadFile(filepath.Join(project, "jobs.ts-"+name+".generated-requirements.txt"))
		if err != nil || string(data) != want {
			t.Fatalf("%s generated content: %q, %v", name, data, err)
		}
	}
	var result analyze.ScanResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	wantPaths := []string{"jobs.ts-daily.generated-requirements.txt", "jobs.ts-legacy.generated-requirements.txt"}
	if !slices.Equal(result.Generation.Paths, wantPaths) {
		t.Fatalf("generated paths: %v", result.Generation.Paths)
	}
}

func TestGeneratePythonRequirementsFromDatabricksBundle(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "config", "renamed.yml"), `resources:
  jobs:
    first:
      tasks:
        - task_key: shared/task
          libraries:
            - pypi: {package: "requests>=2.32"}
            - maven: {coordinates: "org.example:tool:1.0"}
targets:
  production:
    resources:
      jobs:
        second:
          tasks:
            - task_key: shared task
              libraries:
                - pypi: {package: "urllib3<3"}
`)
	writeFile(t, filepath.Join(project, "other.yaml"), `resources:
  jobs:
    third:
      tasks:
        - task_key: shared/task
          libraries:
            - pypi: {package: "idna==3.10"}
`)
	writeFile(t, filepath.Join(project, "unrelated.yaml"), "jobs:\n  example:\n    tasks: [libraries]\n")

	var out, stderr bytes.Buffer
	if code := run([]string{"--json", "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
	var result analyze.ScanResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{
		"config/renamed.yml-shared-task-at-resources.jobs.first.tasks-0.generated-requirements.txt",
		"config/renamed.yml-shared-task-at-targets.production.resources.jobs.second.tasks-0.generated-requirements.txt",
		"other.yaml-shared-task.generated-requirements.txt",
	}
	if !slices.Equal(result.Generation.Paths, wantPaths) {
		t.Fatalf("generated paths: %v, want %v", result.Generation.Paths, wantPaths)
	}
	if len(result.Sources) != 2 || result.Sources[0].Path != "config/renamed.yml" || result.Sources[1].Path != "other.yaml" {
		t.Fatalf("sources: %+v", result.Sources)
	}
	for index, want := range []string{"requests>=2.32\n", "urllib3<3\n", "idna==3.10\n"} {
		body, err := os.ReadFile(filepath.Join(project, filepath.FromSlash(wantPaths[index])))
		if err != nil || string(body) != want {
			t.Fatalf("generated %s: %q, %v", wantPaths[index], body, err)
		}
	}
}

func TestGenerateMavenPOMFromMixedDatabricksBundle(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "bundle.yml"), `resources:
  jobs:
    main:
      tasks:
        - task_key: mixed/task
          libraries:
            - pypi: {package: "${var.invalid_python}"}
            - maven:
                coordinates: "org.example:app:1.2.3"
                exclusions: ["org.example:legacy"]
        - task_key: empty
          libraries: []
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--json", "--generate", "maven-pom", project}, &out, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
	path := filepath.Join(project, "bundle.yml-mixed-task.generated-maven", "pom.xml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document struct{ XMLName xml.Name }
	if err := xml.Unmarshal(body, &document); err != nil || document.XMLName.Local != "project" {
		t.Fatalf("invalid generated POM: root=%s err=%v", document.XMLName.Local, err)
	}
	for _, want := range []string{`xmlns="http://maven.apache.org/POM/4.0.0"`, "<groupId>org.example</groupId>", "<artifactId>app</artifactId>", "<version>1.2.3</version>", "<artifactId>legacy</artifactId>"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("pom missing %q:\n%s", want, body)
		}
	}
	var result analyze.ScanResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Generation.Format != analyze.GenerationMavenPOM || len(result.Generation.Paths) != 1 {
		t.Fatalf("generation = %+v", result.Generation)
	}
}

func TestGenerateMavenFailureLeavesNoDirectories(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "bundle.yaml"), `resources:
  jobs:
    main:
      tasks:
        - task_key: good
          libraries: [{maven: {coordinates: "org.example:good:1.0"}}]
        - task_key: bad
          libraries: [{maven: {coordinates: "not-a-coordinate"}}]
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--generate", "maven-pom", project}, &out, &stderr); code == 0 {
		t.Fatal("expected failure")
	}
	matches, err := filepath.Glob(filepath.Join(project, "*.generated-maven"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("generated directories after failed preflight: %v, %v", matches, err)
	}
}

func TestGeneratePythonIgnoresInvalidDatabricksMaven(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "bundle.yaml"), `resources: {jobs: {main: {tasks: [{task_key: mixed, libraries: [{pypi: {package: "requests==2.32.3"}}, {maven: {coordinates: bad}}]}]}}}`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
	if _, err := os.Stat(filepath.Join(project, "bundle.yaml-mixed.generated-requirements.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratePythonRequirementsFromTerraformGlueJobs(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "jobs.tf"), `
resource "aws_glue_job" "daily" {
  default_arguments = {
    "--additional-python-modules" = "pandas==2.2.1,paramiko"
    "--enable-continuous-cloudwatch-log" = var.logging
  }
}
resource "aws_glue_job" "override" {
  default_arguments = {
    "--job-language" = "python"
    "--additional-python-modules" = "old==1"
  }
  non_overridable_arguments = {
    "--additional-python-modules" = "new==2"
  }
}
resource "aws_glue_job" "scala" {
  default_arguments = {
    "--job-language" = "scala"
    "--additional-python-modules" = "ignored"
  }
}
resource "aws_glue_job" "empty" {
  default_arguments = { "--additional-python-modules" = "" }
}
resource "aws_glue_job" "missing" {}
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--json", "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatalf("run failed: code=%d stderr=%s", code, stderr.String())
	}
	for name, want := range map[string]string{
		"daily":    "pandas==2.2.1\nparamiko\n",
		"override": "new==2\n",
	} {
		data, err := os.ReadFile(filepath.Join(project, "jobs.tf-"+name+".generated-requirements.txt"))
		if err != nil || string(data) != want {
			t.Fatalf("%s generated content: %q, %v", name, data, err)
		}
	}
	var result analyze.ScanResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	statuses := map[string]string{}
	for _, outcome := range result.Generation.Outcomes {
		statuses[outcome.Group] = string(outcome.Status)
	}
	if statuses["empty"] != "empty" || statuses["missing"] != "missing" {
		t.Fatalf("outcomes: %+v", result.Generation.Outcomes)
	}
	if _, found := statuses["scala"]; found {
		t.Fatalf("Scala job produced an outcome: %+v", result.Generation.Outcomes)
	}
}

func TestTerraformGlueIncompleteLaterJobPreventsAllWrites(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "jobs.tf"), `
resource "aws_glue_job" "readable" {
  default_arguments = { "--additional-python-modules" = "pandas==2.2.1" }
}
resource "aws_glue_job" "unreadable" {
  default_arguments = { "--additional-python-modules" = var.modules }
}
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--generate", "python-requirements", project}, &out, &stderr); code == 0 {
		t.Fatalf("generation succeeded: %s", out.String())
	}
	for _, part := range []string{`Glue job "unreadable"`, "line-5-column-1", "cannot be evaluated statically"} {
		if !strings.Contains(stderr.String(), part) {
			t.Fatalf("stderr does not contain %q: %s", part, stderr.String())
		}
	}
	if _, err := os.Stat(filepath.Join(project, "jobs.tf-readable.generated-requirements.txt")); !os.IsNotExist(err) {
		t.Fatalf("valid earlier job was written: %v", err)
	}
}

func TestVendorCIExampleKeepsDocumentedPathsAndZeroOutput(t *testing.T) {
	project := t.TempDir()
	example := filepath.Join("..", "..", "examples", "vendor-ci")
	for _, name := range []string{"jobs.py", "jobs.ts", "workflow.yaml"} {
		content, err := os.ReadFile(filepath.Join(example, name))
		if err != nil {
			t.Fatalf("read example %s: %v", name, err)
		}
		writeFile(t, filepath.Join(project, name), string(content))
	}

	var out, stderr bytes.Buffer
	code := run([]string{"--extend-rules", filepath.Join(example, "rules.yaml"), "--generate", "python-requirements", "--json", project}, &out, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	var result analyze.ScanResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{
		"jobs.py-python-current.generated-requirements.txt",
		"jobs.py-python-legacy.generated-requirements.txt",
		"jobs.ts-typescript-current.generated-requirements.txt",
		"jobs.ts-typescript-legacy.generated-requirements.txt",
		"workflow.yaml-current.generated-requirements.txt",
		"workflow.yaml-legacy.generated-requirements.txt",
	}
	if !slices.Equal(result.Generation.Paths, wantPaths) {
		t.Fatalf("generated paths: %v, want %v", result.Generation.Paths, wantPaths)
	}
	statuses := map[string]string{}
	for _, outcome := range result.Generation.Outcomes {
		statuses[outcome.Group] = string(outcome.Status)
	}
	if statuses["empty"] != "empty" || statuses["missing"] != "missing" {
		t.Fatalf("zero-output statuses: %v", statuses)
	}
}

func TestTypeScriptGlueGenerationUsesLocationForDuplicateAndMissingIDs(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "jobs.ts"), `import * as glue from "aws-cdk-lib/aws-glue";
new glue.CfnJob(this, "same", { defaultArguments: {"--job-language": "python", "--additional-python-modules": "one"} });
new glue.CfnJob(this, "same", { defaultArguments: {"--job-language": "python", "--additional-python-modules": "two"} });
new glue.CfnJob(this, jobID(), { defaultArguments: {"--job-language": "python", "--additional-python-modules": "three"} });
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--json", "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatalf("run failed: code=%d stderr=%s", code, stderr.String())
	}
	var result analyze.ScanResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	want := []string{"jobs.ts-group-at-line-4-column-1.generated-requirements.txt", "jobs.ts-same-at-line-2-column-1.generated-requirements.txt", "jobs.ts-same-at-line-3-column-1.generated-requirements.txt"}
	if !slices.Equal(result.Generation.Paths, want) {
		t.Fatalf("paths: %v, want %v", result.Generation.Paths, want)
	}
}

func TestTypeScriptGlueIncompleteLaterJobPreventsAllWrites(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "jobs.ts"), `import { CfnJob } from "aws-cdk-lib/aws-glue";
new CfnJob(this, "readable", { defaultArguments: {"--job-language": "python", "--additional-python-modules": "pandas==1.4.4"} });
new CfnJob(this, "unreadable", { defaultArguments: {"--job-language": "python", "--additional-python-modules": buildModules()} });
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--generate", "python-requirements", project}, &out, &stderr); code == 0 {
		t.Fatalf("expected failure: stdout=%s", out.String())
	}
	if !strings.Contains(stderr.String(), `job "unreadable"`) || !strings.Contains(stderr.String(), "cannot be evaluated statically") {
		t.Fatalf("missing contextual diagnostic: %s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(project, "jobs.ts-readable.generated-requirements.txt")); !os.IsNotExist(err) {
		t.Fatalf("valid earlier job was written: %v", err)
	}
}

func TestTypeScriptGlueMissingDependencyDeclarationIsAStructuredSkip(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "jobs.ts"), `import { CfnJob } from "aws-cdk-lib/aws-glue";
new CfnJob(this, "missing", { defaultArguments: {"--job-language": "python"} });
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--json", "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatalf("run failed: code=%d stderr=%s", code, stderr.String())
	}
	var result analyze.ScanResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Generation.Outcomes) != 1 {
		t.Fatalf("outcomes: %+v", result.Generation.Outcomes)
	}
	outcome := result.Generation.Outcomes[0]
	if outcome.Group != "missing" || outcome.Location != "line-2-column-1" || outcome.Status != "missing" || outcome.Path != "" {
		t.Fatalf("outcome: %+v", outcome)
	}
}

func TestTypeScriptGlueUnreadableDependencyDeclarationKeepsJobContext(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "jobs.ts"), `import { CfnJob } from "aws-cdk-lib/aws-glue";
new CfnJob(this, "dynamic", { defaultArguments: {"--job-language": "python", "--additional-python-modules": buildModules()} });
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--generate", "python-requirements", project}, &out, &stderr); code == 0 {
		t.Fatal("generation succeeded")
	}
	for _, part := range []string{`job "dynamic"`, "line-2-column-1", `dependency declaration "--additional-python-modules" cannot be evaluated statically`} {
		if !strings.Contains(stderr.String(), part) {
			t.Fatalf("stderr does not contain %q: %s", part, stderr.String())
		}
	}
}

func TestGenerateRejectsMalformedPEP508MarkersBeforeWriting(t *testing.T) {
	for _, marker := range []string{
		`python_version >=`,
		`python_version >= '3.10' and`,
		`python_version '3.10'`,
		`unknown_variable == 'x'`,
		`(python_version >= '3.10'`,
	} {
		t.Run(marker, func(t *testing.T) {
			dir := t.TempDir()
			project := filepath.Join(dir, "project")
			rules := filepath.Join(dir, "rules.yaml")
			writeFile(t, rules, groupedRules)
			writeFile(t, filepath.Join(project, "workflow-a.yaml"), "workflows: [{name: valid, configuration: {python: {dependencies: [paramiko]}}}]\n")
			writeFile(t, filepath.Join(project, "workflow-z.yaml"), "workflows: [{name: invalid, configuration: {python: {dependencies: [\"requests; "+marker+"\"]}}}]\n")
			var out, stderr bytes.Buffer
			if code := run([]string{"--rules", rules, "--generate", "python-requirements", project}, &out, &stderr); code == 0 {
				t.Fatal("generation succeeded")
			}
			if !strings.Contains(stderr.String(), "invalid environment marker") {
				t.Fatalf("stderr: %s", stderr.String())
			}
			if _, err := os.Stat(filepath.Join(project, "workflow-a.yaml-valid.generated-requirements.txt")); !os.IsNotExist(err) {
				t.Fatalf("preflight wrote valid destination: %v", err)
			}
		})
	}
}

func TestTypeScriptGlueGenerationCanBeDisabled(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "job.ts"), `import { CfnJob } from "aws-cdk-lib/aws-glue";
new CfnJob(this, "daily", { defaultArguments: {"--job-language": "python", "--additional-python-modules": "pandas"} });
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--disable-rule", "typescript.cdk.aws_glue_job.python", "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatalf("run failed: %s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(project, "job.ts-daily.generated-requirements.txt")); !os.IsNotExist(err) {
		t.Fatalf("disabled detector generated a file: %v", err)
	}
}

func TestGenerateJSONAndNoFlag(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	rules := filepath.Join(dir, "rules.yaml")
	writeFile(t, rules, groupedRules)
	writeFile(t, filepath.Join(project, "workflow.yaml"), "workflows: [{name: daily, configuration: {python: {dependencies: [paramiko]}}}]\n")
	var out, stderr bytes.Buffer
	if code := run([]string{"--rules", rules, project}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	destination := filepath.Join(project, "workflow.yaml-daily.generated-requirements.txt")
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("normal scan wrote generated file")
	}
	out.Reset()
	stderr.Reset()
	if code := run([]string{"--json", "--rules", rules, "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	var result analyze.ScanResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Generation == nil || len(result.Generation.Paths) != 1 || result.Generation.Paths[0] != "workflow.yaml-daily.generated-requirements.txt" {
		t.Fatalf("generation: %+v", result.Generation)
	}
}

func TestGeneratePreflightLeavesDestinationsUntouched(t *testing.T) {
	for _, tc := range []struct {
		name, second string
		existing     bool
	}{{"invalid-later", "workflows: [{name: broken, configuration: {python: {dependencies: [\"git+https://example.test/repo\"]}}}]\n", false}, {"existing-destination", "workflows: [{name: later, configuration: {python: {dependencies: [flask]}}}]\n", true}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			project := filepath.Join(dir, "project")
			rules := filepath.Join(dir, "rules.yaml")
			writeFile(t, rules, groupedRules)
			writeFile(t, filepath.Join(project, "workflow-a.yaml"), "workflows: [{name: first, configuration: {python: {dependencies: [paramiko]}}}]\n")
			writeFile(t, filepath.Join(project, "workflow-z.yaml"), tc.second)
			if tc.existing {
				writeFile(t, filepath.Join(project, "workflow-z.yaml-later.generated-requirements.txt"), "keep\n")
			}
			var out, stderr bytes.Buffer
			if code := run([]string{"--rules", rules, "--generate", "python-requirements", project}, &out, &stderr); code == 0 {
				t.Fatal("generation succeeded")
			}
			if _, err := os.Stat(filepath.Join(project, "workflow-a.yaml-first.generated-requirements.txt")); !os.IsNotExist(err) {
				t.Fatal("preflight wrote early destination")
			}
			if tc.existing {
				data, _ := os.ReadFile(filepath.Join(project, "workflow-z.yaml-later.generated-requirements.txt"))
				if string(data) != "keep\n" {
					t.Fatalf("existing file changed: %q", data)
				}
			}
		})
	}
}

func TestGenerateDoesNotFollowDestinationSymlink(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	rules := filepath.Join(dir, "rules.yaml")
	writeFile(t, rules, groupedRules)
	writeFile(t, filepath.Join(project, "workflow.yaml"), "workflows: [{name: daily, configuration: {python: {dependencies: [paramiko]}}}]\n")
	target := filepath.Join(dir, "target.txt")
	writeFile(t, target, "keep\n")
	if err := os.Symlink(target, filepath.Join(project, "workflow.yaml-daily.generated-requirements.txt")); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"--rules", rules, "--generate", "python-requirements", project}, &out, &stderr); code == 0 {
		t.Fatal("generation followed destination symlink")
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "keep\n" {
		t.Fatalf("symlink target changed: %q, %v", data, err)
	}
}

func TestGenerateOverwriteReplacesOnlyPlannedRegularFiles(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	rules := filepath.Join(dir, "rules.yaml")
	writeFile(t, rules, groupedRules)
	source := filepath.Join(project, "workflow.yaml")
	destination := filepath.Join(project, "workflow.yaml-daily.generated-requirements.txt")
	unrelated := filepath.Join(project, "notes.txt")
	writeFile(t, source, "workflows: [{name: daily, configuration: {python: {dependencies: [paramiko, \"pandas==1.4.4\"]}}}]\n")
	writeFile(t, destination, "old\n")
	writeFile(t, unrelated, "keep\n")

	for range 2 {
		var out, stderr bytes.Buffer
		if code := run([]string{"--rules", rules, "--generate", "python-requirements", "--overwrite-generated", project}, &out, &stderr); code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, &stderr)
		}
		data, err := os.ReadFile(destination)
		if err != nil || string(data) != "paramiko\npandas==1.4.4\n" {
			t.Fatalf("destination: %q, %v", data, err)
		}
	}
	data, err := os.ReadFile(unrelated)
	if err != nil || string(data) != "keep\n" {
		t.Fatalf("unrelated file changed: %q, %v", data, err)
	}
}

func TestGenerateOverwritePreflightLeavesAllDestinationsUntouched(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	rules := filepath.Join(dir, "rules.yaml")
	writeFile(t, rules, groupedRules)
	writeFile(t, filepath.Join(project, "workflow-a.yaml"), "workflows: [{name: first, configuration: {python: {dependencies: [paramiko]}}}]\n")
	writeFile(t, filepath.Join(project, "workflow-z.yaml"), "workflows: [{name: broken, configuration: {python: {dependencies: [\"git+https://example.test/repo\"]}}}]\n")
	firstDestination := filepath.Join(project, "workflow-a.yaml-first.generated-requirements.txt")
	writeFile(t, firstDestination, "old\n")
	if err := os.Chmod(firstDestination, 0o600); err != nil {
		t.Fatal(err)
	}

	var out, stderr bytes.Buffer
	if code := run([]string{"--rules", rules, "--generate", "python-requirements", "--overwrite-generated", project}, &out, &stderr); code == 0 {
		t.Fatal("generation succeeded")
	}
	data, err := os.ReadFile(firstDestination)
	if err != nil || string(data) != "old\n" {
		t.Fatalf("existing destination changed: %q, %v", data, err)
	}
	info, err := os.Stat(firstDestination)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("existing destination mode changed: %v, %v", info, err)
	}
	if _, err := os.Stat(filepath.Join(project, "workflow-z.yaml-broken.generated-requirements.txt")); !os.IsNotExist(err) {
		t.Fatal("generation created later destination")
	}
}

func TestGenerateOverwriteRejectsDestinationSymlinks(t *testing.T) {
	for _, dangling := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "dangling"}[dangling], func(t *testing.T) {
			dir := t.TempDir()
			project := filepath.Join(dir, "project")
			rules := filepath.Join(dir, "rules.yaml")
			writeFile(t, rules, groupedRules)
			writeFile(t, filepath.Join(project, "workflow.yaml"), "workflows: [{name: daily, configuration: {python: {dependencies: [paramiko]}}}]\n")
			target := filepath.Join(dir, "target.txt")
			if !dangling {
				writeFile(t, target, "keep\n")
			}
			if err := os.Symlink(target, filepath.Join(project, "workflow.yaml-daily.generated-requirements.txt")); err != nil {
				t.Fatal(err)
			}

			var out, stderr bytes.Buffer
			if code := run([]string{"--rules", rules, "--generate", "python-requirements", "--overwrite-generated", project}, &out, &stderr); code == 0 || !strings.Contains(stderr.String(), "not a regular file") {
				t.Fatalf("exit=%d stderr=%s", code, &stderr)
			}
			if !dangling {
				data, err := os.ReadFile(target)
				if err != nil || string(data) != "keep\n" {
					t.Fatalf("symlink target changed: %q, %v", data, err)
				}
			}
		})
	}
}

func TestOverwriteGeneratedRequiresGeneration(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := run([]string{"--overwrite-generated"}, &out, &stderr); code == 0 || !strings.Contains(stderr.String(), "requires --generate") {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
}

func TestGenerateRejectsInvalidConfigurationAndGroups(t *testing.T) {
	for _, tc := range []struct{ name, rule, source, errorPart string }{{"format", groupedRules, "workflows: []\n", "unsupported generation format"}, {"eligibility", strings.Replace(groupedRules, "python-requirements", "cyclonedx", 1), "workflows: []\n", "generate"}, {"wrong-type", groupedRules, "workflows: [{name: daily, configuration: {python: {dependencies: flask}}}]\n", "must be a list"}, {"name-shape", strings.Replace(groupedRules, "name-query: '.name'", "name-query: '.name[]'", 1), "workflows: [{name: [one, two], configuration: {python: {dependencies: [flask]}}}]\n", "name-query must produce one value"}, {"transformation", strings.Replace(groupedRules, "query: '.workflows[]'", "query: '.workflows[] | {name: .name}'", 1), "workflows: [{name: daily, configuration: {python: {dependencies: [flask]}}}]\n", "select yaml groups"}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			project := filepath.Join(dir, "project")
			rules := filepath.Join(dir, "rules.yaml")
			writeFile(t, rules, tc.rule)
			writeFile(t, filepath.Join(project, "workflow.yaml"), tc.source)
			format := "python-requirements"
			if tc.name == "format" {
				format = "other"
			}
			var out, stderr bytes.Buffer
			if code := run([]string{"--rules", rules, "--generate", format, project}, &out, &stderr); code == 0 || !strings.Contains(stderr.String(), tc.errorPart) {
				t.Fatalf("exit=%d stderr=%s", code, &stderr)
			}
		})
	}
}

func TestGenerateFromMappingKeysAndQuotedPath(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	rules := filepath.Join(dir, "rules.yaml")
	writeFile(t, rules, strings.NewReplacer(
		"query: '.workflows[]'", "query: '.[\"workflow groups\"][]'",
		"name-query: '.name'", "name-query: '$key'",
		"dependencies-query: '.configuration.python.dependencies'", "dependencies-query: '.python.dependencies'").Replace(groupedRules))
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "python", "grouped-workflow-mapping", "workflow.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(project, "workflow.yaml"), string(fixture))
	var out, stderr bytes.Buffer
	if code := run([]string{"--rules", rules, "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
	for name, want := range map[string]string{"daily": "paramiko\npandas==1.4.4\n", "legacy": "pandas<1\n"} {
		data, err := os.ReadFile(filepath.Join(project, "workflow.yaml-"+name+".generated-requirements.txt"))
		if err != nil || string(data) != want {
			t.Fatalf("%s: %q, %v", name, data, err)
		}
	}
}

func TestGenerateFilteredGroupsPreserveLocationsAndDisambiguateNames(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	rules := filepath.Join(dir, "rules.yaml")
	writeFile(t, rules, strings.Replace(groupedRules, "query: '.workflows[]'", "query: '.workflows[] | select(.enabled)'", 1))
	writeFile(t, filepath.Join(project, "workflow.yaml"), `workflows:
  - {name: skip, enabled: false, configuration: {python: {dependencies: [ignored]}}}
  - {name: same, enabled: true, configuration: {python: {dependencies: [first]}}}
  - {name: same, enabled: true, configuration: {python: {dependencies: [second]}}}
  - {enabled: true, configuration: {python: {dependencies: [unnamed]}}}
  - {name: "unsafe/name", enabled: true, configuration: {python: {dependencies: [unsafe]}}}
  - {name: "unsafe name", enabled: true, configuration: {python: {dependencies: [collision]}}}
  - {name: "unique/unsafe", enabled: true, configuration: {python: {dependencies: [converted]}}}
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--json", "--rules", rules, "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
	var result analyze.ScanResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{
		"workflow.yaml-group-at-workflows-3.generated-requirements.txt",
		"workflow.yaml-same-at-workflows-1.generated-requirements.txt",
		"workflow.yaml-same-at-workflows-2.generated-requirements.txt",
		"workflow.yaml-unique-unsafe.generated-requirements.txt",
		"workflow.yaml-unsafe-name-at-workflows-4.generated-requirements.txt",
		"workflow.yaml-unsafe-name-at-workflows-5.generated-requirements.txt",
	}
	if strings.Join(result.Generation.Paths, "\n") != strings.Join(wantPaths, "\n") {
		t.Fatalf("paths:\n%v\nwant:\n%v", result.Generation.Paths, wantPaths)
	}
	locations := map[string]string{}
	for _, outcome := range result.Generation.Outcomes {
		locations[outcome.Group+":"+outcome.Path] = outcome.Location
	}
	if locations["same:"+wantPaths[1]] != ".workflows[1]" || locations[":"+wantPaths[0]] != ".workflows[3]" {
		t.Fatalf("outcomes did not retain source identity: %+v", result.Generation.Outcomes)
	}
	if locations["unique/unsafe:"+wantPaths[3]] != ".workflows[6]" {
		t.Fatalf("report did not retain unsafe original name: %+v", result.Generation.Outcomes)
	}
}

func TestGenerateIdenticalMappingValuesRemainDistinct(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	rules := filepath.Join(dir, "rules.yaml")
	writeFile(t, rules, strings.NewReplacer(
		"query: '.workflows[]'", "query: '.workflows[]'",
		"name-query: '.name'", "name-query: '$key'",
		"dependencies-query: '.configuration.python.dependencies'", "dependencies-query: '.dependencies'").Replace(groupedRules))
	writeFile(t, filepath.Join(project, "workflow.yaml"), "workflows: {one: {dependencies: [flask]}, two: {dependencies: [flask]}}\n")
	var out, stderr bytes.Buffer
	if code := run([]string{"--rules", rules, "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	for _, name := range []string{"one", "two"} {
		if _, err := os.Stat(filepath.Join(project, "workflow.yaml-"+name+".generated-requirements.txt")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGenerateFromRootList(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	rules := filepath.Join(dir, "rules.yaml")
	writeFile(t, rules, strings.NewReplacer(
		"query: '.workflows[]'", "query: '.[]'",
		"dependencies-query: '.configuration.python.dependencies'", "dependencies-query: '.dependencies'",
	).Replace(groupedRules))
	writeFile(t, filepath.Join(project, "workflow.yaml"), "- {name: root, dependencies: [flask]}\n")
	var out, stderr bytes.Buffer
	if code := run([]string{"--rules", rules, "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(project, "workflow.yaml-root.generated-requirements.txt"))
	if err != nil || string(data) != "flask\n" {
		t.Fatalf("generated root list: %q, %v", data, err)
	}
}

func TestGeneratePythonCDKGlueJobs(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "python", "glue-cfnjob-multiple", "job.py"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(project, "job.py"), string(fixture))
	writeFile(t, filepath.Join(project, "requirements.txt"), "should-not-generate\n")

	var out, stderr bytes.Buffer
	if code := run([]string{"--json", "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
	var result analyze.ScanResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Generation == nil || len(result.Generation.Paths) != 5 {
		t.Fatalf("generation: %+v", result.Generation)
	}
	wantContents := map[string]int{
		"requests>=2.31\nurllib3<3\n": 1,
		"pandas==1.4.4\n":             1,
		"paramiko\n":                  1,
		"flask<3\n":                   1,
		"flask>=3\n":                  1,
	}
	for _, outcome := range result.Generation.Outcomes {
		data, err := os.ReadFile(filepath.Join(project, filepath.FromSlash(outcome.Path)))
		if err != nil || wantContents[string(data)] == 0 {
			t.Fatalf("group %q path %q: %q, %v", outcome.Group, outcome.Path, data, err)
		}
		wantContents[string(data)]--
		if (outcome.Group == "daily/job" || outcome.Group == "daily job") && !strings.Contains(outcome.Path, "daily-job-at-line-") {
			t.Fatalf("converted collision was not disambiguated: %+v", outcome)
		}
		if outcome.Group == "" && !strings.Contains(outcome.Path, "group-at-line-") {
			t.Fatalf("missing name did not use location fallback: %+v", outcome)
		}
		if outcome.Group == "duplicate" && !strings.Contains(outcome.Path, "duplicate-at-line-") {
			t.Fatalf("duplicate name was not disambiguated: %+v", outcome)
		}
	}

	projectWithoutFlag := filepath.Join(dir, "without-flag")
	writeFile(t, filepath.Join(projectWithoutFlag, "job.py"), string(fixture))
	out.Reset()
	stderr.Reset()
	if code := run([]string{projectWithoutFlag}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	matches, err := filepath.Glob(filepath.Join(projectWithoutFlag, "*.generated-requirements.txt"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("ordinary scan generated files: %v, %v", matches, err)
	}
}

func TestGeneratePythonCDKIncompleteLaterJobLeavesNoOutput(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "a.py"), `from aws_cdk import aws_glue as glue
glue.CfnJob(self, "first", default_arguments={"--job-language": "python", "--additional-python-modules": "requests>=2.31"})
`)
	writeFile(t, filepath.Join(project, "z.py"), `from aws_cdk import aws_glue as glue
modules = load_modules()
glue.CfnJob(self, "broken", default_arguments={"--job-language": "python", "--additional-python-modules": modules})
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--generate", "python-requirements", project}, &out, &stderr); code == 0 {
		t.Fatal("generation succeeded")
	}
	if !strings.Contains(stderr.String(), `job "broken"`) || !strings.Contains(stderr.String(), "unreadable dependency declaration") {
		t.Fatalf("missing contextual error: %s", &stderr)
	}
	if _, err := os.Stat(filepath.Join(project, "a.py-first.generated-requirements.txt")); !os.IsNotExist(err) {
		t.Fatal("preflight wrote the valid earlier job")
	}
}

func TestGeneratePythonCDKRespectsDetectorExclusion(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "job.py"), `from aws_cdk import aws_glue as glue
glue.CfnJob(self, "job", default_arguments={"--job-language": "python", "--additional-python-modules": "flask"})
`)
	var out, stderr bytes.Buffer
	if code := run([]string{"--disable-rule", "python.cdk.aws_glue_job.python", "--generate", "python-requirements", project}, &out, &stderr); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
	if _, err := os.Stat(filepath.Join(project, "job.py-job.generated-requirements.txt")); !os.IsNotExist(err) {
		t.Fatal("disabled detector generated a file")
	}
}
