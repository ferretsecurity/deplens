package analyze

import (
	"slices"
	"testing"
)

func TestPDMLockParserExtractsPackages(t *testing.T) {
	parser, err := newPDMLockParser(pdmLockParserConfig{})
	if err != nil {
		t.Fatalf("newPDMLockParser failed: %v", err)
	}

	result, err := parser.Analyze("pdm.lock", []byte(`
[metadata]
lock_version = "4.5.0"

[[metadata.targets]]
requires_python = ">=3.9"

[[package]]
name = "demo-api"
version = "2.4.0"
groups = ["default"]

[[package]]
name = "demo-core"
version = "1.3.0"
`))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if !result.Recognized {
		t.Fatal("expected PDM lockfile to be recognized")
	}
	if got, want := dependencyNames(result.Dependencies), []string{"demo-api==2.4.0", "demo-core==1.3.0"}; !slices.Equal(got, want) {
		t.Fatalf("unexpected dependencies: got %+v want %v", got, want)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
		t.Fatalf("unexpected analysis: %+v", result.Analysis)
	}
}

func TestPDMLockParserReturnsConclusiveEmptyForMetadataOnlyFile(t *testing.T) {
	parser, err := newPDMLockParser(pdmLockParserConfig{})
	if err != nil {
		t.Fatalf("newPDMLockParser failed: %v", err)
	}

	result, err := parser.Analyze("pdm.lock", []byte("[metadata]\nlock_version = \"4.4.1\"\n"))
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if !result.Recognized {
		t.Fatal("expected PDM lockfile to be recognized")
	}
	if result.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", result.Dependencies)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("unexpected analysis: %+v", result.Analysis)
	}
}
