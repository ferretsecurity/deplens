package main

import (
	"bytes"
	"encoding/json"
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
