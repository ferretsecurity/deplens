//go:build linux

package analyze

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

// Run all fixtures in a subprocess so even a regression to blocking FIFO reads
// can be killed without leaving the test runner or its cleanup stuck.
func TestScanSpecialFiles(t *testing.T) {
	if os.Getenv("DEPLENS_SPECIAL_FILES_TEST_CHILD") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestScanSpecialFiles$", "-test.v")
		cmd.Env = append(os.Environ(), "DEPLENS_SPECIAL_FILES_TEST_CHILD=1", "DEPLENS_SPECIAL_FILES_TEST_ROOT="+t.TempDir())
		output, err := cmd.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("special-file scan did not terminate: %v\n%s", ctx.Err(), output)
		}
		if err != nil {
			t.Fatalf("special-file scan failed: %v\n%s", err, output)
		}
		return
	}

	fixtureRoot := func(t *testing.T) string {
		t.Helper()
		root, err := os.MkdirTemp(os.Getenv("DEPLENS_SPECIAL_FILES_TEST_ROOT"), "scan-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(root) })
		return root
	}

	fifo := func(t *testing.T, path string) {
		t.Helper()
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	socket := func(t *testing.T, path string) {
		t.Helper()
		listener, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { listener.Close() })
	}
	symlink := func(t *testing.T, target, path string) {
		t.Helper()
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
	}
	fixtures := []struct {
		name   string
		create func(*testing.T, string)
	}{
		{"fifo", fifo},
		{"socket", socket},
		{"symlink-fifo", func(t *testing.T, path string) {
			target := filepath.Join(filepath.Dir(path), "pipe")
			fifo(t, target)
			symlink(t, target, path)
		}},
		{"symlink-device", func(t *testing.T, path string) { symlink(t, "/dev/zero", path) }},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			// Cover analyzer reads, presence-only sources, and uv policy reads.
			for _, filename := range []string{"requirements.txt", "bun.lockb", "pyproject.toml"} {
				t.Run(filename, func(t *testing.T) {
					root := fixtureRoot(t)
					ruleset := mustLoadDefaultRules(t)
					want, err := Scan(root, nil, ruleset)
					if err != nil {
						t.Fatal(err)
					}
					fixture.create(t, filepath.Join(root, filename))
					result, err := Scan(root, nil, ruleset)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(result, want) {
						t.Fatalf("special file produced scan results: %#v", result)
					}
				})
			}
			t.Run("project-evidence", func(t *testing.T) {
				root := fixtureRoot(t)
				mustWriteFile(t, filepath.Join(root, "Cargo.toml"), "[package]\nname = 'example'\nversion = '0.1.0'\n[dependencies]\nserde = '1'\n")
				if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
					t.Fatal(err)
				}
				fixture.create(t, filepath.Join(root, "src", "main.rs"))
				result, err := Scan(root, nil, mustLoadDefaultRules(t))
				if err != nil {
					t.Fatal(err)
				}
				runs := filterCheckRuns(result.CheckRuns, "rust-cargo-lockfile-missing-for-application")
				if len(runs) != 1 || runs[0].Status != CheckSkipped || runs[0].ReasonCode != "project-role-unknown" {
					t.Fatalf("special file became Cargo application evidence: %#v", runs)
				}
			})
		})
	}
	t.Run("codeowners-fifo", func(t *testing.T) {
		root := fixtureRoot(t)
		mustWriteFile(t, filepath.Join(root, "requirements.txt"), "requests==2.32.3\n")
		fifo(t, filepath.Join(root, "CODEOWNERS"))
		result, err := Scan(root, nil, mustLoadDefaultRules(t))
		if err != nil {
			t.Fatal(err)
		}
		runs := filterCheckRuns(result.CheckRuns, "dependency-source-codeowners-missing")
		if len(runs) != 1 || runs[0].Status != CheckFailed {
			t.Fatalf("expected CODEOWNERS non-regular-file failure: %#v", runs)
		}
	})
}
