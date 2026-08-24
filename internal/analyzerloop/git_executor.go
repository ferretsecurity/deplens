package analyzerloop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		"exec", "--json", "--model", "gpt-5.6-terra", "--sandbox", "workspace-write", "--ask-for-approval", "never",
		"--config", "model_reasoning_effort=high", "--config", "mcp_servers={}", prompt(e.CorpusRoot, attempt))
	command.Dir = e.Workdir
	output, err := command.CombinedOutput()
	if e.OutputPath != "" {
		if writeErr := os.WriteFile(e.OutputPath, output, 0o600); writeErr != nil {
			return AttemptResult{}, fmt.Errorf("write Codex output: %w", writeErr)
		}
	}
	if err != nil {
		if ctx.Err() != nil {
			return AttemptResult{}, ctx.Err()
		}
		return AttemptResult{}, fmt.Errorf("run isolated Codex attempt: %w", err)
	}
	result, err := parseEngineResult(output)
	if err != nil {
		return AttemptResult{}, err
	}
	return result, nil
}

func prompt(corpusRoot string, attempt Attempt) string {
	return fmt.Sprintf(`You are the %s for dependency detector %q (work item %d).
Work only in the current detached worktree. You may read originals from %q.
Implement or verify the detector according to repository conventions. Inspect all three candidates, create exactly three minimized synthetic fixtures under testdata, and never copy originals or provenance. Do not change the loop harness, ledger, corpus repository, Go modules, or unrelated files. Run focused tests as needed.
At the end, output exactly one line and nothing after it:
<analyzerloop-result>{"summary":"short description","fixtures":["testdata/...","testdata/...","testdata/..."]}</analyzerloop-result>`, attempt.Role, attempt.WorkItem.ID, attempt.WorkItem.Number, corpusRoot)
}

func parseEngineResult(output []byte) (AttemptResult, error) {
	const open, close = "<analyzerloop-result>", "</analyzerloop-result>"
	text := string(output)
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

func (e DirectExecutor) Execute(ctx context.Context, attempt Attempt) (AttemptResult, error) {
	outputPath := filepath.Join(e.RuntimeRoot, "attempts", fmt.Sprintf("%03d-%s-%d.jsonl", attempt.WorkItem.Number, attempt.Role, attempt.Number))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return AttemptResult{}, fmt.Errorf("create attempt output directory: %w", err)
	}
	engine := e.Engine
	engine.Workdir, engine.CorpusRoot, engine.OutputPath = e.RepositoryRoot, e.CorpusRoot, outputPath
	result, err := engine.Execute(ctx, attempt)
	if err != nil {
		return AttemptResult{}, err
	}
	if err := validateWorktree(ctx, e.RepositoryRoot, e.CorpusRoot, attempt); err != nil {
		return AttemptResult{}, err
	}
	paths, err := changedPaths(ctx, e.RepositoryRoot)
	if err != nil {
		return AttemptResult{}, fmt.Errorf("list direct attempt changes: %w", err)
	}
	if err := allowedChangedPaths(paths); err != nil {
		return AttemptResult{}, err
	}
	result.ChangedPaths = paths
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
	patch, err := git(ctx, worktree, "diff", "--binary")
	if err != nil {
		return AttemptResult{}, fmt.Errorf("create validated attempt patch: %w", err)
	}
	if len(bytes.TrimSpace(patch)) == 0 {
		return AttemptResult{}, errors.New("attempt produced no changes")
	}
	apply := exec.CommandContext(ctx, "git", "apply", "--whitespace=error")
	apply.Dir, apply.Stdin = e.RepositoryRoot, bytes.NewReader(patch)
	if output, err := apply.CombinedOutput(); err != nil {
		return AttemptResult{}, fmt.Errorf("apply validated attempt patch: %w: %s", err, strings.TrimSpace(string(output)))
	}
	result.ChangedPaths = paths
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
		return fmt.Errorf("run focused analyzer tests: %w: %s", err, strings.TrimSpace(string(output)))
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
		output, err := command(ctx, worktree, "go", "run", "./cmd/deplens", "--json", path)
		if err != nil || !recognizedCandidate(output, attempt.WorkItem.ID) {
			return fmt.Errorf("original candidate %q was not recognized by detector %q", candidate.ID, attempt.WorkItem.ID)
		}
	}
	return nil
}

func recognizedCandidate(output []byte, detector string) bool {
	var scan struct {
		Sources []struct {
			Detector string `json:"detector"`
			Analysis struct {
				Presence   string `json:"presence"`
				Extraction string `json:"extraction"`
			} `json:"analysis"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(output, &scan); err != nil {
		return false
	}
	for _, source := range scan.Sources {
		if source.Detector == detector && source.Analysis.Presence != "unknown" && source.Analysis.Extraction != "unsupported" && source.Analysis.Extraction != "failed" {
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
	untracked, err := gitLines(ctx, dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	return append(paths, untracked...), nil
}

func command(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}
