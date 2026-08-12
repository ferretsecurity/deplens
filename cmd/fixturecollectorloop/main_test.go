package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validTestProgress(detectors ...DetectorProgress) Progress {
	for i := range detectors {
		if detectors[i].Target == 0 {
			detectors[i].Target = defaultTarget
		}
		if detectors[i].Examples == nil {
			detectors[i].Examples = []string{}
		}
		if detectors[i].State != stateNeedsQueryReview && len(detectors[i].QueryPlan) == 0 {
			detectors[i].QueryPlan = []string{"filename:" + detectors[i].ID}
		}
	}
	return Progress{Version: progressVersion, Limits: defaultCollectionLimits, Detectors: detectors}
}

func TestRunSecondInterruptRecordsRecoveryAndAllowsExplicitDirtyResume(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example"}, root, os.Stdout, os.Stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)

	signals := make(chan os.Signal, 2)
	originalSignals := collectionSignals
	collectionSignals = func() (<-chan os.Signal, func()) { return signals, func() {} }
	t.Cleanup(func() { collectionSignals = originalSignals })
	agent := blockingResearcher{started: make(chan struct{})}
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
	if got := run([]string{"run", "--single", "--progress", progress, "--allow-dirty"}, root, &stdout, &stderr, fakeResearcher{outcome: Outcome{Result: "unsuccessful"}, write: func(Iteration) error { return nil }}); got != 0 {
		t.Fatalf("explicit dirty resume exit status = %d, stderr = %s", got, stderr.String())
	}
	for _, want := range []string{"WARNING: --allow-dirty", "resuming after recovery", "example"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("explicit dirty resume guidance missing %q: %s", want, stderr.String())
		}
	}
}

func TestPrintRecoveryRequiredShowsTargetedCleanupAndDirtyResume(t *testing.T) {
	var stderr bytes.Buffer
	printRecoveryRequired(Recovery{
		DetectorID:   "example",
		Iteration:    2,
		RunID:        "run-123",
		ProgressPath: ".deplens/fixture-collection.yaml",
		ChangedPaths: []string{
			"testdata/corpus/example/owner-repo/source.lock",
		},
	}, &stderr)
	for _, want := range []string{
		"git restore --worktree -- .deplens/fixture-collection.yaml",
		"git clean -fdn -- testdata/corpus/example/owner-repo/source.lock",
		"git clean -fd -- testdata/corpus/example/owner-repo/source.lock",
		"run --single --progress .deplens/fixture-collection.yaml --allow-dirty",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("recovery guidance missing %q: %s", want, stderr.String())
		}
	}
}

func TestRunFirstInterruptAllowsActiveIterationToCheckpoint(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example"}, root, os.Stdout, os.Stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	signals := make(chan os.Signal, 2)
	originalSignals := collectionSignals
	collectionSignals = func() (<-chan os.Signal, func()) { return signals, func() {} }
	t.Cleanup(func() { collectionSignals = originalSignals })
	started, finish := make(chan struct{}), make(chan struct{})
	agent := fakeResearcher{outcome: Outcome{Result: "unsuccessful"}, write: func(Iteration) error {
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

type blockingResearcher struct{ started chan struct{} }

func (a blockingResearcher) Research(ctx context.Context, _ Iteration) (ResearchResult, error) {
	close(a.started)
	<-ctx.Done()
	return ResearchResult{}, ctx.Err()
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

func TestInitializeProgressBuildsReviewedDetectorInventory(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	progress := filepath.Join(t.TempDir(), "collection.yaml")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"initialize-progress", "--progress", progress}, projectRoot, &stdout, &stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d, stderr = %s", got, stderr.String())
	}
	p, err := readProgress(progress)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Detectors) != 144 || p.InventoryFingerprint == "" {
		t.Fatalf("reviewed inventory = %d detectors, fingerprint %q", len(p.Detectors), p.InventoryFingerprint)
	}
	if p.Detectors[0].Form == "" || len(p.Detectors[0].Roles) == 0 {
		t.Fatalf("first detector lacks reviewed semantics: %+v", p.Detectors[0])
	}
}

func TestRunRefusesAnInventoryFingerprintMismatch(t *testing.T) {
	progress := filepath.Join(t.TempDir(), "collection.yaml")
	p := validTestProgress(DetectorProgress{ID: "example", State: statePending})
	p.InventoryFingerprint = "changed-detector-inventory"
	if err := writeProgress(progress, p); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--progress", progress}, t.TempDir(), &stdout, &stderr, unavailableResearcher{}); got != 1 {
		t.Fatalf("run exit status = %d", got)
	}
	if !strings.Contains(stderr.String(), "no longer matches the reviewed plan") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestCommandWorkflow(t *testing.T) {
	newRepository := func(t *testing.T) (string, string) {
		t.Helper()
		root := t.TempDir()
		progress := filepath.Join(root, "collection.yaml")
		var stdout, stderr bytes.Buffer
		if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, &stdout, &stderr, unavailableResearcher{}); got != 0 {
			t.Fatalf("initialize exit status = %d, stderr = %s", got, stderr.String())
		}
		initializeGitRepository(t, root)
		commitGitChanges(t, root)
		return root, progress
	}
	accepted := func() fakeResearcher {
		contents := []byte("example dependency\n")
		return fakeResearcher{outcome: acceptedOutcome, write: func(iteration Iteration) error {
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
		if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, unavailableResearcher{}); got != 1 {
			t.Fatalf("dirty exit status = %d", got)
		}
		if !strings.Contains(stderr.String(), "git reset --hard HEAD") || !strings.Contains(stderr.String(), "--allow-dirty") {
			t.Fatalf("dirty guidance = %s", stderr.String())
		}
		if got := run([]string{"run", "--single", "--progress", progress, "--allow-dirty"}, root, &stdout, &stderr, fakeResearcher{outcome: Outcome{Result: "unsuccessful"}, write: func(Iteration) error { return nil }}); got != 0 {
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
		if !strings.Contains(stdout.String(), "│    Iteration 2 saved") {
			t.Fatalf("checkpoint output = %s", stdout.String())
		}
		originalNow := now
		now = func() time.Time { return time.Unix(100, 0) }
		t.Cleanup(func() { now = originalNow })
		stdout.Reset()
		if got := run([]string{"run", "--progress", progress, "--duration", "0s", "--allow-dirty"}, root, &stdout, &stderr, unavailableResearcher{}); got != 0 {
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
		agent := fakeResearcher{outcome: Outcome{Result: "unsuccessful"}, write: func(Iteration) error {
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
		forcedAgent := blockingResearcher{started: started}
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
		if got := run([]string{"run", "--single", "--progress", progress}, root, io.Discard, &stderr, unavailableResearcher{}); got != 1 {
			t.Fatalf("recovery-required exit status = %d", got)
		}
		if !strings.Contains(stderr.String(), "recovery is required") || strings.Contains(stderr.String(), "log:") {
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
	p := validTestProgress(DetectorProgress{ID: "example", State: statePending})
	if err := writeProgress(progress, p); err != nil {
		t.Fatal(err)
	}
	commitGitChanges(t, root)
	if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, unavailableResearcher{}); got != 1 {
		t.Fatalf("dirty run exit status = %d", got)
	}
	for _, want := range []string{"dirty.txt", "git reset --hard HEAD", "git clean -fd", "--allow-dirty"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("dirty refusal missing %q: %s", want, stderr.String())
		}
	}
	if got := run([]string{"run", "--single", "--progress", progress, "--allow-dirty"}, root, &stdout, &stderr, unavailableResearcher{}); got != 1 {
		t.Fatalf("override run exit status = %d", got)
	}
	if !strings.Contains(stderr.String(), "WARNING: --allow-dirty") {
		t.Fatalf("override warning missing: %s", stderr.String())
	}
}

func TestRunRequiresGitCheckout(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if err := writeProgress(progress, validTestProgress(DetectorProgress{ID: "example", State: statePending})); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, unavailableResearcher{}); got != 1 {
		t.Fatalf("run exit status = %d", got)
	}
	if !strings.Contains(stderr.String(), "requires a Git checkout") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunConsumesAnInMemoryResearchResult(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, &stdout, &stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d, stderr = %s", got, stderr.String())
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)

	researcher := &trackingResearcher{result: ResearchResult{Outcome: Outcome{Result: "unsuccessful"}}}
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, researcher); got != 0 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	if !researcher.called {
		t.Fatal("researcher was not called")
	}
	stored, err := readProgress(progress)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Detectors[0].Iterations != 1 {
		t.Fatalf("iterations = %d, want 1", stored.Detectors[0].Iterations)
	}
}

func TestRunTargetedModeNeverFallsBackToAnotherDetector(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	p := validTestProgress(
		DetectorProgress{ID: "review", State: stateNeedsQueryReview, QueryReviewReason: "manual query required"},
		DetectorProgress{ID: "complete", State: stateComplete, Examples: []string{"one", "two", "three"}},
		DetectorProgress{ID: "ready", State: statePending},
	)
	if err := writeProgress(progress, p); err != nil {
		t.Fatal(err)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)

	researcher := &trackingResearcher{result: ResearchResult{Outcome: Outcome{Result: "unsuccessful"}}}
	var stdout, stderr bytes.Buffer
	for _, target := range []string{"review", "complete"} {
		if got := run([]string{"run", "--detector", target, "--progress", progress}, root, &stdout, &stderr, researcher); got != 0 {
			t.Fatalf("%s-targeted run = %d, stderr = %s", target, got, stderr.String())
		}
		if researcher.called {
			t.Fatalf("targeted %s detector fell back to a different detector", target)
		}
	}
	stored, err := readProgress(progress)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Detectors[2].Iterations != 0 {
		t.Fatalf("unrelated detector was run: %+v", stored.Detectors[2])
	}

	stderr.Reset()
	if got := run([]string{"run", "--detector", "missing", "--progress", progress}, root, io.Discard, &stderr, researcher); got != 1 {
		t.Fatalf("unknown-targeted run = %d", got)
	}
	if !strings.Contains(stderr.String(), `unknown detector "missing"`) {
		t.Fatalf("unknown target guidance = %s", stderr.String())
	}
}

type fakeResearcher struct {
	write   func(Iteration) error
	outcome Outcome
}

func (f fakeResearcher) Research(_ context.Context, iteration Iteration) (ResearchResult, error) {
	return ResearchResult{Outcome: f.outcome}, f.write(iteration)
}

type unavailableResearcher struct{}

func (unavailableResearcher) Research(context.Context, Iteration) (ResearchResult, error) {
	return ResearchResult{}, errors.New("no Researcher is configured; inject one through the command seam")
}

type trackingResearcher struct {
	called bool
	result ResearchResult
}

func (f *trackingResearcher) Research(context.Context, Iteration) (ResearchResult, error) {
	f.called = true
	return f.result, nil
}

var acceptedOutcome = Outcome{Result: "accepted", Added: []string{"owner-repo-abc123/project/dependencies.txt", "owner-repo-abc123/provenance.yaml"}}

func provenance(detectorID, sourcePath string, contents []byte) []byte {
	hash := sha256.Sum256(contents)
	licenseHash := sha256.Sum256([]byte("MIT License\n"))
	commit := "abc123def456"
	return []byte(fmt.Sprintf(`version: 2
detector_id: %s
candidate_id: %s
provider: github
repository: owner/repo
repository_url: https://github.com/owner/repo
default_branch: main
commit: %s
original_path: %s
permalink: https://github.com/owner/repo/blob/%s/%s
retrieved_at: 2026-08-08T00:00:00Z
sha256: %x
governing_license:
  spdx: MIT
  path: LICENSE
  permalink: https://github.com/owner/repo/blob/%s/LICENSE
  sha256: %x
rationale: typical dependency source
`, detectorID, stableCandidateID("github", "owner/repo", commit, sourcePath), commit, sourcePath, commit, sourcePath, hash, commit, licenseHash))
}

func TestRunCreatesAResumableCheckpoint(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, &stdout, &stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d, stderr = %s", got, stderr.String())
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)

	agent := fakeResearcher{outcome: acceptedOutcome, write: func(iteration Iteration) error {
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
	for _, want := range []string{
		"┌─ Detector: example-detector",
		"│  Run position: 1/1 · remaining: 1 · run elapsed: 0s",
		"│  Iteration: 1/7",
		"│  Examples: 0/3",
		"│    Iteration 1 saved",
		"│    1. Source: " + filepath.Join(root, "testdata", "corpus", "example-detector", "owner-repo-abc123", "project", "dependencies.txt"),
		"│       Provenance: " + filepath.Join(root, "testdata", "corpus", "example-detector", "owner-repo-abc123", "provenance.yaml"),
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("run output missing %q: %s", want, stdout.String())
		}
	}
}

func TestRunPrintsEffectiveFlagValues(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, io.Discard, io.Discard, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--progress", progress, "--duration", "0s"}, root, &stdout, &stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	for _, want := range []string{
		"run configuration:",
		"--allow-dirty=false",
		"--candidate-limit=100",
		"--commit=false",
		"--detector=",
		"--duration=0s",
		"--progress=" + progress,
		"--query-limit=5",
		"--single=false",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("configuration missing %q: %s", want, stdout.String())
		}
	}
}

func TestRunPrintsResearchProgress(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, io.Discard, io.Discard, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	var stdout, stderr bytes.Buffer
	researcher := fakeResearcher{outcome: Outcome{Result: "unsuccessful"}, write: func(iteration Iteration) error {
		iteration.ReportProgress(ResearchProgress{Stage: progressSearch, Provider: "github", Query: "filename:go.work", QueryIndex: 1, QueryTotal: 1, Page: 1, Hits: 100, Budget: "search", DownloadedBytes: 512 << 10, ByteLimit: 4 << 20, RemainingBytes: 3584 << 10})
		iteration.ReportProgress(ResearchProgress{Stage: progressSearch, ProviderEvent: &ProviderProgress{Action: "wait", Reason: "primary-rate-limit", Resource: "code_search", Status: 403, Attempt: 1, MaxAttempts: 4, Delay: 42 * time.Second, Reset: time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC), Remaining: 0, Limit: 10, CountersKnown: true, RequestID: "ABCD:1234"}})
		iteration.ReportProgress(ResearchProgress{Stage: progressQualification, Inspected: 4, InspectionLimit: 40, Qualified: 1, Rejected: 3, Filtered: 12, Budget: "acquisition", DownloadedBytes: 2 << 20, ByteLimit: 12 << 20, RemainingBytes: 10 << 20})
		iteration.ReportProgress(ResearchProgress{Stage: progressQualification, Final: true, Inspected: 40, InspectionLimit: 40, Qualified: 9, Rejected: 31, Filtered: 105, Budget: "acquisition", DownloadedBytes: 8 << 20, ByteLimit: 12 << 20, RemainingBytes: 4 << 20})
		iteration.ReportProgress(ResearchProgress{Stage: progressSelection, Candidates: 9})
		iteration.ReportProgress(ResearchProgress{Stage: progressSelection, Final: true, Selected: 3})
		return nil
	}}
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, researcher); got != 0 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	for _, want := range []string{
		"│  Candidate search",
		`│    github query 1/1 · page 1 · 100 results · expression: "filename:go.work" · search: 512.0KiB/4.0MiB · remaining: 3.5MiB`,
		"│    GitHub wait: resource=code_search reason=primary-rate-limit · status=403 · retry=1/4 · delay=42s · remaining=0/10 · reset=2026-08-12T13:00:00Z · request-id=ABCD:1234",
		"│  Candidate qualification",
		"│    Inspected 4/40 · qualified: 1/5 · rejected: 3 · filtered: 12 · acquisition: 2.0MiB/12.0MiB · remaining: 10.0MiB",
		"│    Finished · inspected 40/40 · qualified: 9/5 · rejected: 31 · filtered: 105 · acquisition: 8.0MiB/12.0MiB · remaining: 4.0MiB",
		"│  Candidate selection",
		"│    Started · candidates in context: 9",
		"│    Finished · selected: 3/3",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("run output missing %q: %s", want, stdout.String())
		}
	}
}

func TestCollectionRunProgressReportsDetectorCountsTimingAndEstimate(t *testing.T) {
	startedAt := time.Unix(100, 0)
	detectors := []DetectorProgress{
		{ID: "one", State: statePending, QueryPlan: []string{"filename:one"}},
		{ID: "two", State: statePending, QueryPlan: []string{"filename:two"}},
	}
	progress := newCollectionRunProgress(startedAt, detectors, "")
	var output bytes.Buffer
	progress.printStarted(&output, 15*time.Minute, 100, 100000)
	progress.printDetectorStarted(&output, &detectors[0], detectors, "", startedAt, 0)
	detectors[0].State = stateComplete
	detectors[0].Examples = []string{"one", "two", "three"}
	progress.iterations++
	progress.printDetectorFinished(&output, &detectors[0], detectors, "", startedAt, startedAt.Add(2*time.Minute))
	progress.printSummary(&output, detectors, "", startedAt.Add(2*time.Minute), "test completed")
	for _, want := range []string{
		"Run started\n  Detectors: 2\n  Time limit: 15m0s\n  Candidate inspection target: 100\n  Selection packet token limit: 100000",
		"┌─ Detector: one",
		"│  Run position: 1/2 · remaining: 2 · run elapsed: 0s",
		"└─ Complete · state: complete · examples: 3/3 · elapsed: 2m0s",
		"Run progress: 1/2 finished · 1 remaining · 1 attempted · 1 iterations · elapsed: 2m0s · average: 2m0s · estimated remaining: 2m0s",
		"Run finished\n  Reason: test completed\n  Detectors attempted: 1\n  Detectors finished: 1/2\n  Iterations completed: 1\n  Detectors remaining: 1\n  Elapsed: 2m0s\n  Average per finished detector: 2m0s\n  Estimated remaining time: 2m0s",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("run progress output missing %q: %s", want, output.String())
		}
	}
}

func TestRunCommitCreatesAtomicCollectionCheckpoint(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	contents := []byte("example dependency\n")
	agent := fakeResearcher{outcome: acceptedOutcome, write: func(iteration Iteration) error {
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
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	var stdout, stderr bytes.Buffer
	agent := fakeResearcher{outcome: Outcome{Result: "unsuccessful"}, write: func(Iteration) error { return nil }}
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
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableResearcher{}); got != 0 {
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
	agent := fakeResearcher{outcome: acceptedOutcome, write: func(iteration Iteration) error {
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
			if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableResearcher{}); got != 0 {
				t.Fatalf("initialize exit status = %d", got)
			}
			initializeGitRepository(t, root)
			commitGitChanges(t, root)
			agent := fakeResearcher{outcome: acceptedOutcome, write: func(iteration Iteration) error {
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
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	contents := []byte("example dependency\n")
	agent := fakeResearcher{outcome: acceptedOutcome, write: func(iteration Iteration) error {
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
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)

	badAgent := fakeResearcher{outcome: acceptedOutcome, write: func(Iteration) error {
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

func TestRunRemovesEmptyDirectoriesCreatedByRejectedIteration(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, io.Discard, io.Discard, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)

	abandoned := filepath.Join(root, "testdata", "corpus", "example-detector", "abandoned-example", "nested")
	agent := fakeResearcher{outcome: Outcome{Result: "accepted"}, write: func(Iteration) error {
		return os.MkdirAll(abandoned, 0o755)
	}}
	if got := run([]string{"run", "--single", "--progress", progress}, root, io.Discard, io.Discard, agent); got != 1 {
		t.Fatalf("run exit status = %d", got)
	}
	if _, err := os.Stat(filepath.Dir(abandoned)); !os.IsNotExist(err) {
		t.Fatalf("rejected iteration left empty staging directory: %v", err)
	}
}

func TestRunRejectsProtocolThatDisagreesWithCorpus(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	agent := fakeResearcher{outcome: Outcome{Result: "accepted"}, write: func(iteration Iteration) error {
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
	if !strings.Contains(stderr.String(), "research result does not match") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPrefersInProgressAndCanTargetOneDetector(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	p := validTestProgress(
		DetectorProgress{ID: "pending-first", State: statePending, Examples: []string{}},
		DetectorProgress{ID: "in-progress-second", State: stateInProgress, Examples: []string{}},
	)
	if err := writeProgress(progress, p); err != nil {
		t.Fatal(err)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	var ran []string
	agent := fakeResearcher{outcome: Outcome{Result: "unsuccessful"}, write: func(i Iteration) error { ran = append(ran, i.DetectorID); return nil }}
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
	p := validTestProgress(DetectorProgress{ID: "example", State: stateInProgress, Iterations: 6, Examples: []string{}})
	if err := writeProgress(progress, p); err != nil {
		t.Fatal(err)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	var calls int
	agent := fakeResearcher{outcome: Outcome{Result: "unsuccessful"}, write: func(Iteration) error { calls++; return nil }}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--progress", progress}, root, &stdout, &stderr, agent); got != 0 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("agent calls = %d, want 1", calls)
	}
	p, err := readProgress(progress)
	if err != nil {
		t.Fatal(err)
	}
	if p.Detectors[0].Iterations != 7 || p.Detectors[0].State != stateNeedsCollectionReview {
		t.Fatalf("progress = %+v", p.Detectors[0])
	}
}

func TestRunRefusesExistingProgressLockWithoutMutation(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	p := validTestProgress(DetectorProgress{ID: "example", State: statePending})
	if err := writeProgress(progress, p); err != nil {
		t.Fatal(err)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	if err := os.WriteFile(progress+".lock", []byte("active"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--single", "--progress", progress}, root, &stdout, &stderr, unavailableResearcher{}); got != 1 {
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
	p := validTestProgress(DetectorProgress{ID: "example", State: statePending})
	if err := writeProgress(progress, p); err != nil {
		t.Fatal(err)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	originalNow := now
	now = func() time.Time { return time.Unix(100, 0) }
	t.Cleanup(func() { now = originalNow })
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--progress", progress, "--duration", "0s"}, root, &stdout, &stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "soft duration reached") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
