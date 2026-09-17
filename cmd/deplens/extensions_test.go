package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ferretsecurity/deplens/internal/analyze"
)

func extensionDetector(id, selector string) string {
	return fmt.Sprintf("  - id: %s\n    form: manifest\n    roles: [declaration]\n    filename-regex: '%s'\n", id, selector)
}

func extensionCheck(id string) string {
	return fmt.Sprintf("  - id: %s\n    summary: Go checksum missing\n    severity: medium\n    evaluator: {type: go-sum-missing}\n    remediation: Run go mod tidy.\n", id)
}

func runComposedJSON(t *testing.T, args ...string) analyze.ScanResult {
	t.Helper()
	var out, stderr bytes.Buffer
	if code := run(append([]string{"--json"}, args...), &out, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}
	var result analyze.ScanResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRunRuleExtensions(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	writeFile(t, filepath.Join(project, "go.mod"), "module example.com/app\n\ngo 1.23\n\nrequire example.com/dependency v1.0.0\n")
	writeFile(t, filepath.Join(project, "company.deps"), "anything")
	base := filepath.Join(dir, "base.yaml")
	writeFile(t, base, "rules:\n  - id: go-mod\n    form: manifest\n    roles: [declaration]\n    filename-regex: '^go\\.mod$'\n    analyzer: {type: go-mod}\n")
	extension := filepath.Join(dir, "company,rules.yaml") // A comma belongs to the filename.
	writeFile(t, extension, "rules:\n"+extensionDetector("company", "^company\\.deps$")+"checks:\n"+extensionCheck("company"))
	checks := filepath.Join(dir, "checks.yaml")
	writeFile(t, checks, "checks:\n"+extensionCheck("aaa-company"))

	for _, custom := range []bool{false, true} {
		t.Run(fmt.Sprintf("custom-base=%v", custom), func(t *testing.T) {
			args := []string{}
			if custom {
				args = append(args, "--rules", base)
			}
			baseline := runComposedJSON(t, append(args, project)...)
			if len(baseline.Sources) != 1 {
				t.Fatalf("baseline sources: %+v", baseline.Sources)
			}
			if custom && len(baseline.CheckRuns) != 0 {
				t.Fatalf("replacement retained default checks: %+v", baseline.CheckRuns)
			}
			if !custom && len(baseline.CheckRuns) == 0 {
				t.Fatal("default checks missing")
			}
			args = append(args, "--extend-rules", extension, "--extend-rules", checks)
			result := runComposedJSON(t, append(args, project)...)
			if len(result.Sources) != 2 || result.Sources[0].Detector != "company" || result.Sources[1].Detector != "go-mod" {
				t.Fatalf("sources: %+v", result.Sources)
			}
			var ids []string
			for _, finding := range result.Findings {
				ids = append(ids, string(finding.CheckID))
			}
			if !slices.Contains(ids, "company") || !slices.Contains(ids, "aaa-company") || !slices.IsSorted(ids) {
				t.Fatalf("findings: %+v", result.Findings)
			}
			var out, stderr bytes.Buffer
			if code := run(append(args, project), &out, &stderr); code != 0 || !strings.Contains(out.String(), "company.deps") || !strings.Contains(out.String(), "Go checksum missing") {
				t.Fatalf("human exit=%d output=%s stderr=%s", code, &out, &stderr)
			}
		})
	}
}

func TestRunExtensionPrecedence(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	writeFile(t, filepath.Join(project, "deps"), "{}")
	writeFile(t, filepath.Join(project, "package.json"), "{}")
	first, second := filepath.Join(dir, "first.yaml"), filepath.Join(dir, "second.yaml")
	writeFile(t, first, "rules:\n"+extensionDetector("first", "^deps$")+extensionDetector("later-in-file", "^deps$")+extensionDetector("package-fallback", "^package\\.json$"))
	writeFile(t, second, "rules:\n"+extensionDetector("second", "^deps$"))
	for _, order := range [][]string{{first, second}, {second, first}} {
		result := runComposedJSON(t, "--extend-rules", order[0], "--extend-rules", order[1], project)
		want := strings.TrimSuffix(filepath.Base(order[0]), ".yaml")
		if len(result.Sources) != 2 || string(result.Sources[0].Detector) != want || result.Sources[1].Detector != "js" {
			t.Fatalf("order %v: %+v", order, result.Sources)
		}
	}
	// Paths are relative to the working directory, not to the scan root.
	old := mustGetwd(t)
	t.Cleanup(func() { mustChdir(t, old) })
	mustChdir(t, dir)
	result := runComposedJSON(t, "--extend-rules", "first.yaml", "project")
	if result.Sources[0].Detector != "first" {
		t.Fatalf("sources: %+v", result.Sources)
	}
}

func TestRunInvalidExtensionsFailBeforeScan(t *testing.T) {
	rule := "rules:\n" + extensionDetector("bad-rule", "^deps$")
	check := "checks:\n" + extensionCheck("bad-check")
	for _, tc := range []struct {
		name, document string
		errors         []string
	}{
		{"empty", "{}", []string{"at least one"}},
		{"multiple-documents", rule + "---\n" + check, []string{"exactly one YAML document"}},
		{"unknown-top", "surprise: true", []string{"surprise"}},
		{"unknown-rule", rule + "    surprise: true\n", []string{"bad-rule", "surprise"}},
		{"selector", strings.ReplaceAll(rule, "^deps$", "["), []string{"bad-rule", "filename-regex"}},
		{"analyzer", rule + "    analyzer: {type: nope}\n", []string{"bad-rule", "analyzer"}},
		{"analyzer-field", rule + "    analyzer: {type: go-mod, surprise: true}\n", []string{"bad-rule", "surprise"}},
		{"unknown-check", check + "    surprise: true\n", []string{"bad-check", "surprise"}},
		{"evaluator", strings.ReplaceAll(check, "go-sum-missing", "nope"), []string{"bad-check", "evaluator"}},
		{"evaluator-field", strings.ReplaceAll(check, "type: go-sum-missing", "type: go-sum-missing, surprise: true"), []string{"bad-check", "unknown"}},
		{"duplicate-default-rule", "rules:\n" + extensionDetector("go-mod", "^deps$"), []string{"go-mod", "embedded default rules", "duplicate"}},
		{"duplicate-default-check", "checks:\n" + extensionCheck("javascript-npm-lockfile-missing"), []string{"javascript-npm-lockfile-missing", "embedded default rules", "duplicate"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "extension.yaml")
			writeFile(t, path, tc.document)
			var out, stderr bytes.Buffer
			code := run([]string{"--extend-rules", path, filepath.Join(dir, "nonexistent-scan-root")}, &out, &stderr)
			if code == 0 || out.Len() != 0 {
				t.Fatalf("exit=%d output=%s", code, &out)
			}
			for _, part := range append(tc.errors, path) {
				if !strings.Contains(stderr.String(), part) {
					t.Fatalf("missing %q in %s", part, &stderr)
				}
			}
		})
	}
}

func TestRunDuplicateExtensions(t *testing.T) {
	for _, document := range []string{"rules:\n" + extensionDetector("duplicate", "^deps$"), "checks:\n" + extensionCheck("duplicate")} {
		dir := t.TempDir()
		first, second := filepath.Join(dir, "first.yaml"), filepath.Join(dir, "second.yaml")
		writeFile(t, first, document)
		writeFile(t, second, document)
		var out, stderr bytes.Buffer
		if code := run([]string{"--extend-rules", first, "--extend-rules", second, dir}, &out, &stderr); code == 0 {
			t.Fatal("duplicate accepted")
		}
		for _, part := range []string{"duplicate", first, second} {
			if !strings.Contains(stderr.String(), part) {
				t.Fatalf("missing %q in %s", part, &stderr)
			}
		}
	}
}

func TestRunExtensionHelpAndArguments(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := run([]string{"--help"}, &out, &stderr); code != 0 || !strings.Contains(out.String(), "-extend-rules") || !strings.Contains(out.String(), "repeatable") {
		t.Fatalf("help: %s, %s", &out, &stderr)
	}
	for _, args := range [][]string{{"--extend-rules"}, {".", "--extend-rules", "company.yaml"}, {"--extend-rules", "/does-not-exist.yaml"}} {
		out.Reset()
		stderr.Reset()
		if code := run(args, &out, &stderr); code == 0 || stderr.Len() == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
}
