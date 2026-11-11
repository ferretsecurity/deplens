package analyze

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestScanRequirementsOriginClassification(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "requirements.txt"), ".\n./local-lib\n-e .\n~/lib\ndemo @ https://example.test/demo.whl\nhttps://example.test/demo.whl\nrequests>=2\nnot a requirement\n")
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("sources: %+v", result.Sources)
	}
	source := result.Sources[0]
	if source.Analysis.Extraction != ExtractionPartial || len(source.Diagnostics) != 1 || source.Diagnostics[0].Code != "unsupported-python-requirement" {
		t.Fatalf("analysis: %+v diagnostics: %+v", source.Analysis, source.Diagnostics)
	}
	want := []OriginKind{OriginPath, OriginPath, OriginPath, OriginPath, OriginURL, OriginURL, OriginRegistry, ""}
	if len(source.Dependencies) != len(want) {
		t.Fatalf("dependencies: %+v", source.Dependencies)
	}
	for i, dep := range source.Dependencies {
		if dep.OriginKind != want[i] {
			t.Errorf("%q: origin %q want %q", dep.Raw, dep.OriginKind, want[i])
		}
		if i == 6 {
			if dep.VERS != "vers:pypi/>=2" {
				t.Errorf("registry VERS: %+v", dep)
			}
			continue
		}
		encoded, err := json.Marshal(dep)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"version", "version_constraint", "vers"} {
			if _, ok := fields[key]; ok {
				t.Errorf("%q has unexpected %s: %s", dep.Raw, key, encoded)
			}
		}
		if i != 4 && dep.Name != "" {
			t.Errorf("invented name: %+v", dep)
		}
	}
	if source.Dependencies[0].Attributes["source_path"] != "." || source.Dependencies[2].Attributes["editable"] != "true" || source.Dependencies[4].Name != "demo" {
		t.Fatalf("source metadata: %+v", source.Dependencies)
	}
	if source.Dependencies[7].Raw != "not a requirement" {
		t.Fatal("raw requirement lost")
	}
}
