package analyze

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRegularAndBrokenSymlinks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "dependencies")
	mustWriteFile(t, target, "requests==2.32.3\n")
	for _, fixture := range []struct{ name, target string }{
		{"requirements.txt", target},
		{"broken/requirements.txt", filepath.Join(root, "missing")},
	} {
		link := filepath.Join(root, fixture.name)
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(fixture.target, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != 2 {
		t.Fatalf("expected two sources, got %#v", result.Sources)
	}
	broken, regular := result.Sources[0], result.Sources[1]
	if broken.Path != "broken/requirements.txt" || broken.Analysis != failedAnalysis() || len(broken.Diagnostics) != 1 || broken.Diagnostics[0].Code != "source-read-failed" {
		t.Fatalf("broken symlink lost read diagnostics: %#v", broken)
	}
	if regular.Path != "requirements.txt" || regular.Analysis.Extraction != ExtractionComplete || len(regular.Dependencies) != 1 || regular.Dependencies[0].Name != "requests" {
		t.Fatalf("regular symlink lost dependencies or relative path: %#v", regular)
	}
}

func TestScanDoesNotTraverseSymlinkDirectories(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	mustWriteFile(t, filepath.Join(target, "requirements.txt"), "requests==2.32.3\n")
	if err := os.Symlink(target, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("traversed a symlink directory: %#v", result.Sources)
	}
}

func TestScanRegularSymlinkProvidesProjectEvidence(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "Cargo.toml"), "[package]\nname = 'example'\nversion = '0.1.0'\n[dependencies]\nserde = '1'\n")
	target := filepath.Join(t.TempDir(), "main")
	mustWriteFile(t, target, "fn main() {}\n")
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "src", "main.rs")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if findings := findingsForCheck(result.Findings, "rust-cargo-lockfile-missing-for-application"); len(findings) != 1 {
		t.Fatalf("regular symlink lost Cargo application evidence: %#v", findings)
	}
}

func TestScanUnreadableRegularFileRetainsDiagnostics(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "requirements.txt")
	mustWriteFile(t, path, "requests==2.32.3\n")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o600) })
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("current user can read files without read permissions")
	}
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != 1 || result.Sources[0].Analysis != failedAnalysis() || len(result.Sources[0].Diagnostics) != 1 || result.Sources[0].Diagnostics[0].Code != "source-read-failed" {
		t.Fatalf("unreadable regular file lost read diagnostics: %#v", result.Sources)
	}
}
