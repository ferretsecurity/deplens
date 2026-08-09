package main

import (
	"bytes"
	"context"
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
	Run(context.Context, string, string, []string) ([]byte, error)
}

type streamingCommandRunner interface {
	RunStreaming(context.Context, string, string, []string, io.Writer) ([]byte, error)
}

type systemCommandRunner struct{}

func (systemCommandRunner) Run(ctx context.Context, dir, name string, args []string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	return command.CombinedOutput()
}

func (systemCommandRunner) RunStreaming(ctx context.Context, dir, name string, args []string, output io.Writer) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	var captured bytes.Buffer
	command.Stdout = io.MultiWriter(&captured, output)
	command.Stderr = io.MultiWriter(&captured, output)
	err := command.Run()
	return captured.Bytes(), err
}

// codexAgent starts one fresh non-interactive session for every collection iteration.
// The collector, rather than the session, owns scheduling, validation, logging, and Git.
type codexAgent struct {
	root       string
	stdout     io.Writer
	runner     commandRunner
	retainLogs bool
	logPath    string
}

func newCodexAgent(root string, stdout io.Writer) *codexAgent {
	return &codexAgent{root: root, stdout: stdout, runner: systemCommandRunner{}, retainLogs: true}
}

func (a *codexAgent) SetRetainLogs(retain bool) { a.retainLogs = retain }

func (a *codexAgent) Preflight() error {
	if output, err := a.runner.Run(context.Background(), a.root, "codex", []string{"login", "status"}); err != nil {
		return commandError("Codex authentication", output, err)
	}
	if output, err := a.runner.Run(context.Background(), a.root, "gh", []string{"auth", "status", "--hostname", "github.com"}); err != nil {
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

func (a *codexAgent) Run(ctx context.Context, iteration Iteration) (Outcome, error) {
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

	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return Outcome{}, err
	}
	if err := log.Chmod(0o600); err != nil {
		log.Close()
		return Outcome{}, err
	}
	fmt.Fprintf(a.stdout, "researching detector %s (iteration %d); monitor JSONL with: tail -f %s\n", iteration.DetectorID, iteration.Iteration, logPath)
	args := []string{"exec", "--json", "--sandbox", "workspace-write", "-C", a.root, "--output-schema", schemaPath, "--output-last-message", resultPath, collectionPrompt(iteration)}
	var jsonl []byte
	var runErr error
	if streaming, ok := a.runner.(streamingCommandRunner); ok {
		jsonl, runErr = streaming.RunStreaming(ctx, a.root, "codex", args, log)
	} else {
		jsonl, runErr = a.runner.Run(ctx, a.root, "codex", args)
		if _, err := log.Write(jsonl); err != nil {
			log.Close()
			return Outcome{}, err
		}
	}
	if err := log.Close(); err != nil {
		return Outcome{}, err
	}
	a.logPath = logPath
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
	return outcome, nil
}

// FinalizeIteration retains successful logs by default after the wrapper
// validates and checkpoints the corpus change. Failed validation always
// preserves its log; --retain-logs=false discards successful logs.
func (a *codexAgent) FinalizeIteration(success bool) {
	if a.logPath == "" {
		return
	}
	if success && !a.retainLogs {
		_ = os.Remove(a.logPath)
	} else if success {
		fmt.Fprintf(a.stdout, "research complete; JSONL log retained at %s (it may contain sensitive content)\n", a.logPath)
	}
	a.logPath = ""
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
For every accepted example, create exactly one YAML file named provenance.yaml at the example root: <corpus-dir>/<example-id>/provenance.yaml. Preserve the upstream-relative source path below that same root. provenance.yaml must contain these fields: version: 1, detector_id, provider, repository, repository_url, commit, original_path, permalink, retrieved_at (RFC 3339), sha256, license (approved SPDX ID), license_url, project_kind (application, library, tooling, unknown, or not applicable), variation_tags (non-empty list), and rationale (non-empty). Do not use provenance.json.
Return exactly the JSON object required by the output schema. Added paths are relative to the corpus directory.`, i.DetectorID, i.Iteration, strings.Join(i.MissingDimensions, "; "), strings.Join(i.PriorHistory, "; "), i.CorpusDir, i.QueryLimit, i.CandidateLimit)
}
