package fixturecollector

import (
	"strings"
	"testing"
)

func TestNewProgressInitializesEligibleDetectorsInInventoryOrder(t *testing.T) {
	inventory := []Detector{
		{ID: "extracting", Capabilities: []string{"select", "extract"}},
		{ID: "selector-only", Form: "manifest", Roles: []string{"declaration"}, Capabilities: []string{"select"}},
		{ID: "presence-only", Form: "lockfile", Roles: []string{"resolution"}, Capabilities: []string{"select", "recognize", "assess-presence"}},
	}

	progress, err := NewProgress(inventory)
	if err != nil {
		t.Fatalf("NewProgress() error = %v", err)
	}
	if len(progress.Detectors) != 2 {
		t.Fatalf("eligible detector count = %d, want 2", len(progress.Detectors))
	}
	if got := progress.Detectors[0]; got.ID != "selector-only" || got.State != StatePending || got.IterationLimit != 7 {
		t.Fatalf("first detector = %+v", got)
	}
	if got := progress.Detectors[1]; got.ID != "presence-only" || got.State != StatePending {
		t.Fatalf("second detector = %+v", got)
	}
	if progress.Settings.MaxSearchQueries != 8 || progress.Settings.MaxCandidateInspections != 40 {
		t.Fatalf("unexpected default settings: %+v", progress.Settings)
	}
	if progress.Settings.MinExamples != 3 || progress.Settings.MaxExamples != 5 || progress.Settings.MaxExampleBytes != 2<<20 {
		t.Fatalf("unexpected corpus example defaults: %+v", progress.Settings)
	}
	if progress.InventoryFingerprint == "" {
		t.Fatal("inventory fingerprint is empty")
	}
}

func TestInventoryFingerprintIncludesSemanticAnalyzerConfiguration(t *testing.T) {
	baseline := []Detector{{
		ID: "source", Form: "manifest", Roles: []string{"declaration"},
		FilenameRegex: "^source$", Analyzer: "json", AnalyzerConfig: "{\"queries\":[\"dependencies\"]}",
		Capabilities: []string{"select", "recognize", "assess-presence"},
	}}
	changed := append([]Detector(nil), baseline...)
	changed[0].AnalyzerConfig = "{\"queries\":[\"devDependencies\"]}"

	baselineFingerprint, err := InventoryFingerprint(baseline)
	if err != nil {
		t.Fatalf("InventoryFingerprint() error = %v", err)
	}
	changedFingerprint, err := InventoryFingerprint(changed)
	if err != nil {
		t.Fatalf("InventoryFingerprint() error = %v", err)
	}
	if baselineFingerprint == changedFingerprint {
		t.Fatal("semantic analyzer configuration did not change inventory fingerprint")
	}
}

func TestInventoryFingerprintIgnoresCapabilityAndRoleOrdering(t *testing.T) {
	first := []Detector{{
		ID: "source", Form: "manifest", Roles: []string{"declaration", "constraint"},
		Capabilities: []string{"select", "recognize", "assess-presence"},
	}}
	second := []Detector{{
		ID: "source", Form: "manifest", Roles: []string{"constraint", "declaration"},
		Capabilities: []string{"assess-presence", "select", "recognize"},
	}}

	firstFingerprint, err := InventoryFingerprint(first)
	if err != nil {
		t.Fatalf("InventoryFingerprint() error = %v", err)
	}
	secondFingerprint, err := InventoryFingerprint(second)
	if err != nil {
		t.Fatalf("InventoryFingerprint() error = %v", err)
	}
	if firstFingerprint != secondFingerprint {
		t.Fatalf("same semantic inventory produced different fingerprints: %s != %s", firstFingerprint, secondFingerprint)
	}
}

func TestProgressYAMLRejectsUnknownFieldsAndInvalidDetectorState(t *testing.T) {
	for _, text := range []string{
		"schema-version: 1\nunexpected: value\n",
		"schema-version: 1\ninventory-fingerprint: zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz\nsettings: {}\ndetectors: []\n",
		"schema-version: 1\ninventory-fingerprint: 0123456789012345678901234567890123456789012345678901234567890123\nsettings:\n  max-search-queries: 8\n  max-candidate-inspections: 40\n  max-example-bytes: 1\n  allowed-licenses: [MIT]\ndetectors:\n  - id: x\n    state: impossible\n    iteration-limit: 7\n",
		"schema-version: 1\ninventory-fingerprint: 0123456789012345678901234567890123456789012345678901234567890123\nsettings:\n  min-examples: 3\n  max-examples: 5\n  max-search-queries: 8\n  max-candidate-inspections: 40\n  max-example-bytes: 1\n  allowed-licenses: [MIT]\ndetectors: []\n---\nextra: document\n",
		"schema-version: 1\ninventory-fingerprint: 0123456789012345678901234567890123456789012345678901234567890123\nsettings:\n  min-examples: 3\n  max-examples: 5\n  max-search-queries: 8\n  max-candidate-inspections: 40\n  max-example-bytes: 1\n  allowed-licenses: [MIT]\ndetectors:\n  - id: x\n    state: pending\n    iteration-limit: 7\n    blocked-reason: no\n",
	} {
		if _, err := ParseProgress([]byte(text)); err == nil {
			t.Fatalf("ParseProgress(%q) succeeded", strings.ReplaceAll(text, "\n", " "))
		}
	}
}

func TestProgressRejectsInvalidStateCombinations(t *testing.T) {
	progress, err := NewProgress([]Detector{{ID: "source", Capabilities: []string{"select"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, detector := range []DetectorProgress{
		{ID: "source", State: StatePending, IterationLimit: 7, CompletedIterations: 1},
		{ID: "source", State: StateInProgress, IterationLimit: 7, CompletedIterations: 7},
		{ID: "source", State: StateComplete, IterationLimit: 7},
	} {
		progress.Detectors = []DetectorProgress{detector}
		if err := progress.Validate(); err == nil {
			t.Fatalf("Validate() succeeded for invalid detector state: %+v", detector)
		}
	}
}

func TestProgressRejectsChangedInventory(t *testing.T) {
	inventory := []Detector{{ID: "one", Capabilities: []string{"select"}}}
	progress, err := NewProgress(inventory)
	if err != nil {
		t.Fatal(err)
	}
	changedInventory := []Detector{{ID: "one", Capabilities: []string{"select", "extract"}}}
	changedFingerprint, err := InventoryFingerprint(changedInventory)
	if err != nil {
		t.Fatal(err)
	}
	progress.InventoryFingerprint = changedFingerprint
	if err := progress.ValidateInventory(changedInventory); err == nil || !strings.Contains(err.Error(), "detector coverage") {
		t.Fatalf("ValidateInventory() error = %v, want detector coverage disagreement", err)
	}
}

func TestSelectDetectorPrefersInProgressThenDocumentOrder(t *testing.T) {
	progress := Progress{Detectors: []DetectorProgress{
		{ID: "pending-first", State: StatePending, IterationLimit: 7},
		{ID: "in-progress", State: StateInProgress, IterationLimit: 7},
	}}
	if got := progress.SelectDetector(""); got == nil || got.ID != "in-progress" {
		t.Fatalf("SelectDetector() = %+v, want in-progress", got)
	}
}
