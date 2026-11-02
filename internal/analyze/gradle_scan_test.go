package analyze

import (
	"path/filepath"
	"testing"
)

func TestScanGradleVersionCatalogDoesNotNormalizeRichVersions(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "java", "gradle-version-catalog-rich"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected one catalog, got %+v", result.Sources)
	}
	source := result.Sources[0]
	if source.Detector != "java-gradle-version-catalog" || source.Analysis.Extraction != ExtractionPartial {
		t.Fatalf("expected partial version catalog, got %+v", source)
	}
	if len(source.Dependencies) != 9 || len(source.Diagnostics) != 8 {
		t.Fatalf("expected all aliases and rich-version warnings, got %+v", source)
	}
	for _, dependency := range source.Dependencies {
		if dependency.PackageType != "maven" {
			t.Errorf("unexpected package type: %+v", dependency)
		}
		if dependency.Attributes["alias"] == "simple" {
			if dependency.VersionConstraint != "[1.5]" || dependency.VERS != "vers:maven/1.5" {
				t.Errorf("simple versions should still normalize: %+v", dependency)
			}
			continue
		}
		if dependency.Version != "" || dependency.VersionConstraint != "" || dependency.VERS != "" {
			t.Errorf("rich version acquired a shared version or VERS: %+v", dependency)
		}
	}
}
