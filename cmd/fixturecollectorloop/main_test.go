package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeAgent struct {
	write   func(Iteration) error
	outcome Outcome
}

func (f fakeAgent) Run(iteration Iteration) (Outcome, error) {
	return f.outcome, f.write(iteration)
}

var acceptedOutcome = Outcome{Result: "accepted", Added: []string{"owner-repo-abc123/project/dependencies.txt", "owner-repo-abc123/provenance.yaml"}}

func provenance(detectorID, sourcePath string, contents []byte) []byte {
	hash := sha256.Sum256(contents)
	return []byte(fmt.Sprintf(`version: 1
detector_id: %s
provider: github
repository: owner/repo
repository_url: https://github.com/owner/repo
commit: abc123def456
original_path: %s
permalink: https://github.com/owner/repo/blob/abc123def456/%s
retrieved_at: 2026-08-08T00:00:00Z
sha256: %x
license: MIT
license_url: https://opensource.org/license/mit
project_kind: library
variation_tags: [typical]
rationale: typical dependency source
`, detectorID, sourcePath, sourcePath, hash))
}

func TestRunCreatesAResumableCheckpoint(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, &stdout, &stderr, unavailableAgent{}); got != 0 {
		t.Fatalf("initialize exit status = %d, stderr = %s", got, stderr.String())
	}

	agent := fakeAgent{outcome: acceptedOutcome, write: func(iteration Iteration) error {
		path := filepath.Join(iteration.CorpusDir, "owner-repo-abc123", "project", "dependencies.txt")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte("example dependency\n"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(iteration.CorpusDir, "owner-repo-abc123", "provenance.yaml"), provenance(iteration.DetectorID, "project/dependencies.txt", []byte("example dependency\n")), 0o644)
	}}
	stdout.Reset()
	stderr.Reset()
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, agent); got != 0 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}

	progressDocument, err := readProgress(progress)
	if err != nil {
		t.Fatal(err)
	}
	if got := progressDocument.Detectors[0]; got.State != stateInProgress || got.Iterations != 1 || len(got.Examples) != 1 {
		t.Fatalf("unexpected checkpoint: %+v", got)
	}
	if !strings.Contains(stdout.String(), "checkpoint: example-detector iteration 1") {
		t.Fatalf("summary = %q", stdout.String())
	}
}

func TestRunRejectsUnsafeOrUnverifiedCorpus(t *testing.T) {
	for name, contents := range map[string][]byte{
		"unsafe credential": []byte("token = ghp_abcdefghijklmnopqrstuvwxyz1234567890\n"),
		"git lfs pointer":   []byte("version https://git-lfs.github.com/spec/v1\noid sha256:abc\nsize 1\n"),
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			progress := filepath.Join(root, "collection.yaml")
			if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableAgent{}); got != 0 {
				t.Fatalf("initialize exit status = %d", got)
			}
			agent := fakeAgent{outcome: acceptedOutcome, write: func(iteration Iteration) error {
				path := filepath.Join(iteration.CorpusDir, "owner-repo-abc123", "project", "dependencies.txt")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(path, contents, 0o644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(iteration.CorpusDir, "owner-repo-abc123", "provenance.yaml"), provenance(iteration.DetectorID, "project/dependencies.txt", contents), 0o644)
			}}
			var stdout, stderr bytes.Buffer
			if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, agent); got != 1 {
				t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
			}
			if !strings.Contains(stderr.String(), "unvalidated collection changes") {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestRunRejectsHashMismatch(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableAgent{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	contents := []byte("example dependency\n")
	agent := fakeAgent{outcome: acceptedOutcome, write: func(iteration Iteration) error {
		path := filepath.Join(iteration.CorpusDir, "owner-repo-abc123", "project", "dependencies.txt")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, contents, 0o644); err != nil {
			return err
		}
		bad := provenance(iteration.DetectorID, "project/dependencies.txt", contents)
		bad[bytes.Index(bad, []byte("sha256: "))+len("sha256: ")] = 'z'
		return os.WriteFile(filepath.Join(iteration.CorpusDir, "owner-repo-abc123", "provenance.yaml"), bad, 0o644)
	}}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, agent); got != 1 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "SHA-256") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsChangesOutsideSelectedCorpus(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableAgent{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}

	badAgent := fakeAgent{outcome: acceptedOutcome, write: func(Iteration) error {
		return os.WriteFile(filepath.Join(root, "unrelated.txt"), []byte("no"), 0o644)
	}}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, badAgent); got != 1 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unvalidated collection changes") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	progressDocument, err := readProgress(progress)
	if err != nil {
		t.Fatal(err)
	}
	if got := progressDocument.Detectors[0]; got.Iterations != 0 || got.State != statePending {
		t.Fatalf("invalid attempt advanced progress: %+v", got)
	}
}

func TestRunRejectsProtocolThatDisagreesWithCorpus(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableAgent{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	agent := fakeAgent{outcome: Outcome{Result: "accepted"}, write: func(iteration Iteration) error {
		path := filepath.Join(iteration.CorpusDir, "owner-repo-abc123", "provenance.yaml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("version: 1\n"), 0o644)
	}}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, agent); got != 1 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "protocol does not match") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPrefersInProgressAndCanTargetOneDetector(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	p := Progress{Version: 1, Detectors: []DetectorProgress{
		{ID: "pending-first", State: statePending, Examples: []string{}},
		{ID: "in-progress-second", State: stateInProgress, Examples: []string{}},
	}}
	if err := writeProgress(progress, p); err != nil {
		t.Fatal(err)
	}
	var ran []string
	agent := fakeAgent{outcome: Outcome{Result: "unsuccessful"}, write: func(i Iteration) error { ran = append(ran, i.DetectorID); return nil }}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, agent); got != 0 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	if len(ran) != 1 || ran[0] != "in-progress-second" {
		t.Fatalf("selected detectors = %v", ran)
	}
	if got := run([]string{"run", "--single", "--progress", progress, "--detector", "pending-first"}, root, &stdout, &stderr, agent); got != 0 {
		t.Fatalf("targeted run exit status = %d, stderr = %s", got, stderr.String())
	}
	if len(ran) != 2 || ran[1] != "pending-first" {
		t.Fatalf("targeted detectors = %v", ran)
	}
}

func TestRunFullStopsAtIterationBudgetAndDoesNotAdvanceInfrastructureFailures(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	p := Progress{Version: 1, Detectors: []DetectorProgress{{ID: "example", State: stateInProgress, Iterations: 6, Examples: []string{}}}}
	if err := writeProgress(progress, p); err != nil {
		t.Fatal(err)
	}
	var calls int
	agent := fakeAgent{outcome: Outcome{Result: "unsuccessful"}, write: func(Iteration) error { calls++; return nil }}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--progress", progress}, root, &stdout, &stderr, agent); got != 2 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("agent calls = %d, want 1", calls)
	}
	p, err := readProgress(progress)
	if err != nil {
		t.Fatal(err)
	}
	if p.Detectors[0].Iterations != 7 || p.Detectors[0].State != stateBlocked {
		t.Fatalf("progress = %+v", p.Detectors[0])
	}
}

func TestRunRefusesExistingProgressLockWithoutMutation(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	p := Progress{Version: 1, Detectors: []DetectorProgress{{ID: "example", State: statePending, Examples: []string{}}}}
	if err := writeProgress(progress, p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(progress+".lock", []byte("active"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, unavailableAgent{}); got != 1 {
		t.Fatalf("run exit status = %d", got)
	}
	if !strings.Contains(stderr.String(), "another collection run holds") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	after, err := os.ReadFile(progress)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "iterations: 0") {
		t.Fatalf("progress changed: %s", after)
	}
}

func TestRunStopsBeforeSchedulingAtSoftDuration(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	p := Progress{Version: 1, Detectors: []DetectorProgress{{ID: "example", State: statePending, Examples: []string{}}}}
	if err := writeProgress(progress, p); err != nil {
		t.Fatal(err)
	}
	originalNow := now
	now = func() time.Time { return time.Unix(100, 0) }
	t.Cleanup(func() { now = originalNow })
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--progress", progress, "--duration", "0s"}, root, &stdout, &stderr, unavailableAgent{}); got != 0 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "soft duration reached") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
