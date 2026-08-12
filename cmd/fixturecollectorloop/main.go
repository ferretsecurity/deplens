// fixturecollectorloop collects authentic dependency-source corpus examples.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ferretsecurity/deplens/internal/analyze"
	"gopkg.in/yaml.v3"
)

const (
	statePending               = "pending"
	stateInProgress            = "in-progress"
	stateComplete              = "complete"
	stateNeedsQueryReview      = "needs-query-review"
	stateNeedsContentReview    = "needs-content-review"
	stateNeedsCollectionReview = "needs-collection-review"
	maxIterations              = 7
	defaultTarget              = 3
	progressVersion            = 2
)

// CollectionLimits are reviewed acquisition controls. CandidateInspections is
// the normal inspection target; qualification may exceed it only until five
// candidates qualify. The other resource fields remain hard limits.
type CollectionLimits struct {
	Queries              int `yaml:"queries"`
	ResultPages          int `yaml:"result_pages"`
	CandidateInspections int `yaml:"candidate_inspections"`
	DecodedResponseBytes int `yaml:"decoded_response_bytes"`
	PacketTokens         int `yaml:"packet_tokens"`
	SelectorInvocations  int `yaml:"selector_invocations"`
	SourceBytes          int `yaml:"source_bytes"`
	ValidIterations      int `yaml:"valid_iterations"`
}

var defaultCollectionLimits = CollectionLimits{
	Queries: 8, ResultPages: 10, CandidateInspections: 100,
	DecodedResponseBytes: 16 << 20, PacketTokens: 100000,
	SelectorInvocations: 2, SourceBytes: 2 << 20, ValidIterations: maxIterations,
}

type Progress struct {
	Version              int                `yaml:"version"`
	InventoryFingerprint string             `yaml:"inventory_fingerprint,omitempty"`
	Limits               CollectionLimits   `yaml:"limits"`
	Detectors            []DetectorProgress `yaml:"detectors"`
	Recovery             *Recovery          `yaml:"recovery,omitempty"`
}

type Recovery struct {
	RunID           string   `yaml:"run_id"`
	DetectorID      string   `yaml:"detector_id"`
	Iteration       int      `yaml:"iteration"`
	LastCheckpoint  string   `yaml:"last_checkpoint"`
	ProgressPath    string   `yaml:"progress_path"`
	ValidationError string   `yaml:"validation_error"`
	ChangedPaths    []string `yaml:"changed_paths"`
	Commit          bool     `yaml:"commit"`
	AllowDirty      bool     `yaml:"allow_dirty"`
}

type DetectorProgress struct {
	ID                 string            `yaml:"id"`
	Form               string            `yaml:"form,omitempty"`
	Roles              []string          `yaml:"roles,omitempty"`
	State              string            `yaml:"state"`
	Iterations         int               `yaml:"iterations"`
	Examples           []string          `yaml:"examples"`
	Target             int               `yaml:"target,omitempty"`
	QueryPlan          []string          `yaml:"query_plan,omitempty"`
	QueryReviewReason  string            `yaml:"query_review_reason,omitempty"`
	Queries            []string          `yaml:"queries,omitempty"`
	Candidates         []string          `yaml:"candidates,omitempty"`
	FilteredSearchHits map[string]int    `yaml:"filtered_search_hits,omitempty"`
	Rejections         []string          `yaml:"rejections,omitempty"`
	Omitted            []string          `yaml:"omitted,omitempty"`
	DecisionStates     []DecisionState   `yaml:"decision_states,omitempty"`
	History            []IterationRecord `yaml:"history,omitempty"`
}

// IterationRecord is append-only, content-free evidence of a valid attempt.
type IterationRecord struct {
	Iteration          int            `yaml:"iteration"`
	Result             string         `yaml:"result"`
	AcceptedIDs        []string       `yaml:"accepted_ids,omitempty"`
	Queries            []string       `yaml:"queries,omitempty"`
	Candidates         []string       `yaml:"candidates,omitempty"`
	FilteredSearchHits map[string]int `yaml:"filtered_search_hits,omitempty"`
	Rejections         []string       `yaml:"rejections,omitempty"`
	Omitted            []string       `yaml:"omitted,omitempty"`
	Decision           *DecisionState `yaml:"decision,omitempty"`
}

// DecisionState is content-free evidence used to avoid repeating an identical
// valid selector comparison state.
type DecisionState struct {
	PacketFingerprint         string `yaml:"packet_fingerprint"`
	AcceptedCorpusFingerprint string `yaml:"accepted_corpus_fingerprint"`
	SelectorConfiguration     string `yaml:"selector_configuration_fingerprint"`
}

type Iteration struct {
	DetectorID                       string
	CorpusDir                        string
	Iteration                        int
	QueryLimit                       int
	CandidateLimit                   int
	QueryPlan                        []string
	PacketTokens                     int
	AcceptedReferences               []AcceptedCorpusReference
	PresentedCandidateIDs            map[string]bool
	SelectorConfigurationFingerprint string
	PriorDecisionStates              []DecisionState
	ReportProgress                   func(ResearchProgress)
}

const (
	progressSearch        = "search"
	progressQualification = "qualification"
	progressSelection     = "selection"
)

// ResearchProgress carries content-free, operator-facing activity counts from
// the Go-owned acquisition and isolated selection pipeline.
type ResearchProgress struct {
	Stage                              string
	Provider, Query                    string
	Budget                             string
	QueryIndex, QueryTotal, Page, Hits int
	Inspected, InspectionLimit         int
	Qualified, Rejected, Filtered      int
	Candidates, Selected               int
	DownloadedBytes, ByteLimit         int64
	RemainingBytes                     int64
	Final                              bool
}

type Outcome struct {
	Result             string
	Added              []string
	Queries            []string
	Candidates         []string
	FilteredSearchHits map[string]int
	Rejections         []string
	Omitted            []string
}

var now = time.Now
var gitCommit = gitOutput
var syncProgressDirectory = syncDirectory
var collectionSignals = func() (<-chan os.Signal, func()) {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	return ch, func() { signal.Stop(ch) }
}

const maxExampleSize = 2 << 20

var approvedLicenses = map[string]bool{
	"MIT": true, "Apache-2.0": true, "BSD-2-Clause": true, "BSD-3-Clause": true,
	"ISC": true, "0BSD": true, "CC0-1.0": true, "Unlicense": true,
}

type Provenance struct {
	Version       int               `yaml:"version"`
	DetectorID    string            `yaml:"detector_id"`
	CandidateID   string            `yaml:"candidate_id"`
	Provider      string            `yaml:"provider"`
	Repository    string            `yaml:"repository"`
	RepositoryURL string            `yaml:"repository_url"`
	DefaultBranch string            `yaml:"default_branch"`
	Commit        string            `yaml:"commit"`
	OriginalPath  string            `yaml:"original_path"`
	Permalink     string            `yaml:"permalink"`
	RetrievedAt   string            `yaml:"retrieved_at"`
	SHA256        string            `yaml:"sha256"`
	License       ProvenanceLicense `yaml:"governing_license"`
	Rationale     string            `yaml:"rationale"`
}

type ProvenanceLicense struct {
	SPDX      string `yaml:"spdx"`
	Path      string `yaml:"path"`
	Permalink string `yaml:"permalink"`
	SHA256    string `yaml:"sha256"`
}

func provenanceV2From(c SourceCandidate, detectorID, rationale string) Provenance {
	return Provenance{
		Version:       2,
		DetectorID:    detectorID,
		CandidateID:   c.ID,
		Provider:      c.Provider,
		Repository:    c.Repository,
		RepositoryURL: c.RepositoryURL,
		DefaultBranch: c.DefaultBranch,
		Commit:        c.Commit,
		OriginalPath:  c.OriginalPath,
		Permalink:     c.RepositoryURL + "/blob/" + c.Commit + "/" + c.OriginalPath,
		RetrievedAt:   c.RetrievedAt,
		SHA256:        c.SourceSHA256,
		License: ProvenanceLicense{
			SPDX:      c.License.SPDX,
			Path:      c.License.Path,
			Permalink: c.License.Permalink,
			SHA256:    c.License.SHA256,
		},
		Rationale: rationale,
	}
}

func main() {
	// The default selector is isolated and cannot modify the collection.
	os.Exit(run(os.Args[1:], ".", os.Stdout, os.Stderr, newComposedResearcher(newDefaultGitHubAcquisition(defaultCollectionLimits), newIsolatedCodexSelector(defaultIsolatedCodexSelectorConfig()))))
}

func run(args []string, root string, stdout, stderr io.Writer, researcher Researcher) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: fixturecollectorloop <initialize-progress|run> [flags]")
		return 1
	}
	switch args[0] {
	case "initialize-progress":
		return initialize(args[1:], root, stdout, stderr)
	case "run":
		return collect(args[1:], root, stdout, stderr, researcher)
	default:
		fmt.Fprintf(stderr, "error: unknown command %q\n", args[0])
		return 1
	}
}

func initialize(args []string, root string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("initialize-progress", flag.ContinueOnError)
	fs.SetOutput(stderr)
	progressPath := fs.String("progress", filepath.Join(root, ".deplens", "fixture-collection.yaml"), "collection progress path")
	target := fs.Int("target", defaultTarget, "examples required per detector (3 through 5)")
	var detectors commaList
	fs.Var(&detectors, "detector", "detector identifier (repeatable or comma-separated)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	printFlagValues(stdout, "initialize-progress", fs)
	if *target < 3 || *target > 5 {
		fmt.Fprintln(stderr, "error: target must be between 3 and 5")
		return 1
	}
	if _, err := os.Stat(*progressPath); err == nil {
		fmt.Fprintf(stderr, "error: collection progress already exists: %s\n", *progressPath)
		return 1
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "error: inspect progress path: %v\n", err)
		return 1
	}
	p := Progress{Version: progressVersion, Limits: defaultCollectionLimits}
	if len(detectors) == 0 {
		rules, err := analyze.LoadDefaultRules()
		if err != nil {
			fmt.Fprintf(stderr, "error: load detector inventory: %v\n", err)
			return 1
		}
		if err := validateCoverageInventory(root, rules); err != nil {
			fmt.Fprintf(stderr, "error: validate coverage inventory: %v\n", err)
			return 1
		}
		p.InventoryFingerprint = rules.DetectorInventoryFingerprint()
		for _, capability := range rules.DetectorCapabilities() {
			if hasCapability(capability.Capabilities, "extract") {
				continue
			}
			roles := make([]string, len(capability.Roles))
			for i, role := range capability.Roles {
				roles[i] = string(role)
			}
			plan := generateQueryPlan(capability)
			state := statePending
			if len(plan.queries) == 0 {
				state = stateNeedsQueryReview
			}
			p.Detectors = append(p.Detectors, DetectorProgress{ID: string(capability.ID), Form: string(capability.Form), Roles: roles, State: state, Target: *target, Examples: []string{}, QueryPlan: plan.queries, QueryReviewReason: plan.reason})
		}
	} else {
		p.Detectors = make([]DetectorProgress, len(detectors))
		seen := map[string]bool{}
		for i, id := range detectors {
			if id == "" || seen[id] {
				fmt.Fprintf(stderr, "error: detector identifiers must be non-empty and unique\n")
				return 1
			}
			seen[id] = true
			p.Detectors[i] = DetectorProgress{ID: id, State: statePending, Target: *target, Examples: []string{}, QueryPlan: []string{"filename:" + id}}
		}
	}
	if err := writeProgress(*progressPath, p); err != nil {
		fmt.Fprintf(stderr, "error: initialize collection progress: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "initialized collection progress: %s (%d detectors)\n", *progressPath, len(p.Detectors))
	return 0
}

func collect(args []string, root string, stdout, stderr io.Writer, researcher Researcher) int {
	stdout = &synchronizedWriter{writer: stdout}
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	progressPath := fs.String("progress", filepath.Join(root, ".deplens", "fixture-collection.yaml"), "collection progress path")
	target := fs.String("detector", "", "run one detector")
	single := fs.Bool("single", false, "run one automatically selected iteration")
	duration := fs.Duration("duration", 8*time.Hour, "soft limit for scheduling new iterations")
	queryLimit := fs.Int("query-limit", 5, "maximum queries recorded by one iteration")
	candidateLimit := fs.Int("candidate-limit", defaultCollectionLimits.CandidateInspections, "normal candidate inspection target; continue past it until five qualify")
	allowDirty := fs.Bool("allow-dirty", false, "allow a checkout that already has non-ignored changes")
	commit := fs.Bool("commit", false, "create one local collection commit for each valid iteration")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	printFlagValues(stdout, "run", fs)
	if *duration < 0 {
		fmt.Fprintln(stderr, "error: duration must not be negative")
		return 1
	}
	if *queryLimit < 1 || *candidateLimit < 1 {
		fmt.Fprintln(stderr, "error: query-limit and candidate-limit must be positive")
		return 1
	}
	unlock, err := lockProgress(*progressPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer unlock()
	p, err := readProgress(*progressPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: read collection progress: %v\n", err)
		return 1
	}
	if p.Recovery != nil {
		if !*allowDirty {
			printRecoveryRequired(*p.Recovery, stderr)
			return 1
		}
		recovery := *p.Recovery
		p.Recovery = nil
		if err := writeProgress(*progressPath, p); err != nil {
			fmt.Fprintf(stderr, "error: clear collection recovery state: %v\n", err)
			return 1
		}
		fmt.Fprintf(stderr, "WARNING: --allow-dirty is resuming after recovery for %s iteration %d; preserving the listed unvalidated changes as pre-existing dirty state.\n", recovery.DetectorID, recovery.Iteration)
	}
	if *target != "" && findDetector(p.Detectors, *target) == nil {
		fmt.Fprintf(stderr, "error: unknown detector %q in collection progress\n", *target)
		return 1
	}
	if p.InventoryFingerprint != "" {
		rules, err := analyze.LoadDefaultRules()
		if err != nil {
			fmt.Fprintf(stderr, "error: load detector inventory: %v\n", err)
			return 1
		}
		if p.InventoryFingerprint != rules.DetectorInventoryFingerprint() {
			fmt.Fprintln(stderr, "error: collection progress detector inventory no longer matches the reviewed plan; reinitialize and review it before running")
			return 1
		}
	}
	if err := preflightGit(root, *progressPath, *allowDirty, append([]string{"run"}, args...), stderr); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if *commit {
		if _, err := gitOutput(root, "var", "GIT_AUTHOR_IDENT"); err != nil {
			fmt.Fprintf(stderr, "error: --commit requires a configured Git author identity: %v\n", err)
			return 1
		}
	}
	if preflight, ok := researcher.(researcherPreflighter); ok {
		if err := preflight.Preflight(); err != nil {
			fmt.Fprintf(stderr, "error: collection authentication preflight: %v\n", err)
			return 1
		}
	}
	// A detector becomes complete at the first three independently accepted
	// examples. This also prevents any selector invocation with three existing
	// corpus examples, even for progress created with an older larger target.
	updatedCompletion := false
	for i := range p.Detectors {
		if len(p.Detectors[i].Examples) >= 3 && p.Detectors[i].State == stateInProgress {
			p.Detectors[i].State = stateComplete
			updatedCompletion = true
		}
	}
	if updatedCompletion {
		if err := writeProgress(*progressPath, p); err != nil {
			fmt.Fprintf(stderr, "error: checkpoint collection progress: %v\n", err)
			return 1
		}
	}
	runStarted := now()
	deadline := runStarted.Add(*duration)
	runProgress := newCollectionRunProgress(runStarted, p.Detectors, *target)
	runProgress.printStarted(stdout, *duration, *candidateLimit, p.Limits.PacketTokens)
	signals, stopSignals := collectionSignals()
	defer stopSignals()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		if !now().Before(deadline) {
			runProgress.printSummary(stdout, p.Detectors, *target, now(), "soft duration reached; latest checkpoint preserved")
			return collectionSummary(p, stdout)
		}
		detector := selectDetector(p.Detectors, *target)
		if detector == nil {
			runProgress.printSummary(stdout, p.Detectors, *target, now(), "no eligible detectors remain")
			return collectionSummary(p, stdout)
		}
		if detector.Target == 0 {
			detector.Target = defaultTarget
		}
		if detector.Iterations >= maxIterations {
			detector.State = stateNeedsCollectionReview
			if err := writeProgress(*progressPath, p); err != nil {
				fmt.Fprintf(stderr, "error: checkpoint collection progress: %v\n", err)
				return 1
			}
			if *single || *target != "" {
				return collectionSummary(p, stdout)
			}
			continue
		}
		examplesBefore := len(detector.Examples)
		detectorStarted := now()
		runProgress.printDetectorStarted(stdout, detector, p.Detectors, *target, detectorStarted, examplesBefore)
		result := make(chan iterationResult, 1)
		progressPrinter := newDetectorProgressPrinter(stdout)
		go func() {
			reportProgress := func(progress ResearchProgress) {
				progressPrinter.Print(progress)
			}
			code, checkpointed := runIteration(ctx, root, *progressPath, p, detector, researcher, *queryLimit, *candidateLimit, *commit, *allowDirty, reportProgress, stderr)
			result <- iterationResult{code: code, checkpointed: checkpointed}
		}()
		iteration, stopAfterIteration := waitForIteration(result, signals, cancel, stdout, stderr)
		code, checkpointed := iteration.code, iteration.checkpointed
		runProgress.iterations++
		if code != 0 {
			runProgress.printDetectorFailed(stdout, detector, detectorStarted, now())
			runProgress.printSummary(stdout, p.Detectors, *target, now(), "detector iteration failed")
			return code
		}
		if checkpointed {
			printIterationCheckpoint(stdout, detector.Iterations)
			printSelectedExamplePaths(stdout, root, detector.ID, detector.Examples[examplesBefore:])
		}
		runProgress.printDetectorFinished(stdout, detector, p.Detectors, *target, detectorStarted, now())
		if stopAfterIteration || *single || *target != "" {
			reason := "requested iteration finished"
			if stopAfterIteration {
				reason = "stop requested after active iteration"
			}
			runProgress.printSummary(stdout, p.Detectors, *target, now(), reason)
			return collectionSummary(p, stdout)
		}
	}
}

type collectionRunProgress struct {
	startedAt  time.Time
	total      int
	iterations int
	attempted  map[string]struct{}
	finished   map[string]struct{}
}

func newCollectionRunProgress(startedAt time.Time, detectors []DetectorProgress, target string) *collectionRunProgress {
	return &collectionRunProgress{
		startedAt: startedAt,
		total:     countEligibleDetectors(detectors, target),
		attempted: make(map[string]struct{}),
		finished:  make(map[string]struct{}),
	}
}

func (p *collectionRunProgress) printStarted(stdout io.Writer, duration time.Duration, candidateTarget, packetTokens int) {
	fmt.Fprintf(stdout, "Run started\n  Detectors: %d\n  Time limit: %s\n  Candidate inspection target: %d\n  Selection packet token limit: %d\n",
		p.total, formatRunDuration(duration), candidateTarget, packetTokens)
}

func (p *collectionRunProgress) printDetectorStarted(stdout io.Writer, detector *DetectorProgress, detectors []DetectorProgress, target string, at time.Time, examples int) {
	p.attempted[detector.ID] = struct{}{}
	fmt.Fprintf(stdout, "\n┌─ Detector: %s\n│  Run position: %d/%d · remaining: %d · run elapsed: %s\n│  Iteration: %d/%d\n│  Examples: %d/%d\n",
		detector.ID, len(p.attempted), p.total, countEligibleDetectors(detectors, target), formatRunDuration(at.Sub(p.startedAt)), detector.Iterations+1, maxIterations, examples, defaultTarget)
}

func (p *collectionRunProgress) printDetectorFinished(stdout io.Writer, detector *DetectorProgress, detectors []DetectorProgress, target string, startedAt, finishedAt time.Time) {
	if !isEligible(*detector) {
		p.finished[detector.ID] = struct{}{}
	}
	result := "Iteration finished"
	switch detector.State {
	case stateComplete:
		result = "Complete"
	case stateNeedsQueryReview, stateNeedsContentReview, stateNeedsCollectionReview:
		result = "Review required"
	}
	fmt.Fprintf(stdout, "│\n└─ %s · state: %s · examples: %d/%d · elapsed: %s\n",
		result, detector.State, len(detector.Examples), defaultTarget, formatRunDuration(finishedAt.Sub(startedAt)))
	p.printProgress(stdout, detectors, target, finishedAt)
}

func (p *collectionRunProgress) printDetectorFailed(stdout io.Writer, detector *DetectorProgress, startedAt, failedAt time.Time) {
	fmt.Fprintf(stdout, "│\n└─ Failed · detector: %s · elapsed: %s\n", detector.ID, formatRunDuration(failedAt.Sub(startedAt)))
}

func (p *collectionRunProgress) printProgress(stdout io.Writer, detectors []DetectorProgress, target string, at time.Time) {
	remaining := countEligibleDetectors(detectors, target)
	average, eta := p.estimates(at, remaining)
	fmt.Fprintf(stdout, "\nRun progress: %d/%d finished · %d remaining · %d attempted · %d iterations · elapsed: %s · average: %s · estimated remaining: %s\n",
		len(p.finished), p.total, remaining, len(p.attempted), p.iterations, formatRunDuration(at.Sub(p.startedAt)), average, eta)
}

func (p *collectionRunProgress) printSummary(stdout io.Writer, detectors []DetectorProgress, target string, at time.Time, reason string) {
	remaining := countEligibleDetectors(detectors, target)
	average, eta := p.estimates(at, remaining)
	fmt.Fprintf(stdout, "\nRun finished\n  Reason: %s\n  Detectors attempted: %d\n  Detectors finished: %d/%d\n  Iterations completed: %d\n  Detectors remaining: %d\n  Elapsed: %s\n  Average per finished detector: %s\n  Estimated remaining time: %s\n",
		reason, len(p.attempted), len(p.finished), p.total, p.iterations, remaining, formatRunDuration(at.Sub(p.startedAt)), average, eta)
}

func (p *collectionRunProgress) estimates(at time.Time, remaining int) (string, string) {
	if len(p.finished) == 0 {
		if remaining == 0 {
			return "n/a", "0s"
		}
		return "n/a", "n/a"
	}
	average := at.Sub(p.startedAt) / time.Duration(len(p.finished))
	return formatRunDuration(average), formatRunDuration(average * time.Duration(remaining))
}

func countEligibleDetectors(detectors []DetectorProgress, target string) int {
	if target != "" {
		detector := findDetector(detectors, target)
		if detector != nil && isEligible(*detector) {
			return 1
		}
		return 0
	}
	count := 0
	for _, detector := range detectors {
		if isEligible(detector) {
			count++
		}
	}
	return count
}

func formatRunDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	return duration.Round(time.Second).String()
}

type synchronizedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *synchronizedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}

type detectorProgressPrinter struct {
	stdout io.Writer
	stage  string
}

func newDetectorProgressPrinter(stdout io.Writer) *detectorProgressPrinter {
	return &detectorProgressPrinter{stdout: stdout}
}

func (p *detectorProgressPrinter) Print(progress ResearchProgress) {
	if progress.Stage != p.stage {
		p.stage = progress.Stage
		title := ""
		switch progress.Stage {
		case progressSearch:
			title = "Candidate search"
		case progressQualification:
			title = "Candidate qualification"
		case progressSelection:
			title = "Candidate selection"
		}
		if title != "" {
			fmt.Fprintf(p.stdout, "│\n│  %s\n", title)
		}
	}
	printResearchProgress(p.stdout, progress)
}

func printResearchProgress(stdout io.Writer, progress ResearchProgress) {
	switch progress.Stage {
	case progressSearch:
		fmt.Fprintf(stdout, "│    %s query %d/%d · page %d · %d results · expression: %q · %s: %s/%s · remaining: %s\n",
			progress.Provider, progress.QueryIndex, progress.QueryTotal, progress.Page, progress.Hits, progress.Query, progress.Budget, formatByteCount(progress.DownloadedBytes), formatByteCount(progress.ByteLimit), formatByteCount(progress.RemainingBytes))
	case progressQualification:
		label := "Inspected"
		if progress.Final {
			label = "Finished · inspected"
		}
		fmt.Fprintf(stdout, "│    %s %d/%d · qualified: %d/%d · rejected: %d · filtered: %d · %s: %s/%s · remaining: %s\n",
			label, progress.Inspected, progress.InspectionLimit, progress.Qualified, minimumQualifiedCandidates, progress.Rejected, progress.Filtered, progress.Budget, formatByteCount(progress.DownloadedBytes), formatByteCount(progress.ByteLimit), formatByteCount(progress.RemainingBytes))
	case progressSelection:
		if progress.Final {
			fmt.Fprintf(stdout, "│    Finished · selected: %d/3\n", progress.Selected)
			return
		}
		fmt.Fprintf(stdout, "│    Started · candidates in context: %d\n", progress.Candidates)
	}
}

func formatByteCount(bytes int64) string {
	const (
		kib = 1 << 10
		mib = 1 << 20
	)
	switch {
	case bytes >= mib:
		return fmt.Sprintf("%.1fMiB", float64(bytes)/mib)
	case bytes >= kib:
		return fmt.Sprintf("%.1fKiB", float64(bytes)/kib)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

func printIterationCheckpoint(stdout io.Writer, iteration int) {
	fmt.Fprintf(stdout, "│\n│  Checkpoint\n│    Iteration %d saved\n", iteration)
}

func printSelectedExamplePaths(stdout io.Writer, root, detectorID string, examples []string) {
	if len(examples) == 0 {
		return
	}
	fmt.Fprintln(stdout, "│\n│  Selected candidates")
	prefix := filepath.ToSlash(filepath.Join("testdata", "corpus", detectorID)) + "/"
	selected := 0
	for _, example := range examples {
		source := filepath.ToSlash(example)
		relative := strings.TrimPrefix(source, prefix)
		candidateID, _, ok := strings.Cut(relative, "/")
		if !ok || candidateID == "" || relative == source {
			continue
		}
		selected++
		fmt.Fprintf(stdout, "│    %d. Source: %s\n", selected, filepath.Join(root, filepath.FromSlash(source)))
		provenance := filepath.Join(root, filepath.FromSlash(prefix+candidateID+"/provenance.yaml"))
		fmt.Fprintf(stdout, "│       Provenance: %s\n", provenance)
	}
}

// printFlagValues makes each invocation reproducible by showing the effective
// value of every command flag, including values supplied by defaults.
func printFlagValues(stdout io.Writer, command string, fs *flag.FlagSet) {
	values := make([]string, 0)
	fs.VisitAll(func(f *flag.Flag) {
		values = append(values, fmt.Sprintf("--%s=%s", f.Name, f.Value.String()))
	})
	fmt.Fprintf(stdout, "%s configuration: %s\n", command, strings.Join(values, " "))
}

type iterationResult struct {
	code         int
	checkpointed bool
}

// waitForIteration lets the first interrupt finish a valid checkpoint and uses
// the second to cancel research so recovery details can be recorded.
func waitForIteration(result <-chan iterationResult, signals <-chan os.Signal, cancel context.CancelFunc, stdout, stderr io.Writer) (iterationResult, bool) {
	select {
	case iteration := <-result:
		return iteration, false
	case <-signals:
		fmt.Fprintln(stdout, "│\n│  Stopping requested\n│    The active iteration may validate and checkpoint")
	}
	select {
	case iteration := <-result:
		return iteration, true
	case <-signals:
		fmt.Fprintln(stderr, "collection forced stop: terminating active research; recovery is required before resuming")
		cancel()
		return <-result, true
	}
}

func printRecoveryRequired(r Recovery, stderr io.Writer) {
	fmt.Fprintln(stderr, "error: fixture collection recovery is required before another research iteration starts")
	fmt.Fprintf(stderr, "  detector: %s (iteration %d, run %s)\n", r.DetectorID, r.Iteration, r.RunID)
	fmt.Fprintf(stderr, "  last checkpoint: %s\n  progress: %s\n", r.LastCheckpoint, r.ProgressPath)
	validationError := r.ValidationError
	if validationError == "" {
		validationError = "none recorded"
	}
	fmt.Fprintf(stderr, "  validation error: %s\n", validationError)
	changed := "none recorded"
	if len(r.ChangedPaths) > 0 {
		changed = strings.Join(r.ChangedPaths, ", ")
	}
	fmt.Fprintf(stderr, "  changed paths: %s\n", changed)
	cleanPaths := make([]string, 0, len(r.ChangedPaths))
	for _, path := range r.ChangedPaths {
		if path != r.ProgressPath {
			cleanPaths = append(cleanPaths, path)
		}
	}
	fmt.Fprintln(stderr, "  after reviewing the paths, the recommended cleanup is:")
	fmt.Fprintf(stderr, "    git restore --worktree -- %s\n", r.ProgressPath)
	if len(cleanPaths) > 0 {
		fmt.Fprintf(stderr, "    git clean -fdn -- %s\n", strings.Join(cleanPaths, " "))
		fmt.Fprintf(stderr, "    git clean -fd -- %s\n", strings.Join(cleanPaths, " "))
	}
	fmt.Fprintln(stderr, "  or, after review, preserve the listed changes and resume with:")
	fmt.Fprintf(stderr, "    go run ./cmd/fixturecollectorloop run --single --progress %s --allow-dirty\n", r.ProgressPath)
}

func preflightGit(root, progressPath string, allowDirty bool, command []string, stderr io.Writer) error {
	status, err := gitOutput(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("fixture collection requires a Git checkout: %w", err)
	}
	paths := dirtyPaths(status)
	lockPath, err := filepath.Rel(root, progressPath+".lock")
	if err == nil {
		paths = withoutPath(paths, filepath.ToSlash(lockPath))
	}
	if len(paths) > 0 && !allowDirty {
		fmt.Fprintln(stderr, "checkout contains non-ignored changes:")
		for _, path := range paths {
			fmt.Fprintf(stderr, "  %s\n", path)
		}
		fmt.Fprintln(stderr, "refusing to run until you choose exactly one of:")
		fmt.Fprintln(stderr, "  1. DESTRUCTIVE: git reset --hard HEAD")
		fmt.Fprintln(stderr, "     DESTRUCTIVE: git clean -fd")
		fmt.Fprintf(stderr, "  2. Rerun with --allow-dirty: fixturecollectorloop %s --allow-dirty\n", strings.Join(command, " "))
		return errors.New("dirty checkout")
	}
	if allowDirty {
		fmt.Fprintln(stderr, "WARNING: --allow-dirty permits pre-existing changes; their initial Git and filesystem state is preserved for validation and recovery.")
	}
	return warnBranchState(root, stderr)
}

func withoutPath(paths []string, excluded string) []string {
	filtered := paths[:0]
	for _, path := range paths {
		if path != excluded {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

func gitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func dirtyPaths(status string) []string {
	var paths []string
	entries := strings.Split(status, "\x00")
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 4 {
			continue
		}
		paths = append(paths, entry[3:])
		if entry[0] == 'R' || entry[0] == 'C' {
			// The old path is the next NUL-delimited field in porcelain v1 -z.
			if i+1 < len(entries) && entries[i+1] != "" {
				paths = append(paths, entries[i+1])
			}
			i++
		}
	}
	sort.Strings(paths)
	return paths
}

func warnBranchState(root string, stderr io.Writer) error {
	branch, err := gitOutput(root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "WARNING: detached HEAD; collection does not create or switch branches.")
		return nil
	}
	branch = strings.TrimSpace(branch)
	remote, err := gitOutput(root, "remote")
	if err != nil || strings.TrimSpace(remote) == "" {
		fmt.Fprintln(stderr, "WARNING: no Git remote is configured; the default branch cannot be determined.")
		return nil
	}
	remoteName := strings.Fields(remote)[0]
	defaultRef, err := gitOutput(root, "symbolic-ref", "--quiet", "--short", "refs/remotes/"+remoteName+"/HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "WARNING: the default branch cannot be determined without fetching; collection will not fetch or switch branches.")
		return nil
	}
	defaultBranch := strings.TrimPrefix(strings.TrimSpace(defaultRef), remoteName+"/")
	if branch == defaultBranch {
		fmt.Fprintf(stderr, "WARNING: running collection on the default branch %q; collection will not create or switch branches.\n", branch)
	}
	return nil
}

func runIteration(ctx context.Context, root, progressPath string, p Progress, detector *DetectorProgress, researcher Researcher, queryLimit, candidateLimit int, commit, allowDirty bool, reportProgress func(ResearchProgress), stderr io.Writer) (code int, checkpointed bool) {
	checkpoint, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		checkpoint = "working tree (no Git checkpoint available)"
	} else {
		checkpoint = strings.TrimSpace(checkpoint)
	}
	recovery := newRecovery(progressPath, detector, checkpoint, commit, allowDirty)
	p.Recovery = &recovery
	if err := writeProgress(progressPath, p); err != nil {
		fmt.Fprintf(stderr, "error: checkpoint recovery state: %v\n", err)
		return 1, false
	}
	var before map[string]string
	var beforeDirectories map[string]struct{}
	failure := "collection iteration did not complete"
	progressCheckpointed := false
	defer func() {
		if code != 0 && !progressCheckpointed {
			recordRecovery(root, progressPath, p, recovery, before, failure)
			removeNewEmptyDirectories(filepath.Join(root, "testdata", "corpus", detector.ID), beforeDirectories)
		}
	}()
	before, err = snapshot(root)
	if err != nil {
		failure = err.Error()
		fmt.Fprintf(stderr, "error: snapshot collection state: %v\n", err)
		return 1, false
	}
	corpusDir := filepath.Join(root, "testdata", "corpus", detector.ID)
	beforeDirectories, err = snapshotDirectories(corpusDir)
	if err != nil {
		failure = err.Error()
		fmt.Fprintf(stderr, "error: snapshot collection directories: %v\n", err)
		return 1, false
	}
	references, err := acceptedCorpusReferences(root, detector)
	if err != nil {
		failure = err.Error()
		fmt.Fprintf(stderr, "error: read accepted corpus references: %v\n", err)
		return 1, false
	}
	presented := presentedCandidateIDs(detector)
	result, err := researcher.Research(ctx, Iteration{
		DetectorID:            detector.ID,
		CorpusDir:             corpusDir,
		Iteration:             detector.Iterations + 1,
		QueryLimit:            queryLimit,
		CandidateLimit:        candidateLimit,
		QueryPlan:             append([]string(nil), detector.QueryPlan...),
		PacketTokens:          p.Limits.PacketTokens,
		AcceptedReferences:    references,
		PresentedCandidateIDs: presented,
		PriorDecisionStates:   append([]DecisionState(nil), detector.DecisionStates...),
		ReportProgress:        reportProgress,
	})
	if err != nil {
		failure = err.Error()
		fmt.Fprintf(stderr, "error: collection research: %v\n", err)
		return 1, false
	}
	outcome := result.Outcome
	after, err := snapshot(root)
	if err != nil {
		failure = err.Error()
		fmt.Fprintf(stderr, "error: snapshot collection state: %v\n", err)
		return 1, false
	}
	added, err := validateDelta(before, after, filepath.Join("testdata", "corpus", detector.ID))
	if err != nil {
		failure = err.Error()
		fmt.Fprintf(stderr, "error: unvalidated collection changes: %v\n", err)
		return 1, false
	}
	if len(result.Accepted) != 0 {
		selectedCount := len(result.Accepted)
		survivors := result.Accepted[:0]
		for _, accepted := range result.Accepted {
			if err := validateAcceptedCandidate(accepted, detector.ID, after); err == nil {
				survivors = append(survivors, accepted)
				continue
			}
			if err := removeRejectedCandidate(root, detector.ID, accepted); err != nil {
				failure = err.Error()
				fmt.Fprintf(stderr, "error: remove rejected candidate: %v\n", err)
				return 1, false
			}
			outcome.Rejections = append(outcome.Rejections, "final-validation-mutation")
		}
		result.Accepted = survivors
		if len(survivors) == 0 {
			outcome.Result = "unsuccessful"
			outcome.Added = nil
		} else if len(survivors) != selectedCount {
			outcome.Result = "accepted"
		}
		if len(survivors) != selectedCount {
			after, err = snapshot(root)
			if err != nil {
				failure = err.Error()
				fmt.Fprintf(stderr, "error: snapshot collection state: %v\n", err)
				return 1, false
			}
			added, err = validateDelta(before, after, filepath.Join("testdata", "corpus", detector.ID))
			if err != nil {
				failure = err.Error()
				fmt.Fprintf(stderr, "error: unvalidated collection changes: %v\n", err)
				return 1, false
			}
			outcome.Added = make([]string, 0, len(added))
			for _, path := range added {
				outcome.Added = append(outcome.Added, strings.TrimPrefix(path, "testdata/corpus/"+detector.ID+"/"))
			}
		}
	}
	if outcome.Result != "accepted" && outcome.Result != "unsuccessful" {
		failure = "research returned an invalid outcome result"
		fmt.Fprintln(stderr, "error: collection research: invalid outcome result")
		return 1, false
	}
	if len(outcome.Queries) > queryLimit || len(outcome.Candidates) > candidateLimit {
		failure = "research outcome exceeds configured bounds"
		fmt.Fprintln(stderr, "error: collection research: outcome exceeds the configured query or candidate bound")
		return 1, false
	}
	if outcome.Result == "unsuccessful" && len(added) != 0 {
		failure = "unsuccessful research changed the corpus"
		fmt.Fprintln(stderr, "error: unvalidated collection changes: unsuccessful research changed the corpus")
		return 1, false
	}
	if outcome.Result == "accepted" && len(added) == 0 {
		failure = "research added no corpus example"
		fmt.Fprintln(stderr, "error: unvalidated collection changes: research added no corpus example")
		return 1, false
	}
	if outcome.Result == "accepted" {
		if err := validateOutcome(outcome, detector.ID, added); err != nil {
			failure = err.Error()
			fmt.Fprintf(stderr, "error: unvalidated collection changes: %v\n", err)
			return 1, false
		}
		if err := validateCorpus(detector.ID, added, before, after); err != nil {
			failure = err.Error()
			fmt.Fprintf(stderr, "error: unvalidated collection changes: %v\n", err)
			return 1, false
		}
		if err := validateAcceptedCandidates(result.Accepted, detector.ID, after); err != nil {
			failure = err.Error()
			fmt.Fprintf(stderr, "error: unvalidated collection changes: %v\n", err)
			return 1, false
		}
	}
	detector.Iterations++
	detector.State = stateInProgress
	detector.Examples = append(detector.Examples, corpusExampleFiles(added)...)
	detector.Queries = append(detector.Queries, outcome.Queries...)
	detector.Candidates = append(detector.Candidates, outcome.Candidates...)
	detector.FilteredSearchHits = mergeCounts(detector.FilteredSearchHits, outcome.FilteredSearchHits)
	detector.Rejections = append(detector.Rejections, outcome.Rejections...)
	detector.Omitted = append(detector.Omitted, outcome.Omitted...)
	if result.Decision != nil {
		detector.DecisionStates = append(detector.DecisionStates, *result.Decision)
	}
	detector.History = append(detector.History, iterationRecord(detector.Iterations, outcome, result.Accepted, result.Decision))
	if len(detector.Examples) >= 3 {
		detector.State = stateComplete
	}
	if detector.Iterations >= maxIterations && detector.State != stateComplete {
		detector.State = stateNeedsCollectionReview
	}
	if detector.State != stateComplete && len(detector.Examples) < defaultTarget {
		if result.NoDistinctDecisionState {
			detector.State = stateNeedsCollectionReview
		} else if noPresentableCandidates(outcome) {
			detector.State = stateNeedsContentReview
		}
	}
	p.Recovery = nil
	if err := writeProgress(progressPath, p); err != nil {
		failure = err.Error()
		fmt.Fprintf(stderr, "error: checkpoint collection progress: %v\n", err)
		return 1, false
	}
	progressCheckpointed = true
	if commit {
		if err := commitCheckpoint(root, progressPath, detector.ID, detector.Iterations, outcome.Result, added); err != nil {
			fmt.Fprintf(stderr, "error: collection commit failed after a valid checkpoint: %v\n", err)
			fmt.Fprintln(stderr, "validated corpus and progress remain as uncommitted collection state. Commit them manually, discard the iteration deliberately, or rerun without --commit after reviewing Git status.")
			return 1, false
		}
	}
	return 0, true
}

func removeRejectedCandidate(root, detectorID string, accepted AcceptedCandidate) error {
	if accepted.Directory != accepted.Candidate.ID || accepted.Directory == "" || strings.Contains(accepted.Directory, string(filepath.Separator)) {
		return errors.New("rejected candidate has an unsafe materialization directory")
	}
	path := filepath.Join(root, "testdata", "corpus", detectorID, accepted.Directory)
	return os.RemoveAll(path)
}

// acceptedCorpusReferences reconstructs the limited comparison corpus from
// exact local source and provenance records. It never writes or retains those
// contents beyond the active selector handoff.
func acceptedCorpusReferences(root string, detector *DetectorProgress) ([]AcceptedCorpusReference, error) {
	if len(detector.Examples) == 0 {
		return nil, nil
	}
	if len(detector.Examples) > 2 {
		return nil, errors.New("selector must not run with three accepted corpus examples")
	}
	references := make([]AcceptedCorpusReference, 0, len(detector.Examples))
	prefix := filepath.ToSlash(filepath.Join("testdata", "corpus", detector.ID)) + "/"
	for _, relativeSource := range detector.Examples {
		if filepath.IsAbs(relativeSource) || strings.HasPrefix(filepath.Clean(relativeSource), "..") {
			return nil, errors.New("accepted corpus source path is unsafe")
		}
		cleanRelative := filepath.ToSlash(filepath.Clean(relativeSource))
		if !strings.HasPrefix(cleanRelative, prefix) {
			return nil, errors.New("accepted corpus source is outside its detector corpus")
		}
		parts := strings.Split(strings.TrimPrefix(cleanRelative, prefix), "/")
		if len(parts) < 2 || parts[0] == "" {
			return nil, errors.New("accepted corpus source has no example directory")
		}
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativeSource)))
		if err != nil {
			return nil, err
		}
		provenancePath := filepath.Join(root, "testdata", "corpus", detector.ID, parts[0], "provenance.yaml")
		contents, err := os.ReadFile(provenancePath)
		if err != nil {
			return nil, err
		}
		p, err := parseProvenance(string(contents))
		if err != nil {
			return nil, err
		}
		if err := validateProvenance(p, detector.ID); err != nil {
			return nil, err
		}
		if hash(string(source)) != strings.ToLower(p.SHA256) {
			return nil, errors.New("accepted corpus source does not match provenance hash")
		}
		references = append(references, AcceptedCorpusReference{
			Candidate: sourceCandidateFromProvenance(p, source),
			Rationale: p.Rationale,
		})
	}
	sort.Slice(references, func(i, j int) bool { return references[i].Candidate.ID < references[j].Candidate.ID })
	return references, nil
}

func presentedCandidateIDs(detector *DetectorProgress) map[string]bool {
	omitted := make(map[string]bool, len(detector.Omitted))
	for _, id := range detector.Omitted {
		omitted[id] = true
	}
	presented := make(map[string]bool, len(detector.Candidates))
	for _, id := range detector.Candidates {
		if !omitted[id] {
			presented[id] = true
		}
	}
	return presented
}

// noPresentableCandidates distinguishes an exhausted model-presentability set
// from other unsuccessful bounded research. Only the former establishes
// content review.
func noPresentableCandidates(outcome Outcome) bool {
	if len(outcome.Candidates) != 0 {
		return false
	}
	for _, reason := range outcome.Rejections {
		if reason == string(reasonSourceUTF8) || reason == string(reasonSourcePacketSize) {
			return true
		}
	}
	return false
}

func sourceCandidateFromProvenance(p Provenance, source []byte) SourceCandidate {
	return SourceCandidate{
		ID:            p.CandidateID,
		Provider:      p.Provider,
		Repository:    p.Repository,
		RepositoryURL: p.RepositoryURL,
		DefaultBranch: p.DefaultBranch,
		Commit:        p.Commit,
		OriginalPath:  p.OriginalPath,
		RetrievedAt:   p.RetrievedAt,
		Source:        source,
		SourceSHA256:  p.SHA256,
		License: LicenseEvidence{
			SPDX:      p.License.SPDX,
			Path:      p.License.Path,
			Permalink: p.License.Permalink,
			SHA256:    p.License.SHA256,
		},
	}
}

func iterationRecord(iteration int, outcome Outcome, accepted []AcceptedCandidate, decision *DecisionState) IterationRecord {
	record := IterationRecord{
		Iteration:          iteration,
		Result:             outcome.Result,
		Queries:            append([]string(nil), outcome.Queries...),
		Candidates:         append([]string(nil), outcome.Candidates...),
		FilteredSearchHits: cloneCounts(outcome.FilteredSearchHits),
		Rejections:         append([]string(nil), outcome.Rejections...),
		Omitted:            append([]string(nil), outcome.Omitted...),
		Decision:           decision,
	}
	for _, candidate := range accepted {
		record.AcceptedIDs = append(record.AcceptedIDs, candidate.Candidate.ID)
	}
	sort.Strings(record.AcceptedIDs)
	return record
}

func mergeCounts(dst, src map[string]int) map[string]int {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]int, len(src))
	}
	for reason, count := range src {
		dst[reason] += count
	}
	return dst
}

func cloneCounts(src map[string]int) map[string]int {
	return mergeCounts(nil, src)
}

func newRecovery(progressPath string, detector *DetectorProgress, checkpoint string, commit, allowDirty bool) Recovery {
	runID := fmt.Sprintf("%d", now().UnixNano())
	return Recovery{
		RunID:          runID,
		DetectorID:     detector.ID,
		Iteration:      detector.Iterations + 1,
		LastCheckpoint: checkpoint,
		ProgressPath:   progressPath,
		Commit:         commit,
		AllowDirty:     allowDirty,
	}
}

func recordRecovery(root, progressPath string, p Progress, recovery Recovery, before map[string]string, failure string) {
	recovery.ValidationError = failure
	if after, err := snapshot(root); err == nil {
		recovery.ChangedPaths = changedPaths(before, after)
	}
	p.Recovery = &recovery
	_ = writeProgress(progressPath, p)
}

func changedPaths(before, after map[string]string) []string {
	paths := map[string]struct{}{}
	for path, contents := range after {
		if before[path] != contents {
			paths[path] = struct{}{}
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			paths[path] = struct{}{}
		}
	}
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}

func commitCheckpoint(root, progressPath, detectorID string, iteration int, result string, added []string) error {
	progressRel, err := filepath.Rel(root, progressPath)
	if err != nil || progressRel == ".." || strings.HasPrefix(progressRel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("collection progress must be inside the Git checkout: %s", progressPath)
	}
	paths := make([]string, 0, len(added)+1)
	paths = append(paths, filepath.ToSlash(progressRel))
	paths = append(paths, added...)
	for _, path := range paths {
		if _, err := gitOutput(root, "add", "--", path); err != nil {
			return err
		}
	}
	message := fmt.Sprintf("collect %s corpus examples (iteration %d)", detectorID, iteration)
	if result == "unsuccessful" {
		message = fmt.Sprintf("record %s collection attempt (iteration %d)", detectorID, iteration)
	}
	args := []string{"-c", "commit.gpgSign=false", "commit", "--only", "--no-gpg-sign", "--no-verify", "-m", message, "--"}
	args = append(args, paths...)
	_, err = gitCommit(root, args...)
	return err
}

func lockProgress(progressPath string) (func(), error) {
	path := progressPath + ".lock"
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("another collection run holds %s; inspect it and remove only a confirmed stale lock", path)
	}
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(f, "pid: %d\n", os.Getpid()); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return func() { _ = os.Remove(path) }, nil
}

func collectionSummary(p Progress, stdout io.Writer) int {
	var complete, queryReview, contentReview, collectionReview, active int
	queryIDs := []string{}
	contentIDs := []string{}
	collectionIDs := []string{}
	for _, d := range p.Detectors {
		switch d.State {
		case stateComplete:
			complete++
		case stateNeedsQueryReview:
			queryReview++
			queryIDs = append(queryIDs, d.ID)
		case stateNeedsContentReview:
			contentReview++
			contentIDs = append(contentIDs, d.ID)
		case stateNeedsCollectionReview:
			collectionReview++
			collectionIDs = append(collectionIDs, d.ID)
		default:
			active++
		}
	}
	fmt.Fprintf(stdout, "\nCollection state\n  Complete: %d\n  Needs query review: %d (%s)\n  Needs content review: %d (%s)\n  Needs collection review: %d (%s)\n  Remaining: %d\n",
		complete, queryReview, strings.Join(queryIDs, ","), contentReview, strings.Join(contentIDs, ","), collectionReview, strings.Join(collectionIDs, ","), active)
	return 0
}

func validateCorpus(detectorID string, added []string, before, after map[string]string) error {
	byRoot := map[string][]string{}
	for _, path := range added {
		parts := strings.Split(path, "/")
		if len(parts) < 5 {
			return fmt.Errorf("%s does not preserve a source root", path)
		}
		root := strings.Join(parts[:4], "/")
		byRoot[root] = append(byRoot[root], path)
	}
	knownIdentity, knownContent := map[string]bool{}, map[string]bool{}
	for path, content := range before {
		if filepath.Base(path) == "provenance.yaml" {
			p, err := parseProvenance(content)
			if err == nil {
				knownIdentity[p.Repository+"@"+p.Commit+":"+p.OriginalPath] = true
			}
			continue
		}
		knownContent[hash(content)] = true
	}
	for root, paths := range byRoot {
		provenancePath := root + "/provenance.yaml"
		contents, ok := after[provenancePath]
		if !ok {
			return fmt.Errorf("%s has no provenance record", root)
		}
		p, err := parseProvenance(contents)
		if err != nil {
			return fmt.Errorf("%s: %w", provenancePath, err)
		}
		if err := validateProvenance(p, detectorID); err != nil {
			return fmt.Errorf("%s: %w", provenancePath, err)
		}
		identity := p.Repository + "@" + p.Commit + ":" + p.OriginalPath
		if knownIdentity[identity] {
			return fmt.Errorf("duplicate upstream identity %s", identity)
		}
		knownIdentity[identity] = true
		expected := root + "/" + strings.TrimPrefix(filepath.ToSlash(filepath.Clean(p.OriginalPath)), "./")
		if _, ok := after[expected]; !ok {
			return fmt.Errorf("%s does not contain original path %s", root, p.OriginalPath)
		}
		sourceCount := 0
		for _, path := range paths {
			if path == provenancePath {
				continue
			}
			sourceCount++
			content := after[path]
			if len(content) > maxExampleSize {
				return fmt.Errorf("%s exceeds the %d byte size limit", path, maxExampleSize)
			}
			if isLFSPointer(content) {
				return fmt.Errorf("%s is a Git LFS pointer", path)
			}
			if containsUnsafeContent(content) {
				return fmt.Errorf("%s contains credentials, authentication material, or personal data", path)
			}
			if knownContent[hash(content)] {
				return fmt.Errorf("%s duplicates accepted corpus content", path)
			}
			knownContent[hash(content)] = true
			if path == expected && !strings.EqualFold(hash(content), p.SHA256) {
				return fmt.Errorf("%s does not match its provenance SHA-256", path)
			}
		}
		if sourceCount != 1 {
			return fmt.Errorf("%s must contain exactly one source file", root)
		}
	}
	return nil
}

func parseProvenance(contents string) (Provenance, error) {
	var p Provenance
	d := yaml.NewDecoder(strings.NewReader(contents))
	d.KnownFields(true)
	if err := d.Decode(&p); err != nil {
		return Provenance{}, err
	}
	return p, nil
}

func validateProvenance(p Provenance, detectorID string) error {
	if p.Version != 2 || p.DetectorID != detectorID || p.CandidateID == "" || p.Provider != "github" || p.Repository == "" ||
		p.RepositoryURL == "" || p.Commit == "" || p.OriginalPath == "" || p.Permalink == "" ||
		p.RetrievedAt == "" || p.SHA256 == "" || p.DefaultBranch == "" || p.License.Path == "" || p.License.Permalink == "" || p.License.SHA256 == "" || p.Rationale == "" {
		return errors.New("missing required provenance")
	}
	if p.CandidateID != stableCandidateID(p.Provider, p.Repository, p.Commit, p.OriginalPath) {
		return errors.New("candidate ID does not match immutable identity")
	}
	if !approvedLicenses[p.License.SPDX] {
		return fmt.Errorf("license %q is not approved", p.License.SPDX)
	}
	if filepath.IsAbs(p.OriginalPath) || strings.HasPrefix(filepath.Clean(p.OriginalPath), "..") {
		return errors.New("original path is unsafe")
	}
	if !strings.Contains(p.Permalink, "/blob/"+p.Commit+"/") {
		return errors.New("permalink is not pinned to immutable commit")
	}
	if !strings.Contains(p.License.Permalink, "/blob/"+p.Commit+"/") || len(p.SHA256) != 64 || len(p.License.SHA256) != 64 {
		return errors.New("SHA-256 is invalid")
	}
	return nil
}

func hash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", sum)
}
func isLFSPointer(content string) bool {
	return strings.HasPrefix(content, "version https://git-lfs.github.com/spec/")
}
func containsUnsafeContent(content string) bool {
	return unsafeContentPattern.MatchString(content)
}

// This deliberately recognises credentials and direct personal identifiers,
// not instruction-like prose. Qualification rejects rather than redacts.
var unsafeContentPattern = regexp.MustCompile(`(?im)(?:\b(?:password|passwd|secret|api[_-]?key)\s*[:=]|\bauthorization\s*:|-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----|\bgh[pousr]_[A-Za-z0-9_]{20,}|\bnpm_[A-Za-z0-9]{20,}|\bAKIA[0-9A-Z]{16}\b|//[^\s/:]+:[^\s@/]+@[^\s/]+|:_authToken\s*[:=]|\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b)`)

func selectDetector(detectors []DetectorProgress, target string) *DetectorProgress {
	if target != "" {
		detector := findDetector(detectors, target)
		if detector != nil && isEligible(*detector) {
			return detector
		}
		return nil
	}
	for i := range detectors {
		if detectors[i].State == stateInProgress {
			return &detectors[i]
		}
	}
	for i := range detectors {
		if detectors[i].State == statePending {
			return &detectors[i]
		}
	}
	return nil
}

func findDetector(detectors []DetectorProgress, id string) *DetectorProgress {
	for i := range detectors {
		if detectors[i].ID == id {
			return &detectors[i]
		}
	}
	return nil
}

func isEligible(d DetectorProgress) bool {
	return (d.State == statePending || d.State == stateInProgress) && d.Iterations < maxIterations
}

func readProgress(path string) (Progress, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Progress{}, err
	}
	var p Progress
	d := yaml.NewDecoder(bytes.NewReader(b))
	d.KnownFields(true)
	if err := d.Decode(&p); err != nil {
		return Progress{}, err
	}
	if p.Version != progressVersion {
		return Progress{}, errors.New("collection progress is from an older format; initialize a fresh collection with initialize-progress")
	}
	return validateProgress(p)
}

func validState(state string) bool {
	switch state {
	case statePending, stateInProgress, stateComplete, stateNeedsQueryReview, stateNeedsContentReview, stateNeedsCollectionReview:
		return true
	default:
		return false
	}
}

func validCollectionLimits(limits CollectionLimits) bool {
	return limits.Queries > 0 && limits.ResultPages > 0 && limits.CandidateInspections > 0 &&
		limits.DecodedResponseBytes > 0 && limits.PacketTokens > 0 && limits.SelectorInvocations > 0 &&
		limits.SourceBytes > 0 && limits.ValidIterations == maxIterations
}

func hasCapability(capabilities []string, wanted string) bool {
	for _, capability := range capabilities {
		if capability == wanted {
			return true
		}
	}
	return false
}

func validateCoverageInventory(root string, rules analyze.Ruleset) error {
	data, err := os.ReadFile(filepath.Join(root, "DEPENDENCY_COVERAGE.md"))
	if err != nil {
		return fmt.Errorf("read DEPENDENCY_COVERAGE.md: %w", err)
	}
	listed := map[analyze.DetectorID]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		columns := strings.Split(line, "|")
		if len(columns) != 7 || !strings.Contains(columns[5], "select") {
			continue
		}
		id := strings.Trim(strings.TrimSpace(columns[1]), "`")
		if id != "Detector ID" && !strings.Contains(columns[5], "extract") {
			listed[analyze.DetectorID(id)] = true
		}
	}
	eligible := 0
	for _, capability := range rules.DetectorCapabilities() {
		if hasCapability(capability.Capabilities, "extract") {
			continue
		}
		eligible++
		if !listed[capability.ID] {
			return fmt.Errorf("coverage inventory omits eligible detector %q", capability.ID)
		}
	}
	if len(listed) != eligible {
		return fmt.Errorf("eligible detector count differs: coverage inventory has %d, rule classifier has %d", len(listed), eligible)
	}
	return nil
}

func writeProgress(path string, p Progress) error {
	if _, err := validateProgress(p); err != nil {
		return err
	}
	b, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".progress-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	return syncProgressDirectory(filepath.Dir(path))
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func validateProgress(p Progress) (Progress, error) {
	if p.Version != progressVersion || len(p.Detectors) == 0 || !validCollectionLimits(p.Limits) {
		return Progress{}, errors.New("invalid collection progress schema")
	}
	seen := map[string]bool{}
	for _, detector := range p.Detectors {
		if detector.ID == "" || !validState(detector.State) || detector.Iterations < 0 || detector.Iterations > p.Limits.ValidIterations ||
			(detector.Target != 0 && (detector.Target < 3 || detector.Target > 5)) {
			return Progress{}, errors.New("invalid detector progress")
		}
		if seen[detector.ID] {
			return Progress{}, errors.New("duplicate detector progress")
		}
		seen[detector.ID] = true
		if detector.State == stateNeedsQueryReview {
			if detector.Iterations != 0 || len(detector.QueryPlan) != 0 || detector.QueryReviewReason == "" {
				return Progress{}, errors.New("invalid query-review detector progress")
			}
		} else if len(detector.QueryPlan) == 0 || !isSortedUnique(detector.QueryPlan) || detector.QueryReviewReason != "" {
			return Progress{}, errors.New("invalid detector query plan")
		}
		target := detector.Target
		if target == 0 {
			target = defaultTarget
		}
		switch detector.State {
		case statePending:
			if detector.Iterations != 0 || len(detector.Examples) != 0 {
				return Progress{}, errors.New("invalid pending detector progress")
			}
		case stateComplete:
			if len(detector.Examples) < 3 {
				return Progress{}, errors.New("invalid complete detector progress")
			}
		case stateNeedsContentReview:
			if detector.Iterations == 0 || len(detector.Examples) >= defaultTarget {
				return Progress{}, errors.New("invalid content-review detector progress")
			}
		case stateNeedsCollectionReview:
			if len(detector.Examples) >= defaultTarget {
				return Progress{}, errors.New("invalid collection-review detector progress")
			}
		}
	}
	return p, nil
}

func snapshot(root string) (map[string]string, error) {
	files := map[string]string{}
	localLogDir := filepath.Join(root, ".deplens", "fixture-collection-logs")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if entry.IsDir() {
			if path == localLogDir {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			files[filepath.ToSlash(rel)] = "symlink"
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	return files, err
}

func snapshotDirectories(root string) (map[string]struct{}, error) {
	directories := map[string]struct{}{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root || !entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		directories[filepath.ToSlash(rel)] = struct{}{}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return directories, nil
	}
	return directories, err
}

func removeNewEmptyDirectories(root string, before map[string]struct{}) {
	after, err := snapshotDirectories(root)
	if err != nil {
		return
	}
	paths := make([]string, 0, len(after))
	for path := range after {
		if _, existed := before[path]; !existed {
			paths = append(paths, path)
		}
	}
	sort.Slice(paths, func(i, j int) bool {
		return strings.Count(paths[i], "/") > strings.Count(paths[j], "/")
	})
	for _, path := range paths {
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(path)))
	}
}

func validateDelta(before, after map[string]string, allowed string) ([]string, error) {
	allowed += "/"
	var added []string
	for path, contents := range after {
		old, existed := before[path]
		if existed && old == contents {
			continue
		}
		if !strings.HasPrefix(path, allowed) {
			return nil, fmt.Errorf("%s is outside the selected detector corpus", path)
		}
		if existed {
			return nil, fmt.Errorf("%s modifies an existing corpus file", path)
		}
		if contents == "symlink" {
			return nil, fmt.Errorf("%s is a symlink", path)
		}
		added = append(added, path)
	}
	for path := range before {
		if _, exists := after[path]; !exists {
			return nil, fmt.Errorf("%s was deleted", path)
		}
	}
	sort.Strings(added)
	return added, nil
}

func validateOutcome(outcome Outcome, detectorID string, added []string) error {
	if outcome.Result != "accepted" {
		return fmt.Errorf("research outcome must be accepted")
	}
	declared := append([]string(nil), outcome.Added...)
	sort.Strings(declared)
	if len(declared) != len(added) {
		return fmt.Errorf("research result does not match the added files")
	}
	for i := range added {
		if strings.TrimPrefix(added[i], "testdata/corpus/"+detectorID+"/") == declared[i] {
			continue
		}
		return fmt.Errorf("research result does not match the added files")
	}
	hasProvenance := false
	for _, path := range declared {
		if filepath.Base(path) == "provenance.yaml" {
			hasProvenance = true
		}
	}
	if !hasProvenance {
		return fmt.Errorf("research result added no provenance record")
	}
	if len(corpusExampleFiles(added)) == 0 {
		return fmt.Errorf("research result added no corpus example")
	}
	return nil
}

func corpusExampleFiles(paths []string) []string {
	examples := make([]string, 0, len(paths))
	for _, path := range paths {
		if filepath.Base(path) != "provenance.yaml" {
			examples = append(examples, path)
		}
	}
	return examples
}

type commaList []string

func (l *commaList) String() string { return strings.Join(*l, ",") }
func (l *commaList) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*l = append(*l, part)
		}
	}
	return nil
}
