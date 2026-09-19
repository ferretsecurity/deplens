package main

import (
	"bytes"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ferretsecurity/deplens/internal/analyze"
)

func TestRunPresetsRepresentativeSources(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"dependencies":{"a":"1"}}`)
	writeFile(t, filepath.Join(dir, "pyproject.toml"), "[project]\nname='app'\ndependencies=['a']\n")
	writeFile(t, filepath.Join(dir, "index.html"), `<script src="https://example.com/a.js"></script>`)
	for _, name := range []string{"snyk", "socket"} {
		result := runComposedJSON(t, "--exclude-preset", name, dir)
		var ids []string
		for _, source := range result.Sources {
			ids = append(ids, string(source.Detector))
		}
		slices.Sort(ids)
		expected := []string{"html-external-scripts"}
		if name == "snyk" {
			expected = append(expected, "python-pyproject")
		}
		if !slices.Equal(ids, expected) {
			t.Fatalf("%s: %v", name, ids)
		}
		assertDisabledCheck(t, result, "javascript-npm-lockfile-missing", "js")
	}
}

func TestRunPresetsCompleteComposition(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	for _, file := range []string{"a.input", "b.input", "covered.input", "removed.input"} {
		writeFile(t, filepath.Join(project, file), "data")
	}
	base := filepath.Join(dir, "base.yaml")
	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")
	writeFile(t, base, "rules:\n"+extensionDetector("go-mod", "^covered\\.input$")+extensionDetector("first", "^a\\.input$")+"checks:\n"+extensionCheck("z-affected")+extensionCheck("disabled"))
	writeFile(t, first, "rules:\n"+extensionDetector("second", "^b\\.input$")+extensionDetector("removed", "^removed\\.input$")+"checks:\n"+extensionCheck("a-affected"))
	writeFile(t, second, "rules:\n"+extensionDetector("later", "^[ab]\\.input$")+"checks:\n  - id: ownership\n    summary: Ownership\n    severity: medium\n    evaluator: {type: dependency-source-codeowners}\n    remediation: Add owners.\n")
	flags := []string{"--rules", base, "--extend-rules", first, "--extend-rules", second}
	exclusions := []string{"--exclude-preset", "snyk", "--exclude-preset", "socket", "--exclude-preset", "snyk", "--disable-rule", "go-mod", "--disable-rule", "removed", "--disable-rule", "removed", "--disable-check", "disabled", "--disable-check", "disabled"}
	var previous analyze.ScanResult
	for i, args := range [][]string{append(slices.Clone(flags), exclusions...), append(slices.Clone(exclusions), flags...)} {
		args = append(args, project)
		result := runComposedJSON(t, args...)
		if len(result.Sources) != 2 || result.Sources[0].Detector != "first" || result.Sources[1].Detector != "second" {
			t.Fatalf("sources: %+v", result.Sources)
		}
		assertDisabledCheck(t, result, "a-affected", "go-mod")
		assertDisabledCheck(t, result, "z-affected", "go-mod")
		if !slices.IsSortedFunc(result.CheckRuns, func(a, b analyze.CheckRun) int { return strings.Compare(string(a.CheckID), string(b.CheckID)) }) {
			t.Fatalf("unstable check ordering: %+v", result.CheckRuns)
		}
		for _, r := range result.CheckRuns {
			if r.CheckID == "disabled" {
				t.Fatal(r)
			}
		}
		if len(result.Findings) != 2 {
			t.Fatalf("unrelated ownership findings: %+v", result.Findings)
		}
		for _, f := range result.Findings {
			if f.CheckID != "ownership" {
				t.Fatal(f)
			}
		}
		if i > 0 && !reflect.DeepEqual(previous, result) {
			t.Fatal("flag order changed output")
		}
		previous = result
		var out, stderr bytes.Buffer
		if code := run(args, &out, &stderr); code != 0 {
			t.Fatalf("%d: %s", code, &stderr)
		}
		for _, text := range []string{"a.input", "b.input", "[skipped] a-affected", "[skipped] z-affected", "ownership", "prerequisite detectors disabled: go-mod"} {
			if !strings.Contains(out.String(), text) {
				t.Fatalf("missing %s in %s", text, &out)
			}
		}
		for _, text := range []string{"covered.input", "removed.input", "[skipped] disabled"} {
			if strings.Contains(out.String(), text) {
				t.Fatalf("unexpected %s in %s", text, &out)
			}
		}
	}
}

func TestRunPresetValidationAndHelp(t *testing.T) {
	for _, name := range []string{"Snyk", "socket,snyk", "*", "", "unknown"} {
		var out, stderr bytes.Buffer
		if code := run([]string{"--exclude-preset", name, "/missing/scan/path"}, &out, &stderr); code == 0 || !strings.Contains(stderr.String(), "unknown coverage preset") || out.Len() != 0 {
			t.Fatalf("%q: %d %s", name, code, &stderr)
		}
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"--help"}, &out, &stderr); code != 0 {
		t.Fatal(code)
	}
	for _, flag := range []string{"extend-rules", "disable-rule", "exclude-preset", "disable-check", "snyk", "socket"} {
		if !strings.Contains(out.String(), flag) {
			t.Fatalf("missing %s", flag)
		}
	}
}
