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
	"sync"
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
		if errors.Is(err, flag.ErrHelp) {
			printFlagUsage(stdout, "analyzerloop plan", fs)
			return 0
		}
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
	follow := fs.Bool("follow", false, "show live progress and bounded agent messages")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printFlagUsage(stdout, "analyzerloop run", fs)
			return 0
		}
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
	renderer := newProgressRenderer(stdout, *follow, runtimeRoot)
	renderer.Configuration(numbers, *ledgerPath, *noCommit)
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
			Engine:         analyzerloop.CodexEngine{Progress: renderer},
		}
	} else {
		executor = analyzerloop.GitWorktreeExecutor{
			RepositoryRoot: workingDir,
			CorpusRoot:     ledger.Corpus.Path,
			RuntimeRoot:    runtimeRoot,
			Engine:         analyzerloop.CodexEngine{Progress: renderer},
		}
	}
	runner := analyzerloop.Runner{
		LedgerPath: *ledgerPath,
		Executor:   executor,
		Journal:    journal,
		Progress:   renderer,
	}
	if !*noCommit {
		runner.Committer = analyzerloop.GitCommitter{
			RepositoryRoot: workingDir,
			LedgerPath:     *ledgerPath,
			RunID:          runID,
		}
	}
	started := time.Now()
	if err := runner.Run(context.Background(), numbers); err != nil {
		renderer.RunFinished(false, completedSelected(*ledgerPath, numbers), unfinishedSelected(*ledgerPath, numbers), time.Since(started))
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	completed := completedSelected(*ledgerPath, numbers)
	unfinished := len(numbers) - completed
	renderer.RunFinished(unfinished == 0, completed, unfinished, time.Since(started))
	return 0
}

func completedSelected(path string, selection []int) int {
	ledger, err := analyzerloop.LoadLedger(path)
	if err != nil {
		return 0
	}
	completed := 0
	for _, number := range selection {
		if number <= len(ledger.WorkItems) && ledger.WorkItems[number-1].State == analyzerloop.StateCompleted {
			completed++
		}
	}
	return completed
}

func unfinishedSelected(path string, selection []int) int {
	return len(selection) - completedSelected(path, selection)
}

func runSelection(value string, once bool, ledger analyzerloop.Ledger) ([]int, error) {
	if value == "" {
		for _, item := range ledger.WorkItems {
			if item.State != analyzerloop.StateCompleted && item.State != analyzerloop.StateIgnored {
				return []int{item.Number}, nil
			}
		}
		return nil, errors.New("all work items are completed or ignored")
	}
	numbers, err := analyzerloop.ParseSelection(value, len(ledger.WorkItems))
	if err != nil {
		return nil, err
	}
	runnable := numbers[:0]
	for _, number := range numbers {
		state := ledger.WorkItems[number-1].State
		if state != analyzerloop.StateCompleted && state != analyzerloop.StateIgnored {
			runnable = append(runnable, number)
		}
	}
	if len(runnable) == 0 {
		return nil, errors.New("selected work items are completed or ignored")
	}
	if once {
		return runnable[:1], nil
	}
	return runnable, nil
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

func printFlagUsage(output io.Writer, command string, fs *flag.FlagSet) {
	fmt.Fprintf(output, "Usage: %s [flags]\n", command)
	fs.SetOutput(output)
	fs.PrintDefaults()
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

// progressRenderer writes the deliberately bounded stdout narrative. Raw
// engine output remains private runtime diagnostics and is never mirrored.
type progressRenderer struct {
	output  io.Writer
	follow  bool
	runtime string
	mu      sync.Mutex
}

func newProgressRenderer(output io.Writer, follow bool, runtime string) *progressRenderer {
	return &progressRenderer{output: output, follow: follow, runtime: runtime}
}

func (p *progressRenderer) Configuration(selection []int, ledger string, noCommit bool) {
	mode := "commits enabled"
	if noCommit {
		mode = "trial run · accepted changes left uncommitted"
	}
	follow := "Disabled"
	if p.follow {
		follow = "Enabled"
	}
	p.printf("Configuration\n  Selection     %s\n  Mode          %s\n  Ledger        %s\n  Runtime       %s\n  Policy        Independent set\n  Limits        %d Work Item(s) · 2 Attempts per role\n  Follow Mode   %s\n\n🚀 Analyzer loop\n", formatSelection(selection), mode, ledger, p.runtime, len(selection), follow)
}

func (p *progressRenderer) WorkItemStarted(item analyzerloop.WorkItem) {
	p.printf("\n📦 Work Item %d — Implement detector %q\n", item.Number, item.ID)
}

func (p *progressRenderer) AttemptStarted(attempt analyzerloop.Attempt) {
	p.printf("🔄 %s attempt %d of %d\n\n  ↓ Run %s agent\n", title(string(attempt.Role)), attempt.Number, attempt.Limit, attempt.Role)
}

func (p *progressRenderer) AgentStarted(_ analyzerloop.Attempt, rawPath, _ string) {
	if !p.follow {
		return
	}
	p.printf("      Raw output      %s\n      Follow          tail -f %q\n      Sensitive data  Raw logs and displayed commands may contain sensitive data\n", rawPath, rawPath)
}

func (p *progressRenderer) AgentMessage(_ analyzerloop.Attempt, message string) {
	if !p.follow || message == "" {
		return
	}
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 360 {
		message = message[:357] + "..."
	}
	p.printf("      💭 %s\n", message)
}

func (p *progressRenderer) AgentCommand(_ analyzerloop.Attempt, workdir, command string, exitCode int) {
	if !p.follow {
		return
	}
	result := "✓ Agent command completed"
	if exitCode != 0 {
		result = fmt.Sprintf("⚠ Agent command exited %d", exitCode)
	}
	p.printf("      ╭─ agent · %s\n      ╰─$ %s\n      %s\n", workdir, command, result)
}

func (p *progressRenderer) AgentEdited(_ analyzerloop.Attempt, paths []string) {
	if !p.follow || len(paths) == 0 {
		return
	}
	p.printf("      📝 Agent edited\n")
	for _, path := range paths {
		p.printf("         %s\n", path)
	}
}

func (p *progressRenderer) AgentHeartbeat(_ analyzerloop.Attempt, elapsed time.Duration) {
	if p.follow {
		p.printf("      … Agent is still running · %s elapsed\n", duration(elapsed))
	}
}

func (p *progressRenderer) AgentFinished(_ analyzerloop.Attempt, elapsed time.Duration) {
	p.printf("  ✓ Agent finished · %s\n", duration(elapsed))
}

func (p *progressRenderer) ValidationStarted(_ analyzerloop.Attempt) {
	p.printf("\n  ↓ Validate analyzer implementation\n")
}

func (p *progressRenderer) ValidationAccepted(_ analyzerloop.Attempt, paths []string) {
	if p.follow && len(paths) > 0 {
		p.printf("      📄 Harness accepted\n")
		for _, path := range paths {
			p.printf("         %s\n", path)
		}
	}
	p.printf("  ✓ Checkpoint validation passed · %d changed file(s) accepted\n", len(paths))
}

func (p *progressRenderer) FixtureOutputStarted(_ analyzerloop.Attempt, directory string) {
	p.printf("\n  ↓ Capture Deplens fixture output\n")
	if p.follow {
		p.printf("      Output         %s\n", directory)
	}
}

func (p *progressRenderer) FixtureOutputSaved(_ analyzerloop.Attempt, fixture, humanPath, jsonPath string) {
	if !p.follow {
		return
	}
	p.printf("      %s\n        Human      %s\n        JSON       %s\n", fixture, humanPath, jsonPath)
}

func (p *progressRenderer) FixtureOutputFinished(_ analyzerloop.Attempt, fixtures int) {
	p.printf("  ✓ Captured human-readable and JSON CLI output · %d fixture(s)\n", fixtures)
}

func (p *progressRenderer) AttemptFinished(attempt analyzerloop.Attempt, result analyzerloop.AttemptResult, err error, elapsed time.Duration) {
	if err != nil {
		p.printf("  ❌ %s attempt %d of %d failed · %s\n     Reason       %s\n     Diagnostics  %s\n     Inspect      tail -n 80 %q\n", title(string(attempt.Role)), attempt.Number, attempt.Limit, duration(elapsed), err, p.runtime, filepath.Join(p.runtime, "journal.jsonl"))
		return
	}
	p.printf("  ✅ %s attempt %d of %d accepted · %s\n", title(string(attempt.Role)), attempt.Number, attempt.Limit, duration(elapsed))
	_ = result
}

func (p *progressRenderer) WorkItemFinished(item analyzerloop.WorkItem, completed bool) {
	state := "unfinished"
	result := "No accepted verifier checkpoint"
	next := fmt.Sprintf("Run analyzerloop again for Work Item %d", item.Number)
	if completed {
		state = "completed"
		result = "Verifier accepted the analyzer implementation"
		next = "No action needed"
	}
	p.printf("\n────────────────────────────────────────────────────────\n%s Work Item %d %s\n\n  Result        %s\n  Deliverable   Analyzer changes and three synthetic fixtures\n  State         %s\n  Next          %s\n  Diagnostics   %s\n────────────────────────────────────────────────────────\n", resultMark(completed), item.Number, state, result, title(item.State), next, p.runtime)
}

func (p *progressRenderer) RunFinished(success bool, completed, unfinished int, elapsed time.Duration) {
	result := "finished — Work Items remain unfinished"
	if success {
		result = "completed"
	}
	p.printf("\n════════════════════════════════════════════════════════\n%s Analyzer loop %s · %s\n\n  Work Items    %d completed · %d unfinished\n  Runtime       %s\n  Next          Inspect the worktree and runtime diagnostics before the next run\n════════════════════════════════════════════════════════\n", resultMark(success), result, duration(elapsed), completed, unfinished, p.runtime)
}

func (p *progressRenderer) printf(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.output, format, args...)
}

func duration(value time.Duration) string {
	if value < time.Second {
		return "<1s"
	}
	return value.Round(time.Second).String()
}

func title(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func resultMark(success bool) string {
	if success {
		return "✅"
	}
	return "❌"
}

func formatSelection(numbers []int) string {
	if len(numbers) == 1 {
		return fmt.Sprintf("Work Item %d", numbers[0])
	}
	parts := make([]string, len(numbers))
	for index, number := range numbers {
		parts[index] = fmt.Sprint(number)
	}
	return "Work Items " + strings.Join(parts, ",")
}
