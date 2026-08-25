package analyzerloop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAllowedChangedPathsRejectsHarnessAndCorpusChanges(t *testing.T) {
	if err := allowedChangedPaths([]string{"internal/analyze/demo.go", "testdata/demo/fixture", "README.md"}); err != nil {
		t.Fatalf("allowed paths rejected: %v", err)
	}
	if err := allowedChangedPaths([]string{"cmd/analyzerloop/main.go"}); err == nil {
		t.Fatal("harness change was allowed")
	}
	if err := allowedChangedPaths([]string{"testdata/corpus/demo/original"}); err == nil {
		t.Fatal("original corpus copy was allowed")
	}
}

func TestRecognizedCandidateAcceptsCompleteDependencyAndEmptySources(t *testing.T) {
	unsupported := []byte(`{"sources":[{"detector":"demo","analysis":{"presence":"present","extraction":"unsupported"}}]}`)
	if recognizedCandidate(unsupported, "demo") {
		t.Fatal("unsupported source was accepted")
	}
	completeWithoutDependencies := []byte(`{"sources":[{"detector":"demo","analysis":{"presence":"present","extraction":"complete"}}]}`)
	if recognizedCandidate(completeWithoutDependencies, "demo") {
		t.Fatal("dependency-free source was accepted")
	}
	empty := []byte(`{"sources":[{"detector":"demo","analysis":{"presence":"absent","extraction":"complete"}}]}`)
	if !recognizedCandidate(empty, "demo") {
		t.Fatal("complete empty source was rejected")
	}
	emptyWithDependencies := []byte(`{"sources":[{"detector":"demo","analysis":{"presence":"absent","extraction":"complete"},"dependencies":[{"name":"unexpected"}]}]}`)
	if recognizedCandidate(emptyWithDependencies, "demo") {
		t.Fatal("empty source with dependencies was accepted")
	}
	complete := []byte(`{"sources":[{"detector":"demo","analysis":{"presence":"present","extraction":"complete"},"dependencies":[{"name":"example"}]}]}`)
	if !recognizedCandidate(complete, "demo") {
		t.Fatal("complete source was rejected")
	}
}

func TestParseEngineResultReadsCodexAgentMessage(t *testing.T) {
	output := []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"<analyzerloop-result>{\"summary\":\"done\",\"fixtures\":[\"testdata/a\",\"testdata/b\",\"testdata/c\"]}</analyzerloop-result>"}}`)
	result, err := parseEngineResult(output)
	if err != nil {
		t.Fatalf("parseEngineResult: %v", err)
	}
	if result.Summary != "done" || len(result.Fixtures) != 3 {
		t.Fatalf("result = %#v", result)
	}
}

func TestPromptSeparatesImplementerAndVerifierResponsibilities(t *testing.T) {
	workItem := WorkItem{Number: 1, ID: "demo"}
	implementer := prompt("/corpus", Attempt{WorkItem: workItem, Role: RoleImplementer})
	if !strings.Contains(implementer, "Implement a real source analyzer") || !strings.Contains(implementer, "Create exactly three") || !strings.Contains(implementer, "Do not change, remove, or replace existing fixtures") {
		t.Fatalf("implementer prompt missing extraction requirements: %q", implementer)
	}
	if !strings.Contains(implementer, "TestMatchSelectorOnlySourceMatchesSupportedFiles") || !strings.Contains(implementer, "go test ./internal/analyze") {
		t.Fatalf("implementer prompt missing analyzer migration checks: %q", implementer)
	}
	if !strings.Contains(implementer, "analysis presence \"absent\" and extraction \"complete\"") {
		t.Fatalf("implementer prompt missing empty-source requirement: %q", implementer)
	}
	verifier := prompt("/corpus", Attempt{WorkItem: workItem, Role: RoleVerifier})
	if !strings.Contains(verifier, "Do not add, remove, or replace fixtures") || strings.Contains(verifier, "Create exactly three") {
		t.Fatalf("verifier prompt has incorrect fixture instructions: %q", verifier)
	}
	if !strings.Contains(verifier, "TestMatchSelectorOnlySourceIgnoresAnalyzerBackedSources") || !strings.Contains(verifier, "go test ./internal/analyze") {
		t.Fatalf("verifier prompt missing analyzer migration checks: %q", verifier)
	}
	if !strings.Contains(verifier, "analysis presence \"absent\" and extraction \"complete\"") {
		t.Fatalf("verifier prompt missing empty-source requirement: %q", verifier)
	}
}

func TestFixtureDirectoryUsesContainingDirectory(t *testing.T) {
	worktree := filepath.Join("repo", "checkout")
	got := fixtureDirectory(worktree, "testdata/demo/fixture.yaml")
	want := filepath.Join(worktree, "testdata", "demo")
	if got != want {
		t.Fatalf("fixture directory = %q, want %q", got, want)
	}
}

func TestCaptureFixtureOutputsWritesRawHumanAndJSONOutput(t *testing.T) {
	runtimeRoot := t.TempDir()
	attempt := Attempt{WorkItem: WorkItem{Number: 4}, Role: RoleVerifier, Number: 1}
	fixtures := []string{
		"testdata/buf/manifest-v1-deps/buf.yaml",
		"testdata/buf/manifest-v2-module/buf.yaml",
		"testdata/buf/manifest-v2-deps-modules/buf.yaml",
	}
	var calls []string
	run := func(_ context.Context, worktree, fixtureDir string, jsonOutput bool) ([]byte, []byte, error) {
		calls = append(calls, worktree+"|"+fixtureDir+"|"+strconv.FormatBool(jsonOutput))
		if jsonOutput {
			return []byte(`{"sources":[]}`), nil, nil
		}
		return []byte("Found 1 dependency source\n"), nil, nil
	}
	if err := captureFixtureOutputsWithCommand(context.Background(), "/repo", runtimeRoot, attempt, fixtures, nil, run); err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(runtimeRoot, "fixture-output", "work-item-004", "verifier-attempt-1")
	for index, fixture := range fixtures {
		prefix := fmt.Sprintf("%03d-%s", index+1, fixtureOutputLabel(fixture))
		human, err := os.ReadFile(filepath.Join(directory, prefix+".txt"))
		if err != nil || string(human) != "Found 1 dependency source\n" {
			t.Fatalf("human output for %q = %q, %v", fixture, human, err)
		}
		jsonOutput, err := os.ReadFile(filepath.Join(directory, prefix+".json"))
		if err != nil || string(jsonOutput) != `{"sources":[]}` {
			t.Fatalf("JSON output for %q = %q, %v", fixture, jsonOutput, err)
		}
	}
	if len(calls) != 6 || !strings.HasSuffix(calls[0], "|false") || !strings.HasSuffix(calls[1], "|true") {
		t.Fatalf("commands = %v", calls)
	}
}

func TestDirectAttemptSnapshotRestoresRejectedChanges(t *testing.T) {
	ctx := context.Background()
	worktree := t.TempDir()
	runTestGit(t, worktree, "init")
	runTestGit(t, worktree, "config", "user.name", "Test User")
	runTestGit(t, worktree, "config", "user.email", "test@example.com")
	writeTestFile(t, filepath.Join(worktree, ".gitignore"), "/.ralph/\n")
	writeTestFile(t, filepath.Join(worktree, "tracked.txt"), "base\n")
	runTestGit(t, worktree, "add", ".")
	runTestGit(t, worktree, "commit", "-m", "base")

	writeTestFile(t, filepath.Join(worktree, "tracked.txt"), "accepted\n")
	writeTestFile(t, filepath.Join(worktree, "staged.txt"), "accepted\n")
	runTestGit(t, worktree, "add", "staged.txt")
	writeTestFile(t, filepath.Join(worktree, "testdata", "demo", "accepted.txt"), "accepted\n")
	snapshot, err := captureDirectAttempt(ctx, worktree, filepath.Join(worktree, ".ralph"))
	if err != nil {
		t.Fatalf("captureDirectAttempt: %v", err)
	}
	defer os.RemoveAll(snapshot.directory)

	writeTestFile(t, filepath.Join(worktree, "tracked.txt"), "rejected\n")
	writeTestFile(t, filepath.Join(worktree, "staged.txt"), "rejected\n")
	writeTestFile(t, filepath.Join(worktree, "testdata", "demo", "accepted.txt"), "rejected\n")
	writeTestFile(t, filepath.Join(worktree, "testdata", "demo", "rejected.txt"), "rejected\n")
	runTestGit(t, worktree, "add", "-A")

	if err := snapshot.restore(ctx); err != nil {
		t.Fatalf("snapshot.restore: %v", err)
	}
	assertTestFile(t, filepath.Join(worktree, "tracked.txt"), "accepted\n")
	assertTestFile(t, filepath.Join(worktree, "staged.txt"), "accepted\n")
	assertTestFile(t, filepath.Join(worktree, "testdata", "demo", "accepted.txt"), "accepted\n")
	if _, err := os.Stat(filepath.Join(worktree, "testdata", "demo", "rejected.txt")); !os.IsNotExist(err) {
		t.Fatalf("rejected fixture still exists, stat error = %v", err)
	}
	if output := runTestGit(t, worktree, "diff", "--cached", "--name-only"); output != "staged.txt\n" {
		t.Fatalf("staged files = %q, want staged.txt", output)
	}
}

func TestDirectAttemptSnapshotReportsOnlyAttemptChanges(t *testing.T) {
	ctx := context.Background()
	worktree := t.TempDir()
	runTestGit(t, worktree, "init")
	runTestGit(t, worktree, "config", "user.name", "Test User")
	runTestGit(t, worktree, "config", "user.email", "test@example.com")
	writeTestFile(t, filepath.Join(worktree, ".gitignore"), "/.ralph/\n")
	writeTestFile(t, filepath.Join(worktree, ".deplens", "analyzer-implementation.yaml"), "state: pending\n")
	writeTestFile(t, filepath.Join(worktree, "internal", "analyze", "buf.go"), "package analyze\n")
	runTestGit(t, worktree, "add", ".")
	runTestGit(t, worktree, "commit", "-m", "base")

	// These changes are the accepted implementer checkpoint and its ledger
	// transition. A verifier must not be held responsible for either one.
	writeTestFile(t, filepath.Join(worktree, ".deplens", "analyzer-implementation.yaml"), "state: in_progress\n")
	writeTestFile(t, filepath.Join(worktree, "internal", "analyze", "buf.go"), "package analyze\n\nfunc Buf() {}\n")
	snapshot, err := captureDirectAttempt(ctx, worktree, filepath.Join(worktree, ".ralph"))
	if err != nil {
		t.Fatalf("captureDirectAttempt: %v", err)
	}
	defer os.RemoveAll(snapshot.directory)

	paths, err := snapshot.changedPaths(ctx)
	if err != nil {
		t.Fatalf("snapshot.changedPaths: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("attempt paths = %q, want none", paths)
	}

	writeTestFile(t, filepath.Join(worktree, ".deplens", "analyzer-implementation.yaml"), "state: completed\n")
	paths, err = snapshot.changedPaths(ctx)
	if err != nil {
		t.Fatalf("snapshot.changedPaths after ledger edit: %v", err)
	}
	if got, want := strings.Join(paths, ","), ".deplens/analyzer-implementation.yaml"; got != want {
		t.Fatalf("attempt paths = %q, want %q", got, want)
	}
	if err := allowedChangedPaths(paths); err == nil {
		t.Fatal("verifier ledger edit was allowed")
	}

	if err := snapshot.restore(ctx); err != nil {
		t.Fatalf("snapshot.restore: %v", err)
	}
	writeTestFile(t, filepath.Join(worktree, "internal", "analyze", "buf.go"), "package analyze\n\nfunc Buf() { _ = 1 }\n")
	paths, err = snapshot.changedPaths(ctx)
	if err != nil {
		t.Fatalf("snapshot.changedPaths after analyzer edit: %v", err)
	}
	if got, want := strings.Join(paths, ","), "internal/analyze/buf.go"; got != want {
		t.Fatalf("attempt paths = %q, want %q", got, want)
	}
	if err := allowedChangedPaths(paths); err != nil {
		t.Fatalf("verifier analyzer edit was rejected: %v", err)
	}
}

func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func assertTestFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("contents of %q = %q, want %q", path, got, want)
	}
}

func TestAgentEventExtractsFollowDetails(t *testing.T) {
	message, command := agentEvent([]byte(`{"type":"item.completed","item":{"type":"agent_message","text":"checking fixture"}}`))
	if message != "checking fixture" || command != (agentCommand{}) {
		t.Fatalf("agent message = (%q, %#v)", message, command)
	}
	message, command = agentEvent([]byte(`{"type":"item.completed","item":{"type":"command_execution","command":"go test ./...","exit_code":1}}`))
	if message != "" || command.Command != "go test ./..." || command.ExitCode != 1 {
		t.Fatalf("agent command = (%q, %#v)", message, command)
	}
}
