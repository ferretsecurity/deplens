// analyzerloop plans and runs durable semantic-analyzer implementation work.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ferretsecurity/deplens/internal/analyzerloop"
	"gopkg.in/yaml.v3"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage())
		return 1
	}
	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "error: get working directory: %v\n", err)
		return 1
	}
	switch args[0] {
	case "plan":
		return plan(args[1:], workingDir, stdout, stderr)
	case "run":
		return execute(args[1:], workingDir, stdout, stderr)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage())
		return 0
	default:
		fmt.Fprintf(stderr, "error: unknown subcommand %q\n\n%s", args[0], usage())
		return 1
	}
}

func plan(args []string, workingDir string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("analyzerloop plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	corpus := fs.String("corpus", filepath.Join(workingDir, "..", "deplens-fixture-corpus"), "path to deplens-fixture-corpus")
	ledger := fs.String("ledger", filepath.Join(workingDir, ".deplens", "analyzer-implementation.yaml"), "ledger output path")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: plan accepts no positional arguments")
		return 1
	}
	commit, err := gitValue(workingDir, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	verificationPath := filepath.Join(*corpus, ".deplens", "corpus-verification.yaml")
	baseCommit, err := verificationDeplensCommit(verificationPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := descendantOf(workingDir, baseCommit, commit); err != nil {
		fmt.Fprintf(stderr, "error: current commit does not descend from the corpus base: %v\n", err)
		return 1
	}
	corpusCommit, err := gitValue(*corpus, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	created, err := analyzerloop.Plan(analyzerloop.PlanOptions{
		CorpusRoot:       *corpus,
		VerificationPath: verificationPath,
		LedgerPath:       *ledger,
		DeplensCommit:    baseCommit,
		CorpusCommit:     corpusCommit,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if len(created.WorkItems) == 0 {
		fmt.Fprintln(stdout, "No eligible analyzer work items; no ledger was written.")
		return 0
	}
	fmt.Fprintf(stdout, "Created %s with %d eligible analyzer work items. Review and commit it before running.\n", *ledger, len(created.WorkItems))
	return 0
}

func execute(args []string, workingDir string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("analyzerloop run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	corpus := fs.String("corpus", filepath.Join(workingDir, "..", "deplens-fixture-corpus"), "path to deplens-fixture-corpus")
	ledgerPath := fs.String("ledger", filepath.Join(workingDir, ".deplens", "analyzer-implementation.yaml"), "approved ledger path")
	selection := fs.String("select", "", "work-item selection, for example 1,3...7,12")
	once := fs.Bool("once", false, "run only the first selected unfinished work item")
	noCommit := fs.Bool("no-commit", false, "leave accepted changes uncommitted")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: run accepts no positional arguments")
		return 1
	}
	ledger, err := analyzerloop.LoadLedger(*ledgerPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	commit, err := gitValue(workingDir, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintf(stderr, "error: read current commit: %v\n", err)
		return 1
	}
	if err := descendantOf(workingDir, ledger.Deplens.Commit, commit); err != nil {
		fmt.Fprintf(stderr, "error: current commit does not descend from the approved ledger: %v\n", err)
		return 1
	}
	corpusCommit, err := gitValue(*corpus, "rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintf(stderr, "error: read corpus commit: %v\n", err)
		return 1
	}
	if corpusCommit != ledger.Corpus.Commit {
		fmt.Fprintln(stderr, "error: corpus commit does not match the approved ledger")
		return 1
	}
	if err := analyzerloop.PreflightRun(context.Background(), workingDir, *corpus); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	numbers, err := runSelection(*selection, *once, ledger)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	runID := time.Now().UTC().Format("20060102T150405Z")
	runtimeRoot := filepath.Join(workingDir, ".ralph", "runs", runID)
	journal, err := analyzerloop.NewFileJournal(filepath.Join(runtimeRoot, "journal.jsonl"))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	var executor analyzerloop.Executor
	if *noCommit {
		executor = analyzerloop.DirectExecutor{
			RepositoryRoot: workingDir,
			CorpusRoot:     ledger.Corpus.Path,
			RuntimeRoot:    runtimeRoot,
			Engine:         analyzerloop.CodexEngine{},
		}
	} else {
		executor = analyzerloop.GitWorktreeExecutor{
			RepositoryRoot: workingDir,
			CorpusRoot:     ledger.Corpus.Path,
			RuntimeRoot:    runtimeRoot,
			Engine:         analyzerloop.CodexEngine{},
		}
	}
	runner := analyzerloop.Runner{
		LedgerPath: *ledgerPath,
		Executor:   executor,
		Journal:    journal,
	}
	if !*noCommit {
		runner.Committer = analyzerloop.GitCommitter{
			RepositoryRoot: workingDir,
			LedgerPath:     *ledgerPath,
			RunID:          runID,
		}
	}
	if err := runner.Run(context.Background(), numbers); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Analyzer loop finished. Runtime journal: %s\n", runtimeRoot)
	return 0
}

func runSelection(value string, once bool, ledger analyzerloop.Ledger) ([]int, error) {
	if value == "" {
		for _, item := range ledger.WorkItems {
			if item.State != analyzerloop.StateCompleted {
				return []int{item.Number}, nil
			}
		}
		return nil, errors.New("all work items are completed")
	}
	numbers, err := analyzerloop.ParseSelection(value, len(ledger.WorkItems))
	if err != nil {
		return nil, err
	}
	if once {
		for _, number := range numbers {
			if ledger.WorkItems[number-1].State != analyzerloop.StateCompleted {
				return []int{number}, nil
			}
		}
		return nil, errors.New("selected work items are completed")
	}
	return numbers, nil
}

func verificationDeplensCommit(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read corpus verification ledger: %w", err)
	}
	var verification struct {
		Deplens struct {
			Commit string `yaml:"commit"`
		} `yaml:"deplens"`
	}
	if err := yaml.Unmarshal(data, &verification); err != nil {
		return "", fmt.Errorf("parse corpus verification ledger: %w", err)
	}
	if verification.Deplens.Commit == "" {
		return "", errors.New("corpus verification ledger has no deplens commit")
	}
	return verification.Deplens.Commit, nil
}

func gitValue(dir string, args ...string) (string, error) {
	output, err := command(context.Background(), dir, "git", args...)
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func descendantOf(dir, ancestor, descendant string) error {
	if ancestor == "" {
		return errors.New("approved ledger has no base commit")
	}
	output, err := command(context.Background(), dir, "git", "merge-base", "--is-ancestor", ancestor, descendant)
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return fmt.Errorf("%s is not an ancestor of %s", ancestor, descendant)
		}
		return fmt.Errorf("run git merge-base --is-ancestor: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func command(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func usage() string {
	return `Usage: analyzerloop <plan|run> [flags]

plan creates an approved analyzer work ledger from the verified fixture corpus.
run executes selected work items through isolated implementer and verifier attempts.
`
}
