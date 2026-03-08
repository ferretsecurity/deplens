package analyze

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectManifestMatchesSupportedFiles(t *testing.T) {
	testCases := []struct {
		name string
		want ManifestType
	}{
		{name: "requirements.txt", want: RequirementsTXT},
		{name: "uv.lock", want: UVLock},
		{name: "package.json", want: PackageJSON},
		{name: "yarn.lock", want: YarnLock},
		{name: "pom.xml", want: PomXML},
	}

	for _, tc := range testCases {
		got, ok := detectManifest(tc.name)
		if !ok {
			t.Fatalf("expected %s to be detected", tc.name)
		}
		if got != tc.want {
			t.Fatalf("expected type %q, got %q", tc.want, got)
		}
	}
}

func TestDetectManifestIgnoresSimilarNames(t *testing.T) {
	testCases := []string{
		"requirements-dev.txt",
		"package-lock.json",
		"pom.xml.backup",
		"yarn.lock.old",
		"uv.lock.json",
	}

	for _, tc := range testCases {
		if _, ok := detectManifest(tc); ok {
			t.Fatalf("expected %s to be ignored", tc)
		}
	}
}

func TestScanFindsNestedManifestsSortedByRelativePath(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "b", "requirements.txt"), "")
	mustWriteFile(t, filepath.Join(root, "a", "package.json"), "")

	result, err := Scan(root, nil)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Path != "a/package.json" || result.Manifests[1].Path != "b/requirements.txt" {
		t.Fatalf("unexpected manifest order: %+v", result.Manifests)
	}
}

func TestScanSkipsIgnoredDirectories(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "node_modules", "package.json"), "")
	mustWriteFile(t, filepath.Join(root, "src", "package.json"), "")

	result, err := Scan(root, []string{"node_modules"})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Path != "src/package.json" {
		t.Fatalf("unexpected manifest path: %+v", result.Manifests[0])
	}
}

func TestScanOverrideIgnoreListChangesTraversalBehavior(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "vendor", "pom.xml"), "")

	result, err := Scan(root, nil)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected vendor manifest to be found without ignore list, got %d", len(result.Manifests))
	}

	result, err = Scan(root, []string{"vendor"})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected vendor manifest to be ignored, got %+v", result.Manifests)
	}
}

func TestScanRejectsFilePath(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "package.json")
	mustWriteFile(t, filePath, "{}")

	_, err := Scan(filePath, nil)
	if err == nil {
		t.Fatalf("expected error for file path")
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}
