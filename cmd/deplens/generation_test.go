package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
