package analyze

import "testing"

func TestRRenvLockParserRecognizesPackagesAndEmptyLockfiles(t *testing.T) {
	parser, err := newRRenvLockParser(rRenvLockParserConfig{})
	if err != nil {
		t.Fatalf("newRRenvLockParser failed: %v", err)
	}

	t.Run("packages", func(t *testing.T) {
		result, err := parser.Analyze("renv.lock", []byte(`{
  "R": {"Version": "4.4.0"},
  "Packages": {
    "plottools": {"Package": "plottools", "Version": "2.0.0"},
    "helper": {"Version": "1.0.0"}
  }
}`))
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}
		if !result.Recognized {
			t.Fatal("expected renv lockfile to be recognized")
		}
		want := []DependencyReference{
			{Raw: "helper@1.0.0", Name: "helper", Version: "1.0.0"},
			{Raw: "plottools@2.0.0", Name: "plottools", Version: "2.0.0"},
		}
		if !equalDependencies(result.Dependencies, want) {
			t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
		}
		if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
			t.Fatalf("unexpected analysis: %+v", result.Analysis)
		}
	})

	t.Run("empty", func(t *testing.T) {
		result, err := parser.Analyze("renv.lock", []byte(`{
  "R": {"Version": "4.1.0"},
  "Packages": {}
}`))
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}
		if !result.Recognized {
			t.Fatal("expected renv lockfile to be recognized")
		}
		if result.Dependencies != nil {
			t.Fatalf("expected no dependencies, got %+v", result.Dependencies)
		}
		if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
			t.Fatalf("unexpected analysis: %+v", result.Analysis)
		}
	})
}
