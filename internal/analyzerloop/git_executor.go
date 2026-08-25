package analyzerloop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// CodexEngine is the narrow production adapter. It receives a fresh prompt
// and returns only the deliberately small structured completion result.
type CodexEngine struct {
	Executable string
	Workdir    string
	CorpusRoot string
	OutputPath string
	Timeout    time.Duration
	Progress   ProgressReporter
}

func (e CodexEngine) Execute(ctx context.Context, attempt Attempt) (AttemptResult, error) {
	if e.Workdir == "" || e.CorpusRoot == "" {
		return AttemptResult{}, errors.New("Codex engine requires worktree and corpus paths")
	}
	if e.Executable == "" {
		e.Executable = "codex"
	}
	if e.Timeout == 0 {
		e.Timeout = 45 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()
	command := exec.CommandContext(ctx, e.Executable,
		"exec", "--json", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--model", "gpt-5.6-terra", "--sandbox", "workspace-write",
		"--config", "model_reasoning_effort=high", "--config", "approval_policy=\"never\"", "--config", "mcp_servers={}", prompt(e.CorpusRoot, attempt))
	command.Dir = e.Workdir
	var output bytes.Buffer
	var capture *engineOutputCapture
	if e.OutputPath != "" {
		file, openErr := os.OpenFile(e.OutputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if openErr != nil {
			return AttemptResult{}, fmt.Errorf("open Codex output: %w", openErr)
		}
		defer file.Close()
		capture = &engineOutputCapture{output: &output, file: file, progress: e.Progress, attempt: attempt, workdir: e.Workdir}
	} else {
		capture = &engineOutputCapture{output: &output, progress: e.Progress, attempt: attempt, workdir: e.Workdir}
	}
	stdout, pipeErr := command.StdoutPipe()
	if pipeErr != nil {
		return AttemptResult{}, fmt.Errorf("open Codex stdout: %w", pipeErr)
	}
	stderr, pipeErr := command.StderrPipe()
	if pipeErr != nil {
		return AttemptResult{}, fmt.Errorf("open Codex stderr: %w", pipeErr)
	}
	report(e.Progress, func(progress ProgressReporter) { progress.AgentStarted(attempt, e.OutputPath, e.Workdir) })
	started := time.Now()
	if err := command.Start(); err != nil {
		return AttemptResult{}, fmt.Errorf("start isolated Codex attempt: %w", err)
	}
	var copies sync.WaitGroup
	copies.Add(2)
	go func() { defer copies.Done(); _, _ = io.Copy(capture, stdout) }()
	go func() { defer copies.Done(); _, _ = io.Copy(capture, stderr) }()
	done := make(chan struct{})
	go reportHeartbeats(done, e.Progress, attempt, started)
	err := command.Wait()
	close(done)
	copies.Wait()
	report(e.Progress, func(progress ProgressReporter) { progress.AgentFinished(attempt, time.Since(started)) })
	outputBytes := output.Bytes()
	if err != nil {
		if ctx.Err() != nil {
			return AttemptResult{}, ctx.Err()
		}
		return AttemptResult{}, fmt.Errorf("run isolated Codex attempt: %w", err)
	}
	result, err := parseEngineResult(outputBytes)
	if err != nil {
		return AttemptResult{}, err
	}
	return result, nil
}

func reportHeartbeats(done <-chan struct{}, progress ProgressReporter, attempt Attempt, started time.Time) {
	if progress == nil {
		return
	}
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	select {
	case <-done:
		return
	case <-timer.C:
		report(progress, func(reporter ProgressReporter) { reporter.AgentHeartbeat(attempt, time.Since(started)) })
	}
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			report(progress, func(reporter ProgressReporter) { reporter.AgentHeartbeat(attempt, time.Since(started)) })
		}
	}
}

type engineOutputCapture struct {
	mu       sync.Mutex
	output   *bytes.Buffer
	file     *os.File
	pending  []byte
	progress ProgressReporter
	attempt  Attempt
	workdir  string
}

func (capture *engineOutputCapture) Write(data []byte) (int, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.file != nil {
		if _, err := capture.file.Write(data); err != nil {
			return 0, err
		}
	}
	_, _ = capture.output.Write(data)
	capture.pending = append(capture.pending, data...)
	for {
		lineEnd := bytes.IndexByte(capture.pending, '\n')
		if lineEnd < 0 {
			break
		}
		line := capture.pending[:lineEnd]
		capture.pending = capture.pending[lineEnd+1:]
		message, command := agentEvent(line)
		if message != "" {
			report(capture.progress, func(progress ProgressReporter) { progress.AgentMessage(capture.attempt, message) })
		}
		if command.Command != "" {
			report(capture.progress, func(progress ProgressReporter) {
				progress.AgentCommand(capture.attempt, capture.workdir, command.Command, command.ExitCode)
			})
		}
	}
	return len(data), nil
}

type agentCommand struct {
	Command  string
	ExitCode int
}

func agentEvent(line []byte) (string, agentCommand) {
	var event struct {
		Type string `json:"type"`
		Item struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Command  string `json:"command"`
			ExitCode int    `json:"exit_code"`
		} `json:"item"`
	}
	if err := json.Unmarshal(line, &event); err != nil || event.Type != "item.completed" {
		return "", agentCommand{}
	}
	if event.Item.Type == "agent_message" {
		return strings.TrimSpace(event.Item.Text), agentCommand{}
	}
	if event.Item.Type == "command_execution" {
		return "", agentCommand{Command: strings.TrimSpace(event.Item.Command), ExitCode: event.Item.ExitCode}
	}
	return "", agentCommand{}
}

func prompt(corpusRoot string, attempt Attempt) string {
	common := fmt.Sprintf(`You are the %s for dependency detector %q (work item %d).
Work only in the current detached worktree. You may read the three original candidates from %q.
Do not change the loop harness, ledger, corpus repository, Go modules, or unrelated files. Never copy originals or provenance. Follow repository conventions, including updating README.md for the detector when required.
If this change makes a previously selector-only rule analyzer-backed, update the selector-only regression coverage: remove the filename from TestMatchSelectorOnlySourceMatchesSupportedFiles and add it to TestMatchSelectorOnlySourceIgnoresAnalyzerBackedSources. Do the equivalent for any existing repository test that distinguishes selector-only from analyzer-backed rules.`, attempt.Role, attempt.WorkItem.ID, attempt.WorkItem.Number, corpusRoot)
	var roleInstructions string
	switch attempt.Role {
	case RoleImplementer:
		roleInstructions = `
Implement a real source analyzer for this detector. A filename-only match, a rule-only change, or fixtures without extracted dependencies is not a solution.
Inspect all three originals and determine the dependency references the analyzer must extract. Add or update the analyzer, its registration, rule configuration, and focused tests as needed. The analyzer must report at least one dependency for each original with analysis presence "present" and extraction "complete".
Create exactly three new minimized synthetic fixtures under testdata, one for each original's distinct dependency pattern. Do not change, remove, or replace existing fixtures. Report each of the three new fixture paths. Add focused tests that assert their extracted dependency references. Do not merely assert that the files are detected.`
	case RoleVerifier:
		roleInstructions = `
Verify the existing implementation as a dependency extractor, not merely as a filename detector. Inspect all three originals and the three existing minimized fixtures. Confirm that each produces at least one extracted dependency with analysis presence "present" and extraction "complete", and that focused tests assert the expected references.
Do not add, remove, or replace fixtures. Repair the analyzer, registration, rule configuration, tests, or documentation only when necessary to make the extraction correct.`
	default:
		roleInstructions = "\nVerify the dependency extractor according to repository conventions."
	}
	return common + roleInstructions + `
Before reporting success, run go test ./internal/analyze. The harness runs the same package-level test suite; tests limited to files you added are not sufficient.
At the end, output exactly one line and nothing after it:
<analyzerloop-result>{"summary":"short description","fixtures":["testdata/...","testdata/...","testdata/..."]}</analyzerloop-result>`
}

func parseEngineResult(output []byte) (AttemptResult, error) {
	const open, close = "<analyzerloop-result>", "</analyzerloop-result>"
	text := string(output)
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		message, _ := agentEvent(line)
		if strings.Contains(message, open) {
			text = message
		}
	}
	start := strings.LastIndex(text, open)
	if start < 0 {
		return AttemptResult{}, errors.New("Codex did not return an analyzer loop result")
	}
	end := strings.Index(text[start+len(open):], close)
	if end < 0 {
		return AttemptResult{}, errors.New("Codex returned an unterminated analyzer loop result")
	}
	var result AttemptResult
	if err := json.Unmarshal([]byte(text[start+len(open):start+len(open)+end]), &result); err != nil {
		return AttemptResult{}, fmt.Errorf("parse Codex analyzer loop result: %w", err)
	}
	return result, nil
}

// GitWorktreeExecutor isolates an attempt, validates its diff, then applies a
// validated patch to the target branch. The caller commits the subsequent
// ledger transaction before the next attempt starts.
type GitWorktreeExecutor struct {
	RepositoryRoot string
	CorpusRoot     string
	RuntimeRoot    string
	Engine         CodexEngine
}

// DirectExecutor is used only for the explicitly requested --no-commit mode.
// It retains the two fresh agent sessions but leaves their accepted changes in
// the target worktree so the verifier can inspect the implementer checkpoint.
type DirectExecutor struct {
	RepositoryRoot string
	CorpusRoot     string
	RuntimeRoot    string
	Engine         CodexEngine
}

func (e DirectExecutor) Execute(ctx context.Context, attempt Attempt) (result AttemptResult, err error) {
	snapshot, err := captureDirectAttempt(ctx, e.RepositoryRoot, e.RuntimeRoot)
	if err != nil {
		return AttemptResult{}, err
	}
	accepted := false
	defer func() {
		if accepted {
			_ = os.RemoveAll(snapshot.directory)
			return
		}
		if restoreErr := snapshot.restore(ctx); restoreErr != nil {
			err = fmt.Errorf("restore rejected direct attempt (snapshot %q): %w", snapshot.directory, restoreErr)
			return
		}
		_ = os.RemoveAll(snapshot.directory)
	}()

	beforeFixtures, err := untrackedTestdata(ctx, e.RepositoryRoot)
	if err != nil {
		return AttemptResult{}, err
	}
	outputPath := filepath.Join(e.RuntimeRoot, "attempts", fmt.Sprintf("%03d-%s-%d.jsonl", attempt.WorkItem.Number, attempt.Role, attempt.Number))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return AttemptResult{}, fmt.Errorf("create attempt output directory: %w", err)
	}
	engine := e.Engine
	engine.Workdir, engine.CorpusRoot, engine.OutputPath = e.RepositoryRoot, e.CorpusRoot, outputPath
	result, err = engine.Execute(ctx, attempt)
	if err != nil {
		return AttemptResult{}, err
	}
	paths, err := snapshot.changedPaths(ctx)
	if err != nil {
		return AttemptResult{}, fmt.Errorf("list direct attempt delta: %w", err)
	}
	report(engine.Progress, func(progress ProgressReporter) { progress.AgentEdited(attempt, paths) })
	report(engine.Progress, func(progress ProgressReporter) { progress.ValidationStarted(attempt) })
	if err := validateWorktree(ctx, e.RepositoryRoot, e.CorpusRoot, attempt); err != nil {
		return AttemptResult{}, err
	}
	if err := allowedChangedPaths(paths); err != nil {
		return AttemptResult{}, err
	}
	afterFixtures, err := untrackedTestdata(ctx, e.RepositoryRoot)
	if err != nil {
		return AttemptResult{}, err
	}
	if err := verifyFixtures(e.RepositoryRoot, attempt, result, paths, addedPaths(beforeFixtures, afterFixtures)); err != nil {
		return AttemptResult{}, err
	}
	if err := validateFixtures(ctx, e.RepositoryRoot, attempt, result.Fixtures); err != nil {
		return AttemptResult{}, err
	}
	if attempt.Role == RoleVerifier {
		if err := captureFixtureOutputs(ctx, e.RepositoryRoot, e.RuntimeRoot, attempt, result.Fixtures, engine.Progress); err != nil {
			return AttemptResult{}, err
		}
	}
	result.ChangedPaths = paths
	report(engine.Progress, func(progress ProgressReporter) { progress.ValidationAccepted(attempt, paths) })
	accepted = true
	return result, nil
}

func (e GitWorktreeExecutor) Execute(ctx context.Context, attempt Attempt) (AttemptResult, error) {
	if err := os.MkdirAll(filepath.Join(e.RuntimeRoot, "worktrees"), 0o700); err != nil {
		return AttemptResult{}, fmt.Errorf("create attempt worktree directory: %w", err)
	}
	worktree, err := os.MkdirTemp(filepath.Join(e.RuntimeRoot, "worktrees"), fmt.Sprintf("%03d-%s-", attempt.WorkItem.Number, attempt.Role))
	if err != nil {
		return AttemptResult{}, fmt.Errorf("create attempt worktree path: %w", err)
	}
	if err := os.Remove(worktree); err != nil {
		return AttemptResult{}, fmt.Errorf("prepare attempt worktree path: %w", err)
	}
	if _, err := git(ctx, e.RepositoryRoot, "worktree", "add", "--detach", worktree, "HEAD"); err != nil {
		return AttemptResult{}, fmt.Errorf("create detached attempt worktree: %w", err)
	}
	defer func() { _, _ = git(context.Background(), e.RepositoryRoot, "worktree", "remove", "--force", worktree) }()

	outputPath := filepath.Join(e.RuntimeRoot, "attempts", fmt.Sprintf("%03d-%s-%d.jsonl", attempt.WorkItem.Number, attempt.Role, attempt.Number))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return AttemptResult{}, fmt.Errorf("create attempt output directory: %w", err)
	}
	engine := e.Engine
	engine.Workdir, engine.CorpusRoot, engine.OutputPath = worktree, e.CorpusRoot, outputPath
	result, err := engine.Execute(ctx, attempt)
	if err != nil {
		return AttemptResult{}, err
	}
	if err := includeUntracked(ctx, worktree); err != nil {
		return AttemptResult{}, err
	}
	edited, err := changedPaths(ctx, worktree)
	if err != nil {
		return AttemptResult{}, fmt.Errorf("list attempt changes: %w", err)
	}
	report(engine.Progress, func(progress ProgressReporter) { progress.AgentEdited(attempt, edited) })
	report(engine.Progress, func(progress ProgressReporter) { progress.ValidationStarted(attempt) })
	if err := validateWorktree(ctx, worktree, e.CorpusRoot, attempt); err != nil {
		return AttemptResult{}, err
	}
	paths, err := gitLines(ctx, worktree, "diff", "--name-only")
	if err != nil {
		return AttemptResult{}, fmt.Errorf("list attempt changes: %w", err)
	}
	if err := allowedChangedPaths(paths); err != nil {
		return AttemptResult{}, err
	}
	newFixtures, err := addedTestdata(ctx, worktree)
	if err != nil {
		return AttemptResult{}, err
	}
	if err := verifyFixtures(worktree, attempt, result, paths, newFixtures); err != nil {
		return AttemptResult{}, err
	}
	if err := validateFixtures(ctx, worktree, attempt, result.Fixtures); err != nil {
		return AttemptResult{}, err
	}
	if attempt.Role == RoleVerifier {
		if err := captureFixtureOutputs(ctx, worktree, e.RuntimeRoot, attempt, result.Fixtures, engine.Progress); err != nil {
			return AttemptResult{}, err
		}
	}
	patch, err := git(ctx, worktree, "diff", "--binary")
	if err != nil {
		return AttemptResult{}, fmt.Errorf("create validated attempt patch: %w", err)
	}
	if len(bytes.TrimSpace(patch)) == 0 && attempt.Role == RoleImplementer {
		return AttemptResult{}, errors.New("attempt produced no changes")
	}
	if len(bytes.TrimSpace(patch)) == 0 {
		result.ChangedPaths = paths
		report(engine.Progress, func(progress ProgressReporter) { progress.ValidationAccepted(attempt, paths) })
		return result, nil
	}
	apply := exec.CommandContext(ctx, "git", "apply", "--whitespace=error")
	apply.Dir, apply.Stdin = e.RepositoryRoot, bytes.NewReader(patch)
	if output, err := apply.CombinedOutput(); err != nil {
		return AttemptResult{}, fmt.Errorf("apply validated attempt patch: %w: %s", err, strings.TrimSpace(string(output)))
	}
	result.ChangedPaths = paths
	report(engine.Progress, func(progress ProgressReporter) { progress.ValidationAccepted(attempt, paths) })
	return result, nil
}

func validateWorktree(ctx context.Context, worktree, corpusRoot string, attempt Attempt) error {
	paths, err := changedPaths(ctx, worktree)
	if err != nil {
		return fmt.Errorf("list changed Go files: %w", err)
	}
	paths = filterGoPaths(paths)
	if len(paths) > 0 {
		args := append([]string{"-w"}, paths...)
		if output, err := command(ctx, worktree, "gofmt", args...); err != nil {
			return fmt.Errorf("format changed Go files: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	if output, err := command(ctx, worktree, "go", "test", "./internal/analyze"); err != nil {
		return fmt.Errorf("run analyzer package tests: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if attempt.Role == RoleVerifier {
		if output, err := command(ctx, worktree, "go", "test", "./..."); err != nil {
			return fmt.Errorf("run full test suite: %w: %s", err, strings.TrimSpace(string(output)))
		}
		if output, err := command(ctx, worktree, "go", "vet", "./..."); err != nil {
			return fmt.Errorf("run go vet: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	for _, candidate := range attempt.WorkItem.Candidates {
		path := filepath.Join(corpusRoot, attempt.WorkItem.ID, candidate.ID)
		if err := validateExtractedSource(ctx, worktree, path, attempt.WorkItem.ID); err != nil {
			return fmt.Errorf("original candidate %q: %w", candidate.ID, err)
		}
	}
	return nil
}

func validateFixtures(ctx context.Context, worktree string, attempt Attempt, fixtures []string) error {
	for _, fixture := range fixtures {
		path := fixtureDirectory(worktree, fixture)
		if err := validateExtractedSource(ctx, worktree, path, attempt.WorkItem.ID); err != nil {
			return fmt.Errorf("fixture %q: %w", fixture, err)
		}
	}
	return nil
}

type fixtureOutputCommand func(context.Context, string, string, bool) ([]byte, []byte, error)

// captureFixtureOutputs records the exact CLI stdout for every accepted fixture.
// It runs in the attempt worktree before that verifier checkpoint is applied.
func captureFixtureOutputs(ctx context.Context, worktree, runtimeRoot string, attempt Attempt, fixtures []string, progress ProgressReporter) error {
	return captureFixtureOutputsWithCommand(ctx, worktree, runtimeRoot, attempt, fixtures, progress, runFixtureOutputCommand)
}

func captureFixtureOutputsWithCommand(ctx context.Context, worktree, runtimeRoot string, attempt Attempt, fixtures []string, progress ProgressReporter, run fixtureOutputCommand) error {
	directory := filepath.Join(runtimeRoot, "fixture-output", fmt.Sprintf("work-item-%03d", attempt.WorkItem.Number), fmt.Sprintf("verifier-attempt-%d", attempt.Number))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create fixture output directory: %w", err)
	}
	report(progress, func(progress ProgressReporter) { progress.FixtureOutputStarted(attempt, directory) })
	for index, fixture := range fixtures {
		fixtureDir := fixtureDirectory(worktree, fixture)
		prefix := fmt.Sprintf("%03d-%s", index+1, fixtureOutputLabel(fixture))
		humanPath := filepath.Join(directory, prefix+".txt")
		jsonPath := filepath.Join(directory, prefix+".json")
		for _, output := range []struct {
			json bool
			path string
			kind string
		}{
			{path: humanPath, kind: "human-readable"},
			{json: true, path: jsonPath, kind: "JSON"},
		} {
			stdout, stderr, err := run(ctx, worktree, fixtureDir, output.json)
			if err != nil {
				message := strings.TrimSpace(string(stderr))
				if message != "" {
					return fmt.Errorf("capture %s CLI output for fixture %q: %w: %s", output.kind, fixture, err, message)
				}
				return fmt.Errorf("capture %s CLI output for fixture %q: %w", output.kind, fixture, err)
			}
			if err := os.WriteFile(output.path, stdout, 0o600); err != nil {
				return fmt.Errorf("write %s CLI output for fixture %q: %w", output.kind, fixture, err)
			}
		}
		report(progress, func(progress ProgressReporter) { progress.FixtureOutputSaved(attempt, fixture, humanPath, jsonPath) })
	}
	report(progress, func(progress ProgressReporter) { progress.FixtureOutputFinished(attempt, len(fixtures)) })
	return nil
}

func runFixtureOutputCommand(ctx context.Context, worktree, fixtureDir string, jsonOutput bool) ([]byte, []byte, error) {
	args := []string{"run", "./cmd/deplens"}
	if jsonOutput {
		args = append(args, "--json")
	}
	args = append(args, fixtureDir)
	command := exec.CommandContext(ctx, "go", args...)
	var stdout, stderr bytes.Buffer
	command.Dir, command.Stdout, command.Stderr = worktree, &stdout, &stderr
	return stdout.Bytes(), stderr.Bytes(), command.Run()
}

func fixtureOutputLabel(fixture string) string {
	label := filepath.ToSlash(fixture)
	label = strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(label)
	return strings.Trim(label, "-.")
}

func fixtureDirectory(worktree, fixture string) string {
	return filepath.Dir(filepath.Join(worktree, filepath.FromSlash(fixture)))
}

func validateExtractedSource(ctx context.Context, worktree, path, detector string) error {
	output, err := command(ctx, worktree, "go", "run", "./cmd/deplens", "--json", path)
	if err != nil {
		if message := strings.TrimSpace(string(output)); message != "" {
			return fmt.Errorf("scan dependency source: %w: %s", err, message)
		}
		return fmt.Errorf("scan dependency source: %w", err)
	}
	if !recognizedCandidate(output, detector) {
		return fmt.Errorf("does not produce complete dependency extraction for detector %q", detector)
	}
	return nil
}

func recognizedCandidate(output []byte, detector string) bool {
	var scan struct {
		Sources []struct {
			Detector     string            `json:"detector"`
			Dependencies []json.RawMessage `json:"dependencies"`
			Analysis     struct {
				Presence   string `json:"presence"`
				Extraction string `json:"extraction"`
			} `json:"analysis"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(output, &scan); err != nil {
		return false
	}
	for _, source := range scan.Sources {
		if source.Detector == detector && source.Analysis.Presence == "present" && source.Analysis.Extraction == "complete" && len(source.Dependencies) > 0 {
			return true
		}
	}
	return false
}

func verifyFixtures(worktree string, attempt Attempt, result AttemptResult, changed, added []string) error {
	if err := validateAttemptResult(result); err != nil {
		return err
	}
	for _, fixture := range result.Fixtures {
		info, err := os.Stat(filepath.Join(worktree, filepath.FromSlash(fixture)))
		if err != nil {
			return fmt.Errorf("recorded fixture %q does not exist: %w", fixture, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("recorded fixture %q is not a regular file", fixture)
		}
	}
	fixtureSet := make(map[string]bool, len(result.Fixtures))
	for _, fixture := range result.Fixtures {
		fixtureSet[filepath.ToSlash(fixture)] = true
	}
	for _, path := range changed {
		path = filepath.ToSlash(path)
		if strings.HasPrefix(path, "testdata/") && !fixtureSet[path] {
			return fmt.Errorf("attempt changed unrecorded fixture %q", path)
		}
	}
	if attempt.Role != RoleImplementer {
		if len(added) != 0 {
			return fmt.Errorf("verifier added fixture %q", added[0])
		}
		return nil
	}
	if len(added) != len(result.Fixtures) {
		return fmt.Errorf("implementer added %d fixtures, want exactly 3", len(added))
	}
	for _, fixture := range result.Fixtures {
		if !containsPath(added, fixture) {
			return fmt.Errorf("implementer fixture %q was not newly created", fixture)
		}
	}
	return nil
}

func containsPath(paths []string, wanted string) bool {
	wanted = filepath.ToSlash(wanted)
	for _, path := range paths {
		if filepath.ToSlash(path) == wanted {
			return true
		}
	}
	return false
}

func filterGoPaths(paths []string) []string {
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.HasSuffix(path, ".go") {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

func allowedChangedPaths(paths []string) error {
	for _, path := range paths {
		path = filepath.ToSlash(path)
		if strings.HasPrefix(path, "testdata/corpus/") || path == "cmd/analyzerloop" || strings.HasPrefix(path, "cmd/analyzerloop/") || path == ".deplens/analyzer-implementation.yaml" || path == "go.mod" || path == "go.sum" {
			return fmt.Errorf("attempt changed forbidden path %q", path)
		}
		if strings.HasPrefix(path, "internal/analyze/") || strings.HasPrefix(path, "testdata/") || path == "README.md" || path == "DEPENDENCY_COVERAGE.md" || path == "internal/analyze/default_rules.yaml" {
			continue
		}
		return fmt.Errorf("attempt changed out-of-scope path %q", path)
	}
	return nil
}

func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return command(ctx, dir, "git", args...)
}

func gitLines(ctx context.Context, dir string, args ...string) ([]string, error) {
	output, err := git(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(output)), nil
}

func includeUntracked(ctx context.Context, dir string) error {
	paths, err := gitLines(ctx, dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return fmt.Errorf("list untracked attempt files: %w", err)
	}
	if len(paths) == 0 {
		return nil
	}
	if output, err := git(ctx, dir, append([]string{"add", "-N", "--"}, paths...)...); err != nil {
		return fmt.Errorf("include untracked attempt files in patch: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func changedPaths(ctx context.Context, dir string) ([]string, error) {
	paths, err := gitLines(ctx, dir, "diff", "--name-only")
	if err != nil {
		return nil, err
	}
	cached, err := gitLines(ctx, dir, "diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}
	untracked, err := gitLines(ctx, dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	return uniquePaths(append(append(paths, cached...), untracked...)), nil
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.ToSlash(path)
		if seen[path] {
			continue
		}
		seen[path] = true
		unique = append(unique, path)
	}
	sort.Strings(unique)
	return unique
}

func addedTestdata(ctx context.Context, dir string) ([]string, error) {
	output, err := git(ctx, dir, "diff", "--name-only", "--diff-filter=A", "--", "testdata")
	if err != nil {
		return nil, fmt.Errorf("list newly added fixtures: %w", err)
	}
	return strings.Fields(string(output)), nil
}

func untrackedTestdata(ctx context.Context, dir string) ([]string, error) {
	paths, err := gitLines(ctx, dir, "ls-files", "--others", "--exclude-standard", "--", "testdata")
	if err != nil {
		return nil, fmt.Errorf("list untracked fixtures: %w", err)
	}
	return paths, nil
}

func addedPaths(before, after []string) []string {
	known := make(map[string]bool, len(before))
	for _, path := range before {
		known[filepath.ToSlash(path)] = true
	}
	added := make([]string, 0, len(after))
	for _, path := range after {
		if !known[filepath.ToSlash(path)] {
			added = append(added, path)
		}
	}
	return added
}

// directAttemptSnapshot restores a rejected direct attempt to the exact
// worktree state at its start. Accepted attempts deliberately remain visible
// so a verifier can inspect and improve the implementer checkpoint.
type directAttemptSnapshot struct {
	worktree       string
	directory      string
	stagedPatch    []byte
	unstagedPatch  []byte
	untrackedPaths []string
	pathStates     map[string]directPathState
}

type directPathState struct {
	worktree string
	index    string
}

func captureDirectAttempt(ctx context.Context, worktree, runtimeRoot string) (directAttemptSnapshot, error) {
	if err := os.MkdirAll(runtimeRoot, 0o700); err != nil {
		return directAttemptSnapshot{}, fmt.Errorf("create direct attempt runtime directory: %w", err)
	}
	directory, err := os.MkdirTemp(runtimeRoot, ".attempt-state-")
	if err != nil {
		return directAttemptSnapshot{}, fmt.Errorf("create direct attempt snapshot: %w", err)
	}
	snapshot := directAttemptSnapshot{worktree: worktree, directory: directory}
	fail := func(err error) (directAttemptSnapshot, error) {
		_ = os.RemoveAll(directory)
		return directAttemptSnapshot{}, err
	}
	if snapshot.stagedPatch, err = git(ctx, worktree, "diff", "--cached", "--binary"); err != nil {
		return fail(fmt.Errorf("capture staged direct attempt changes: %w", err))
	}
	if snapshot.unstagedPatch, err = git(ctx, worktree, "diff", "--binary"); err != nil {
		return fail(fmt.Errorf("capture unstaged direct attempt changes: %w", err))
	}
	if snapshot.untrackedPaths, err = gitLines(ctx, worktree, "ls-files", "--others", "--exclude-standard"); err != nil {
		return fail(fmt.Errorf("list untracked direct attempt files: %w", err))
	}
	paths, err := changedPaths(ctx, worktree)
	if err != nil {
		return fail(fmt.Errorf("list baseline direct attempt changes: %w", err))
	}
	snapshot.pathStates, err = directPathStates(ctx, worktree, paths)
	if err != nil {
		return fail(fmt.Errorf("capture baseline direct attempt changes: %w", err))
	}
	for _, path := range snapshot.untrackedPaths {
		from, err := safeSnapshotPath(worktree, path)
		if err != nil {
			return fail(err)
		}
		to, err := safeSnapshotPath(directory, path)
		if err != nil {
			return fail(err)
		}
		if err := copySnapshotFile(from, to); err != nil {
			return fail(fmt.Errorf("snapshot untracked file %q: %w", path, err))
		}
	}
	return snapshot, nil
}

// changedPaths returns only the Git-visible changes made after the snapshot.
// The target worktree may already contain an accepted checkpoint and a ledger
// update when --no-commit starts the verifier.
func (snapshot directAttemptSnapshot) changedPaths(ctx context.Context) ([]string, error) {
	current, err := changedPaths(ctx, snapshot.worktree)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(snapshot.pathStates)+len(current))
	for path := range snapshot.pathStates {
		paths = append(paths, path)
	}
	paths = append(paths, current...)
	paths = uniquePaths(paths)

	changed := make([]string, 0, len(paths))
	for _, path := range paths {
		before, existed := snapshot.pathStates[path]
		after, err := directPathStateFor(ctx, snapshot.worktree, path)
		if err != nil {
			return nil, fmt.Errorf("inspect %q: %w", path, err)
		}
		if !existed || before != after {
			changed = append(changed, path)
		}
	}
	return changed, nil
}

func directPathStates(ctx context.Context, worktree string, paths []string) (map[string]directPathState, error) {
	states := make(map[string]directPathState, len(paths))
	for _, path := range paths {
		state, err := directPathStateFor(ctx, worktree, path)
		if err != nil {
			return nil, fmt.Errorf("inspect %q: %w", path, err)
		}
		states[path] = state
	}
	return states, nil
}

func directPathStateFor(ctx context.Context, worktree, path string) (directPathState, error) {
	file, err := directFileState(filepath.Join(worktree, filepath.FromSlash(path)))
	if err != nil {
		return directPathState{}, err
	}
	index, err := git(ctx, worktree, "ls-files", "--stage", "--", path)
	if err != nil {
		return directPathState{}, err
	}
	return directPathState{worktree: file, index: strings.TrimSpace(string(index))}, nil
}

func directFileState(path string) (string, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "absent", nil
	}
	if err != nil {
		return "", err
	}
	var content []byte
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		content = []byte(target)
	} else if info.Mode().IsRegular() {
		content, err = os.ReadFile(path)
		if err != nil {
			return "", err
		}
	}
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%s:%x", info.Mode(), sum), nil
}

func (snapshot directAttemptSnapshot) restore(ctx context.Context) error {
	if _, err := git(ctx, snapshot.worktree, "reset", "--quiet", "--", "."); err != nil {
		return fmt.Errorf("reset tracked files: %w", err)
	}
	if _, err := git(ctx, snapshot.worktree, "restore", "--worktree", "--", "."); err != nil {
		return fmt.Errorf("restore tracked files: %w", err)
	}
	current, err := gitLines(ctx, snapshot.worktree, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return fmt.Errorf("list new untracked files: %w", err)
	}
	for _, path := range current {
		file, err := safeSnapshotPath(snapshot.worktree, path)
		if err != nil {
			return err
		}
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove untracked file %q: %w", path, err)
		}
	}
	if err := applySnapshotPatch(ctx, snapshot.worktree, snapshot.stagedPatch, true); err != nil {
		return fmt.Errorf("restore staged changes: %w", err)
	}
	if err := applySnapshotPatch(ctx, snapshot.worktree, snapshot.unstagedPatch, false); err != nil {
		return fmt.Errorf("restore unstaged changes: %w", err)
	}
	for _, path := range snapshot.untrackedPaths {
		from, err := safeSnapshotPath(snapshot.directory, path)
		if err != nil {
			return err
		}
		to, err := safeSnapshotPath(snapshot.worktree, path)
		if err != nil {
			return err
		}
		if err := copySnapshotFile(from, to); err != nil {
			return fmt.Errorf("restore untracked file %q: %w", path, err)
		}
	}
	return nil
}

func applySnapshotPatch(ctx context.Context, worktree string, patch []byte, index bool) error {
	if len(bytes.TrimSpace(patch)) == 0 {
		return nil
	}
	args := []string{"apply"}
	if index {
		args = append(args, "--index")
	}
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = worktree
	command.Stdin = bytes.NewReader(patch)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func safeSnapshotPath(root, path string) (string, error) {
	path = filepath.Clean(path)
	if path == "." || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe snapshot path %q", path)
	}
	return filepath.Join(root, path), nil
}

func copySnapshotFile(from, to string) error {
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
		return err
	}
	input, err := os.Open(from)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func command(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}
