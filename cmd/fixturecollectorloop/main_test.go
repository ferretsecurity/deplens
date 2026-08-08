package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunSecondInterruptRecordsRecoveryAndRefusesResume(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example"}, root, os.Stdout, os.Stderr, unavailableAgent{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)

	signals := make(chan os.Signal, 2)
	originalSignals := collectionSignals
	collectionSignals = func() (<-chan os.Signal, func()) { return signals, func() {} }
	t.Cleanup(func() { collectionSignals = originalSignals })
	agent := blockingAgent{started: make(chan struct{})}
	result := make(chan int, 1)
	var stdout, stderr bytes.Buffer
	go func() {
		result <- run([]string{"run", "--progress", progress}, root, &stdout, &stderr, agent)
	}()
	<-agent.started
	signals <- os.Interrupt
	signals <- os.Interrupt
	if got := <-result; got != 1 {
		t.Fatalf("forced interruption exit status = %d, stderr = %s", got, stderr.String())
	}
	stored, err := readProgress(progress)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Recovery == nil || stored.Recovery.DetectorID != "example" {
		t.Fatalf("recovery = %+v", stored.Recovery)
	}
	stdout.Reset()
	stderr.Reset()
	if got := run([]string{"run", "--single", "--progress", progress, "--allow-dirty"}, root, &stdout, &stderr, unavailableAgent{}); got != 1 {
		t.Fatalf("recovery-required exit status = %d", got)
	}
	for _, want := range []string{"recovery is required", "example", "last checkpoint", "progress:"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("recovery guidance missing %q: %s", want, stderr.String())
		}
	}
}

func TestRunFirstInterruptAllowsActiveIterationToCheckpoint(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example"}, root, os.Stdout, os.Stderr, unavailableAgent{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	signals := make(chan os.Signal, 2)
	originalSignals := collectionSignals
	collectionSignals = func() (<-chan os.Signal, func()) { return signals, func() {} }
	t.Cleanup(func() { collectionSignals = originalSignals })
	started, finish := make(chan struct{}), make(chan struct{})
	agent := fakeAgent{outcome: Outcome{Result: "unsuccessful"}, write: func(Iteration) error {
		close(started)
		<-finish
		return nil
	}}
	var stdout, stderr bytes.Buffer
	result := make(chan int, 1)
	go func() {
		result <- run([]string{"run", "--progress", progress}, root, &stdout, &stderr, agent)
	}()
	<-started
	signals <- os.Interrupt
	close(finish)
	if got := <-result; got != 0 {
		t.Fatalf("graceful interruption exit status = %d, stderr = %s", got, stderr.String())
	}
	stored, err := readProgress(progress)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Recovery != nil || stored.Detectors[0].Iterations != 1 {
		t.Fatalf("graceful interrupt did not checkpoint: %+v", stored)
	}
	if !strings.Contains(stdout.String(), "active iteration may validate and checkpoint") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

type blockingAgent struct{ started chan struct{} }

func (a blockingAgent) Run(ctx context.Context, _ Iteration) (Outcome, error) {
	close(a.started)
	<-ctx.Done()
	return Outcome{}, ctx.Err()
}

func initializeGitRepository(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "Fixture Collector Test"}, {"config", "user.email", "fixture-collector@example.test"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("test repository\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitGitChanges(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"-c", "commit.gpgSign=false", "commit", "--no-gpg-sign", "--no-verify", "-qm", "test checkpoint"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
}

func TestCommandWorkflow(t *testing.T) {
	newRepository := func(t *testing.T) (string, string) {
		t.Helper()
		root := t.TempDir()
		progress := filepath.Join(root, "collection.yaml")
		var stdout, stderr bytes.Buffer
		if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, &stdout, &stderr, unavailableAgent{}); got != 0 {
			t.Fatalf("initialize exit status = %d, stderr = %s", got, stderr.String())
		}
		initializeGitRepository(t, root)
		commitGitChanges(t, root)
		return root, progress
	}
	accepted := func() fakeAgent {
		contents := []byte("example dependency\n")
		return fakeAgent{outcome: acceptedOutcome, write: func(iteration Iteration) error {
			path := filepath.Join(iteration.CorpusDir, "owner-repo-abc123", "project", "dependencies.txt")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, contents, 0o644); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(iteration.CorpusDir, "owner-repo-abc123", "provenance.yaml"), provenance(iteration.DetectorID, "project/dependencies.txt", contents), 0o644)
		}}
	}

	t.Run("initializes, refuses dirt, warns on override, checkpoints, and stops at deadline", func(t *testing.T) {
		root, progress := newRepository(t)
		if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, unavailableAgent{}); got != 1 {
			t.Fatalf("dirty exit status = %d", got)
		}
		if !strings.Contains(stderr.String(), "git reset --hard HEAD") || !strings.Contains(stderr.String(), "--allow-dirty") {
			t.Fatalf("dirty guidance = %s", stderr.String())
		}
		if got := run([]string{"run", "--single", "--progress", progress, "--allow-dirty"}, root, &stdout, &stderr, fakeAgent{outcome: Outcome{Result: "unsuccessful"}, write: func(Iteration) error { return nil }}); got != 0 {
			t.Fatalf("override exit status = %d", got)
		}
		if !strings.Contains(stderr.String(), "WARNING: --allow-dirty") {
			t.Fatalf("override warning = %s", stderr.String())
		}
		if err := os.Remove(filepath.Join(root, "dirty.txt")); err != nil {
			t.Fatal(err)
		}
		stdout.Reset()
		stderr.Reset()
		if got := run([]string{"run", "--single", "--progress", progress, "--allow-dirty"}, root, &stdout, &stderr, accepted()); got != 0 {
			t.Fatalf("checkpoint exit status = %d, stderr = %s", got, stderr.String())
		}
		if !strings.Contains(stdout.String(), "checkpoint: example-detector iteration 2") {
			t.Fatalf("checkpoint output = %s", stdout.String())
		}
		originalNow := now
		now = func() time.Time { return time.Unix(100, 0) }
		t.Cleanup(func() { now = originalNow })
		stdout.Reset()
		if got := run([]string{"run", "--progress", progress, "--duration", "0s", "--allow-dirty"}, root, &stdout, &stderr, unavailableAgent{}); got != 0 {
			t.Fatalf("deadline exit status = %d", got)
		}
		if !strings.Contains(stdout.String(), "soft duration reached") {
			t.Fatalf("deadline output = %s", stdout.String())
		}
	})

	t.Run("stops gracefully and records forced recovery", func(t *testing.T) {
		root, progress := newRepository(t)
		signals := make(chan os.Signal, 2)
		originalSignals := collectionSignals
		collectionSignals = func() (<-chan os.Signal, func()) { return signals, func() {} }
		t.Cleanup(func() { collectionSignals = originalSignals })
		started := make(chan struct{})
		release := make(chan struct{})
		agent := fakeAgent{outcome: Outcome{Result: "unsuccessful"}, write: func(Iteration) error {
			close(started)
			<-release
			return nil
		}}
		result := make(chan int, 1)
		go func() { result <- run([]string{"run", "--progress", progress}, root, io.Discard, io.Discard, agent) }()
		<-started
		signals <- os.Interrupt
		close(release)
		if got := <-result; got != 0 {
			t.Fatalf("graceful stop exit status = %d", got)
		}
		stored, err := readProgress(progress)
		if err != nil || stored.Recovery != nil || stored.Detectors[0].Iterations != 1 {
			t.Fatalf("graceful checkpoint = %+v, err = %v", stored, err)
		}

		root, progress = newRepository(t)
		signals = make(chan os.Signal, 2)
		started = make(chan struct{})
		forcedAgent := blockingAgent{started: started}
		result = make(chan int, 1)
		go func() {
			result <- run([]string{"run", "--progress", progress}, root, io.Discard, io.Discard, forcedAgent)
		}()
		<-started
		signals <- os.Interrupt
		signals <- os.Interrupt
		if got := <-result; got != 1 {
			t.Fatalf("forced stop exit status = %d", got)
		}
		var stderr bytes.Buffer
		if got := run([]string{"run", "--single", "--progress", progress, "--allow-dirty"}, root, io.Discard, &stderr, unavailableAgent{}); got != 1 {
			t.Fatalf("recovery-required exit status = %d", got)
		}
		if !strings.Contains(stderr.String(), "recovery is required") {
			t.Fatalf("recovery guidance = %s", stderr.String())
		}
	})

	t.Run("creates an optional local commit", func(t *testing.T) {
		root, progress := newRepository(t)
		if got := run([]string{"run", "--single", "--commit", "--progress", progress}, root, io.Discard, io.Discard, accepted()); got != 0 {
			t.Fatalf("commit exit status = %d", got)
		}
		message, err := gitOutput(root, "log", "-1", "--format=%s")
		if err != nil || !strings.Contains(message, "collect example-detector corpus examples") {
			t.Fatalf("commit = %q, err = %v", message, err)
		}
	})
}

func TestRunRefusesDirtyCheckoutAndAllowsExplicitOverride(t *testing.T) {
	root := t.TempDir()
	initializeGitRepository(t, root)
	progress := filepath.Join(root, "collection.yaml")
	p := Progress{Version: 1, Detectors: []DetectorProgress{{ID: "example", State: statePending, Examples: []string{}}}}
	if err := writeProgress(progress, p); err != nil {
		t.Fatal(err)
	}
	commitGitChanges(t, root)
	if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, unavailableAgent{}); got != 1 {
		t.Fatalf("dirty run exit status = %d", got)
	}
	for _, want := range []string{"dirty.txt", "git reset --hard HEAD", "git clean -fd", "--allow-dirty"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("dirty refusal missing %q: %s", want, stderr.String())
		}
	}
	if got := run([]string{"run", "--single", "--progress", progress, "--allow-dirty"}, root, &stdout, &stderr, unavailableAgent{}); got != 1 {
		t.Fatalf("override run exit status = %d", got)
	}
	if !strings.Contains(stderr.String(), "WARNING: --allow-dirty") {
		t.Fatalf("override warning missing: %s", stderr.String())
	}
}

func TestRunRequiresGitCheckout(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if err := writeProgress(progress, Progress{Version: 1, Detectors: []DetectorProgress{{ID: "example", State: statePending}}}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, unavailableAgent{}); got != 1 {
		t.Fatalf("run exit status = %d", got)
	}
	if !strings.Contains(stderr.String(), "requires a Git checkout") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

type fakeAgent struct {
	write   func(Iteration) error
	outcome Outcome
}

func (f fakeAgent) Run(_ context.Context, iteration Iteration) (Outcome, error) {
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
	initializeGitRepository(t, root)
	commitGitChanges(t, root)

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

func TestRunCommitCreatesAtomicCollectionCheckpoint(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableAgent{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	contents := []byte("example dependency\n")
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
	if got := run([]string{"run", "--single", "--commit", "--progress", progress}, root, &stdout, &stderr, agent); got != 0 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	message, err := gitOutput(root, "log", "-1", "--format=%s")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(message), "collect example-detector corpus examples (iteration 1)"; got != want {
		t.Fatalf("commit message = %q, want %q", got, want)
	}
	changed, err := gitOutput(root, "show", "--format=", "--name-only", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"collection.yaml", "testdata/corpus/example-detector/owner-repo-abc123/project/dependencies.txt", "testdata/corpus/example-detector/owner-repo-abc123/provenance.yaml"} {
		if !strings.Contains(changed, want) {
			t.Fatalf("commit paths missing %q: %s", want, changed)
		}
	}
	status, err := gitOutput(root, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if status != "" {
		t.Fatalf("collection checkpoint left changes: %q", status)
	}
}

func TestRunCommitRecordsUnsuccessfulCollectionAttempt(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableAgent{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	var stdout, stderr bytes.Buffer
	agent := fakeAgent{outcome: Outcome{Result: "unsuccessful"}, write: func(Iteration) error { return nil }}
	if got := run([]string{"run", "--single", "--commit", "--progress", progress}, root, &stdout, &stderr, agent); got != 0 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	message, err := gitOutput(root, "log", "-1", "--format=%s")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(message), "record example-detector collection attempt (iteration 1)"; got != want {
		t.Fatalf("commit message = %q, want %q", got, want)
	}
	changed, err := gitOutput(root, "show", "--format=", "--name-only", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(changed) != "collection.yaml" {
		t.Fatalf("progress-only commit changed %q", changed)
	}
}

func TestRunCommitFailurePreservesValidatedUncommittedCheckpoint(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableAgent{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	originalGitCommit := gitCommit
	gitCommit = func(_ string, args ...string) (string, error) {
		return "", fmt.Errorf("git %s: simulated commit failure", strings.Join(args, " "))
	}
	t.Cleanup(func() { gitCommit = originalGitCommit })
	contents := []byte("example dependency\n")
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
	if got := run([]string{"run", "--single", "--commit", "--progress", progress}, root, &stdout, &stderr, agent); got != 1 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	updated, err := readProgress(progress)
	if err != nil {
		t.Fatal(err)
	}
	if got := updated.Detectors[0]; got.Iterations != 1 || len(got.Examples) != 1 {
		t.Fatalf("valid checkpoint was not preserved: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(root, "testdata", "corpus", "example-detector", "owner-repo-abc123", "project", "dependencies.txt")); err != nil {
		t.Fatalf("validated corpus was not preserved: %v", err)
	}
	for _, want := range []string{"collection commit failed after a valid checkpoint", "remain as uncommitted collection state"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("commit failure guidance missing %q: %s", want, stderr.String())
		}
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
			initializeGitRepository(t, root)
			commitGitChanges(t, root)
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
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
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
	initializeGitRepository(t, root)
	commitGitChanges(t, root)

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
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
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
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	var ran []string
	agent := fakeAgent{outcome: Outcome{Result: "unsuccessful"}, write: func(i Iteration) error { ran = append(ran, i.DetectorID); return nil }}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, agent); got != 0 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	if len(ran) != 1 || ran[0] != "in-progress-second" {
		t.Fatalf("selected detectors = %v", ran)
	}
	if got := run([]string{"run", "--single", "--progress", progress, "--detector", "pending-first", "--allow-dirty"}, root, &stdout, &stderr, agent); got != 0 {
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
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
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
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
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
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
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

type fakeCommandRunner struct {
	calls [][]string
	final Outcome
	err   error
}

func (f *fakeCommandRunner) Run(_ context.Context, _ string, name string, args []string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if name != "codex" || len(args) == 0 || args[0] != "exec" {
		return []byte("authenticated"), nil
	}
	for i := range args {
		if args[i] == "--output-last-message" && i+1 < len(args) {
			contents, _ := json.Marshal(f.final)
			if err := os.WriteFile(args[i+1], contents, 0o600); err != nil {
				return nil, err
			}
		}
	}
	return []byte(`{"type":"item.completed"}` + "\n"), f.err
}

func TestCodexAgentPreflightsAndUsesFreshStructuredSession(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	runner := &fakeCommandRunner{final: Outcome{Result: "unsuccessful", Added: []string{}, Queries: []string{}, Candidates: []string{}, Rejections: []string{}}}
	agent := &codexAgent{root: root, stdout: &stdout, runner: runner}
	if err := agent.Preflight(); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.Run(context.Background(), Iteration{DetectorID: "npm", CorpusDir: filepath.Join(root, "testdata", "corpus", "npm"), Iteration: 2, QueryLimit: 5, CandidateLimit: 20}); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 3 || strings.Join(runner.calls[0], " ") != "codex login status" || strings.Join(runner.calls[1], " ") != "gh auth status --hostname github.com" {
		t.Fatalf("preflight calls = %#v", runner.calls)
	}
	call := strings.Join(runner.calls[2], " ")
	for _, want := range []string{"codex exec", "--json", "--sandbox workspace-write", "--output-schema", "--output-last-message", "default branch to a commit SHA", "at most 5 search queries"} {
		if !strings.Contains(call, want) {
			t.Fatalf("session command missing %q: %s", want, call)
		}
	}
	logs, err := filepath.Glob(filepath.Join(root, ".deplens", "fixture-collection-logs", "*.jsonl"))
	if err != nil || len(logs) != 0 {
		t.Fatalf("successful non-retained logs = %v, err = %v", logs, err)
	}
}

func TestCodexAgentRetainsFailureLogsOwnerOnly(t *testing.T) {
	root := t.TempDir()
	runner := &fakeCommandRunner{final: Outcome{}, err: fmt.Errorf("simulated failure")}
	agent := &codexAgent{root: root, stdout: io.Discard, runner: runner}
	_, err := agent.Run(context.Background(), Iteration{DetectorID: "npm", CorpusDir: filepath.Join(root, "testdata", "corpus", "npm"), Iteration: 1})
	if err == nil || !strings.Contains(err.Error(), "log retained") || !strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("error = %v", err)
	}
	path := filepath.Join(root, ".deplens", "fixture-collection-logs", "npm-1.jsonl")
	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("stat failure log: %v", statErr)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("failure log mode = %v", info.Mode())
	}
}

func TestCodexAgentRetainsSuccessfulLogsOnlyWhenRequested(t *testing.T) {
	root := t.TempDir()
	runner := &fakeCommandRunner{final: Outcome{Result: "unsuccessful", Added: []string{}, Queries: []string{}, Candidates: []string{}, Rejections: []string{}}}
	agent := &codexAgent{root: root, stdout: io.Discard, runner: runner, retainLogs: true}
	if _, err := agent.Run(context.Background(), Iteration{DetectorID: "npm", CorpusDir: filepath.Join(root, "testdata", "corpus", "npm"), Iteration: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".deplens", "fixture-collection-logs", "npm-1.jsonl")); err != nil {
		t.Fatalf("retained success log: %v", err)
	}
}
