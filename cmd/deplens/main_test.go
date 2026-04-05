package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDefaultPathWorks(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "package.json")
	writeFile(t, manifestPath, "{}")

	oldWD := mustGetwd(t)
	t.Cleanup(func() {
		mustChdir(t, oldWD)
	})
	mustChdir(t, tmpDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(nil, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "package.json") {
		t.Fatalf("expected output to include package.json, got %q", stdout.String())
	}
}

func TestRunExplicitPathWorks(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	writeFile(t, filepath.Join(projectDir, "requirements.txt"), "requests==2.0.0")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{projectDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "requirements.txt") {
		t.Fatalf("expected output to include requirements.txt, got %q", stdout.String())
	}
}

func TestRunExplicitPathDetectsRequirementsIn(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{filepath.Join("..", "..", "testdata", "sample-monorepo")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "requirements.qt6_3.in") {
		t.Fatalf("expected output to include requirements.qt6_3.in, got %q", stdout.String())
	}
}

func TestRunHidesConfirmedEmptyManifestsByDefault(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{filepath.Join("..", "..", "testdata", "python", "setup-cfg-inline-comma-unsupported")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Found 1 manifest:") {
		t.Fatalf("expected summary to include manifest count, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "- 1 confirmed empty") {
		t.Fatalf("expected summary to include confirmed empty count, got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "setup.cfg [no dependencies]") {
		t.Fatalf("expected confirmed-empty manifest to be hidden by default, got %q", stdout.String())
	}
}

func TestRunShowEmptyIncludesConfirmedEmptyManifests(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--show-empty", filepath.Join("..", "..", "testdata", "python", "setup-cfg-inline-comma-unsupported")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "setup.cfg [no dependencies]") {
		t.Fatalf("expected confirmed-empty manifest to be shown with --show-empty, got %q", stdout.String())
	}
}

func TestRunJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "pom.xml"), "<project/>")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--json", tmpDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}

	var payload struct {
		Root      string `json:"root"`
		Manifests []struct {
			Type string `json:"type"`
			Path string `json:"path"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}
	if len(payload.Manifests) != 1 || payload.Manifests[0].Type != "java" {
		t.Fatalf("unexpected manifests payload: %+v", payload.Manifests)
	}
}

func TestParseArgsShowEmptyFlag(t *testing.T) {
	cfg, err := parseArgs([]string{"--show-empty"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if !cfg.showEmpty {
		t.Fatalf("expected showEmpty to be true")
	}
}

func TestRunDetectsTerraformGluePythonSource(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "job.tf"), `
resource "aws_glue_job" "python_shell_example" {
  default_arguments = {
    "--job-language" = "python"
    "--additional-python-modules" = "scikit-learn==1.4.1.post1,pandas==2.2.1"
  }
}
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{tmpDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Found 1 manifest:") ||
		!strings.Contains(stdout.String(), "job.tf [matched]") {
		t.Fatalf("expected output to include terraform glue source, got %q", stdout.String())
	}
}

func TestRunDetectsHTMLExternalScripts(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "index.html"), `
<script src="https://cdn.jsdelivr.net/npm/dompurify@3.0.8/dist/purify.min.js"></script>
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{tmpDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "index.html [1 dep]") ||
		!strings.Contains(stdout.String(), "https://cdn.jsdelivr.net/npm/dompurify@3.0.8/dist/purify.min.js") {
		t.Fatalf("expected output to include html external script dependency, got %q", stdout.String())
	}
}

func TestRunInvalidPathReturnsNonZero(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"/definitely/missing/path"}, &stdout, &stderr)
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if stderr.Len() == 0 {
		t.Fatalf("expected error output")
	}
}

func TestRunEmptyDirectoryReturnsSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{tmpDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No manifests found.") {
		t.Fatalf("expected empty state output, got %q", stdout.String())
	}
}

func TestRunWithCustomRulesFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	writeFile(t, filepath.Join(projectDir, "deps.gradle"), "")
	rulesPath := filepath.Join(tmpDir, "rules.yaml")
	writeFile(t, rulesPath, "rules:\n  - name: gradle\n    filename-regex: '^deps\\.gradle$'\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--rules", rulesPath, projectDir}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "gradle") {
		t.Fatalf("expected output to include gradle, got %q", stdout.String())
	}
}

func TestRunMissingRulesFileReturnsNonZero(t *testing.T) {
	tmpDir := t.TempDir()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--rules", filepath.Join(tmpDir, "missing.yaml"), tmpDir}, &stdout, &stderr)
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if !strings.Contains(stderr.String(), "read rules file") {
		t.Fatalf("expected rules file error, got %q", stderr.String())
	}
}
