package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeAgent struct {
	write   func(Iteration) error
	outcome Outcome
}

func (f fakeAgent) Run(iteration Iteration) (Outcome, error) {
	return f.outcome, f.write(iteration)
}

var acceptedOutcome = Outcome{Result: "accepted", Added: []string{"owner-repo-abc123/project/dependencies.txt", "owner-repo-abc123/provenance.yaml"}}

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
		return os.WriteFile(filepath.Join(iteration.CorpusDir, "owner-repo-abc123", "provenance.yaml"), []byte("version: 1\n"), 0o644)
	}}
	stdout.Reset()
	stderr.Reset()
	if got := run([]string{"run", "--progress", progress}, root, &stdout, &stderr, agent); got != 0 {
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
	if got := run([]string{"run", "--progress", progress}, root, &stdout, &stderr, badAgent); got != 1 {
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
	if got := run([]string{"run", "--progress", progress}, root, &stdout, &stderr, agent); got != 1 {
		t.Fatalf("run exit status = %d, stderr = %s", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "protocol does not match") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
