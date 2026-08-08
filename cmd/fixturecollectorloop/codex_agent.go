package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// commandRunner is deliberately small so collector tests never need Codex, GitHub, or a network.
type commandRunner interface {
	Run(dir, name string, args []string) ([]byte, error)
}

type systemCommandRunner struct{}

func (systemCommandRunner) Run(dir, name string, args []string) ([]byte, error) {
	command := exec.Command(name, args...)
	command.Dir = dir
	return command.CombinedOutput()
}

// codexAgent starts one fresh non-interactive session for every collection iteration.
// The collector, rather than the session, owns scheduling, validation, logging, and Git.
type codexAgent struct {
	root       string
	stdout     io.Writer
	runner     commandRunner
	retainLogs bool
}

func newCodexAgent(root string, stdout io.Writer) *codexAgent {
	return &codexAgent{root: root, stdout: stdout, runner: systemCommandRunner{}}
}

func (a *codexAgent) SetRetainLogs(retain bool) { a.retainLogs = retain }

func (a *codexAgent) Preflight() error {
	if output, err := a.runner.Run(a.root, "codex", []string{"login", "status"}); err != nil {
		return commandError("Codex authentication", output, err)
	}
	if output, err := a.runner.Run(a.root, "gh", []string{"auth", "status", "--hostname", "github.com"}); err != nil {
		return commandError("GitHub authentication", output, err)
	}
	return nil
}

func commandError(name string, output []byte, err error) error {
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("%s: %w", name, err)
	}
	return fmt.Errorf("%s: %w: %s", name, err, detail)
}

func (a *codexAgent) Run(iteration Iteration) (Outcome, error) {
	logDir := filepath.Join(a.root, ".deplens", "fixture-collection-logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return Outcome{}, err
	}
	logPath := filepath.Join(logDir, fmt.Sprintf("%s-%d.jsonl", iteration.DetectorID, iteration.Iteration))
	result, err := os.CreateTemp(logDir, ".outcome-*.json")
	if err != nil {
		return Outcome{}, err
	}
	resultPath := result.Name()
	if err := result.Close(); err != nil {
		return Outcome{}, err
	}
	defer os.Remove(resultPath)
	schema, err := os.CreateTemp(logDir, ".outcome-schema-*.json")
	if err != nil {
		return Outcome{}, err
	}
	schemaPath := schema.Name()
	if _, err := schema.WriteString(outcomeSchema); err != nil {
		schema.Close()
		os.Remove(schemaPath)
		return Outcome{}, err
	}
	if err := schema.Close(); err != nil {
		os.Remove(schemaPath)
		return Outcome{}, err
	}
	defer os.Remove(schemaPath)

	fmt.Fprintf(a.stdout, "researching detector %s (iteration %d); detailed JSONL is retained locally on failure\n", iteration.DetectorID, iteration.Iteration)
	args := []string{"exec", "--json", "--sandbox", "workspace-write", "-C", a.root, "--output-schema", schemaPath, "--output-last-message", resultPath, collectionPrompt(iteration)}
	jsonl, runErr := a.runner.Run(a.root, "codex", args)
	if err := writePrivateFile(logPath, jsonl); err != nil {
		return Outcome{}, err
	}
	if runErr != nil {
		return Outcome{}, fmt.Errorf("Codex session failed; log retained at %s (it may contain sensitive content): %w", logPath, runErr)
	}
	contents, err := os.ReadFile(resultPath)
	if err != nil {
		return Outcome{}, fmt.Errorf("Codex session produced no final outcome; log retained at %s: %w", logPath, err)
	}
	var outcome Outcome
	if err := json.Unmarshal(contents, &outcome); err != nil {
		return Outcome{}, fmt.Errorf("Codex final outcome is invalid; log retained at %s: %w", logPath, err)
	}
	if !a.retainLogs {
		_ = os.Remove(logPath)
	} else {
		fmt.Fprintf(a.stdout, "research complete; JSONL log retained at %s (it may contain sensitive content)\n", logPath)
	}
	return outcome, nil
}

func writePrivateFile(path string, contents []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(contents); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

const outcomeSchema = `{"type":"object","additionalProperties":false,"required":["Result","Added","Queries","Candidates","Rejections"],"properties":{"Result":{"enum":["accepted","unsuccessful"]},"Added":{"type":"array","items":{"type":"string"}},"Queries":{"type":"array","items":{"type":"string"}},"Candidates":{"type":"array","items":{"type":"string"}},"Rejections":{"type":"array","items":{"type":"string"}}}}`

func collectionPrompt(i Iteration) string {
	return fmt.Sprintf(`Collect one authentic dependency-source corpus example for detector %q, iteration %d.
Missing profile dimensions: %s.
Prior history (queries, candidates, and rejections): %s.
Work only under %q. You have normal workspace-write permissions, but must not run Git commands or change files outside that directory. Do not commit, reset, restore, stash, clean, push, pull, create branches, or modify collection progress.
Use GitHub only for read-only discovery and retrieval through gh/API. Before accepting an example, resolve its repository default branch to a commit SHA using GitHub APIs, retrieve the source at that immutable SHA, and write provenance pinned to that SHA. Do at most %d search queries and inspect at most %d candidates. Never print candidate contents in the final response.
Return exactly the JSON object required by the output schema. Added paths are relative to the corpus directory.`, i.DetectorID, i.Iteration, strings.Join(i.MissingDimensions, "; "), strings.Join(i.PriorHistory, "; "), i.CorpusDir, i.QueryLimit, i.CandidateLimit)
}
