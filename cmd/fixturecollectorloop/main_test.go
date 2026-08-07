package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ferretsecurity/deplens/internal/analyze"
	"github.com/ferretsecurity/deplens/internal/fixturecollector"
)

func TestInitializeProgressCreatesOnlyReviewedProgressDocument(t *testing.T) {
	root := t.TempDir()
	progressPath := filepath.Join(root, "testdata", "corpus", "collection-progress.yaml")
	var stdout, stderr bytes.Buffer

	if status := run([]string{"initialize-progress", "--root", root}, &stdout, &stderr); status != 0 {
		t.Fatalf("run() status = %d, stderr = %s", status, stderr.String())
	}
	data, err := os.ReadFile(progressPath)
	if err != nil {
		t.Fatalf("read progress: %v", err)
	}
	progress, err := fixturecollector.ParseProgress(data)
	if err != nil {
		t.Fatalf("parse progress: %v", err)
	}
	if len(progress.Detectors) == 0 {
		t.Fatal("initialized progress has no eligible detectors")
	}
	if progress.Settings.MinExamples != 3 || progress.Settings.MaxExamples != 5 {
		t.Fatalf("example range = %d-%d, want 3-5", progress.Settings.MinExamples, progress.Settings.MaxExamples)
	}
	inventory, err := analyze.DefaultDetectorInventory()
	if err != nil {
		t.Fatalf("load detector inventory: %v", err)
	}
	expected := make(map[string]bool)
	for _, detector := range inventory {
		if !contains(detector.Capabilities, "extract") {
			expected[detector.ID] = true
		}
	}
	if len(progress.Detectors) != len(expected) {
		t.Fatalf("eligible detector count = %d, want %d", len(progress.Detectors), len(expected))
	}
	for _, detector := range progress.Detectors {
		if !expected[detector.ID] {
			t.Fatalf("unexpected extracting detector in collection progress: %s", detector.ID)
		}
		delete(expected, detector.ID)
	}
	if len(expected) != 0 {
		t.Fatalf("missing eligible detectors: %v", expected)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}

	if status := run([]string{"initialize-progress", "--root", root}, &stdout, &stderr); status != 1 {
		t.Fatalf("second run status = %d, want 1", status)
	}
	entries, err := os.ReadDir(filepath.Join(root, "testdata", "corpus"))
	if err != nil {
		t.Fatalf("read corpus directory: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "collection-progress.yaml" {
		t.Fatalf("initialization created files other than collection progress: %v", entries)
	}
}

func TestInitializeProgressRefusesRulesAndCoverageDisagreement(t *testing.T) {
	root := t.TempDir()
	inventory := []analyze.DetectorInventoryEntry{{ID: "selector-only", Capabilities: []string{"select"}}}
	var stdout, stderr bytes.Buffer
	status := runWithInventory(
		[]string{"initialize-progress", "--root", root}, &stdout, &stderr,
		func() ([]analyze.DetectorInventoryEntry, error) { return inventory, nil },
		func(detectors []fixturecollector.Detector) (fixturecollector.Progress, error) {
			progress, err := fixturecollector.NewProgress(detectors)
			if err != nil {
				return fixturecollector.Progress{}, err
			}
			progress.Detectors = nil
			return progress, nil
		},
	)
	if status != 1 {
		t.Fatalf("runWithInventory() status = %d, want 1; stderr = %s", status, stderr.String())
	}
	if !strings.Contains(stderr.String(), "detector coverage") {
		t.Fatalf("stderr = %q, want detector coverage disagreement", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "testdata", "corpus", "collection-progress.yaml")); !os.IsNotExist(err) {
		t.Fatalf("progress document exists after rejected initialization; stat error = %v", err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
