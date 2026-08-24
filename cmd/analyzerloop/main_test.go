package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ferretsecurity/deplens/internal/analyzerloop"
)

func TestPlanAcceptsCorpusBaseCommitAncestor(t *testing.T) {
	root := t.TempDir()
	deplens := filepath.Join(root, "deplens")
	gitInit(t, deplens)
	writeFile(t, filepath.Join(deplens, "internal", "analyze", "default_rules.yaml"), "rules")
	gitCommit(t, deplens, "base")
	base := gitOutput(t, deplens, "rev-parse", "HEAD")
	writeFile(t, filepath.Join(deplens, "implementation.txt"), "loop implementation")
	gitCommit(t, deplens, "implementation")

	corpus := filepath.Join(root, "corpus")
	gitInit(t, corpus)
	for index := 1; index <= 3; index++ {
		writeFile(t, filepath.Join(corpus, "testdata", "corpus", "example", fmt.Sprintf("candidate-%d", index), "example.lock"), fmt.Sprintf("source-%d", index))
	}
	verification := fmt.Sprintf(`version: 1
corpus:
  path: testdata/corpus
deplens:
  commit: %s
work_items:
  - id: example
    detector: {id: example, form: lockfile, roles: [resolution]}
    result: OK
    candidates:
      - candidate_id: candidate-1
        original_path: example.lock
        source_sha256: %s
        verdict: valid
      - candidate_id: candidate-2
        original_path: example.lock
        source_sha256: %s
        verdict: valid
      - candidate_id: candidate-3
        original_path: example.lock
        source_sha256: %s
        verdict: valid
`, base, sha256Text("source-1"), sha256Text("source-2"), sha256Text("source-3"))
	writeFile(t, filepath.Join(corpus, ".deplens", "corpus-verification.yaml"), verification)
	gitCommit(t, corpus, "corpus")

	var stdout, stderr bytes.Buffer
	if code := plan([]string{"--corpus", corpus}, deplens, &stdout, &stderr); code != 0 {
		t.Fatalf("plan() = %d, stderr = %s", code, stderr.String())
	}
	ledger, err := analyzerloop.LoadLedger(filepath.Join(deplens, ".deplens", "analyzer-implementation.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Deplens.Commit != base {
		t.Fatalf("ledger base commit = %q, want %q", ledger.Deplens.Commit, base)
	}
}

func TestRunHelpSucceeds(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run --help = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "-follow") {
		t.Fatalf("help output = %q, want -follow flag", stdout.String())
	}
}

func TestProgressRendererFollowIncludesRawLogCommand(t *testing.T) {
	var output bytes.Buffer
	renderer := newProgressRenderer(&output, true, "/repo/.ralph/runs/123")
	renderer.AgentStarted(analyzerloop.Attempt{}, "/repo/.ralph/runs/123/attempts/1.jsonl", "/repo")
	renderer.AgentCommand(analyzerloop.Attempt{}, "/repo", "go test ./...", 0)
	renderer.AgentHeartbeat(analyzerloop.Attempt{}, time.Minute)
	text := output.String()
	if !strings.Contains(text, "tail -f") || !strings.Contains(text, "go test ./...") || !strings.Contains(text, "Agent is still running") {
		t.Fatalf("follow output = %q", text)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitCommit(t *testing.T, dir, message string) {
	t.Helper()
	runGit(t, dir, "add", ".")
	runGit(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", message)
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes.TrimSpace(output))
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func sha256Text(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum)
}

func TestRunSelectionDefaultsToFirstUnfinishedItem(t *testing.T) {
	ledger := analyzerloop.Ledger{WorkItems: []analyzerloop.WorkItem{
		{Number: 1, State: analyzerloop.StateCompleted},
		{Number: 2, State: analyzerloop.StateInProgress},
	}}
	got, err := runSelection("", false, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selection = %v, want %v", got, want)
	}
}

func TestRunSelectionOnceSkipsCompletedSelectedItems(t *testing.T) {
	ledger := analyzerloop.Ledger{WorkItems: []analyzerloop.WorkItem{
		{Number: 1, State: analyzerloop.StateCompleted},
		{Number: 2, State: analyzerloop.StatePending},
	}}
	got, err := runSelection("1,2", true, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selection = %v, want %v", got, want)
	}
}
