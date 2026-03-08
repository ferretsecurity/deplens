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
	if len(payload.Manifests) != 1 || payload.Manifests[0].Type != "pom.xml" {
		t.Fatalf("unexpected manifests payload: %+v", payload.Manifests)
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

