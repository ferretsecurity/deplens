// fixturecollectorloop collects authentic dependency-source corpus examples.
package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	statePending    = "pending"
	stateInProgress = "in-progress"
	stateComplete   = "complete"
	stateBlocked    = "blocked"
	stateExcluded   = "excluded"
	maxIterations   = 7
	defaultTarget   = 3
)

type Progress struct {
	Version   int                `yaml:"version"`
	Detectors []DetectorProgress `yaml:"detectors"`
}

type DetectorProgress struct {
	ID         string   `yaml:"id"`
	State      string   `yaml:"state"`
	Iterations int      `yaml:"iterations"`
	Examples   []string `yaml:"examples"`
	Target     int      `yaml:"target,omitempty"`
	Queries    []string `yaml:"queries,omitempty"`
	Candidates []string `yaml:"candidates,omitempty"`
	Rejections []string `yaml:"rejections,omitempty"`
}

type Iteration struct {
	DetectorID        string
	CorpusDir         string
	Iteration         int
	QueryLimit        int
	CandidateLimit    int
	PriorHistory      []string
	MissingDimensions []string
}

type Outcome struct {
	Result     string
	Added      []string
	Queries    []string
	Candidates []string
	Rejections []string
}

var now = time.Now
var gitCommit = gitOutput

const maxExampleSize = 2 << 20

var approvedLicenses = map[string]bool{
	"MIT": true, "Apache-2.0": true, "BSD-2-Clause": true, "BSD-3-Clause": true,
	"ISC": true, "0BSD": true, "CC0-1.0": true, "Unlicense": true,
}

type Provenance struct {
	Version       int      `yaml:"version"`
	DetectorID    string   `yaml:"detector_id"`
	Provider      string   `yaml:"provider"`
	Repository    string   `yaml:"repository"`
	RepositoryURL string   `yaml:"repository_url"`
	Commit        string   `yaml:"commit"`
	OriginalPath  string   `yaml:"original_path"`
	Permalink     string   `yaml:"permalink"`
	RetrievedAt   string   `yaml:"retrieved_at"`
	SHA256        string   `yaml:"sha256"`
	License       string   `yaml:"license"`
	LicenseURL    string   `yaml:"license_url"`
	ProjectKind   string   `yaml:"project_kind"`
	VariationTags []string `yaml:"variation_tags"`
	Rationale     string   `yaml:"rationale"`
}

type Agent interface {
	Run(Iteration) (Outcome, error)
}

type unavailableAgent struct{}

func (unavailableAgent) Run(Iteration) (Outcome, error) {
	return Outcome{}, errors.New("no Codex agent is configured; inject an agent through the command seam")
}

func main() { os.Exit(run(os.Args[1:], ".", os.Stdout, os.Stderr, newCodexAgent(".", os.Stdout))) }

func run(args []string, root string, stdout, stderr io.Writer, agent Agent) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: fixturecollectorloop <initialize-progress|run> [flags]")
		return 1
	}
	switch args[0] {
	case "initialize-progress":
		return initialize(args[1:], root, stdout, stderr)
	case "run":
		return collect(args[1:], root, stdout, stderr, agent)
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
	if len(detectors) == 0 {
		fmt.Fprintln(stderr, "error: at least one --detector is required for this tracer bullet")
		return 1
	}
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
	p := Progress{Version: 1, Detectors: make([]DetectorProgress, len(detectors))}
	seen := map[string]bool{}
	for i, id := range detectors {
		if id == "" || seen[id] {
			fmt.Fprintf(stderr, "error: detector identifiers must be non-empty and unique\n")
			return 1
		}
		seen[id] = true
		p.Detectors[i] = DetectorProgress{ID: id, State: statePending, Target: *target, Examples: []string{}}
	}
	if err := writeProgress(*progressPath, p); err != nil {
		fmt.Fprintf(stderr, "error: initialize collection progress: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "initialized collection progress: %s (%d detectors)\n", *progressPath, len(p.Detectors))
	return 0
}

func collect(args []string, root string, stdout, stderr io.Writer, agent Agent) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	progressPath := fs.String("progress", filepath.Join(root, ".deplens", "fixture-collection.yaml"), "collection progress path")
	target := fs.String("detector", "", "run one detector")
	single := fs.Bool("single", false, "run one automatically selected iteration")
	duration := fs.Duration("duration", 8*time.Hour, "soft limit for scheduling new iterations")
	queryLimit := fs.Int("query-limit", 5, "maximum queries recorded by one iteration")
	candidateLimit := fs.Int("candidate-limit", 20, "maximum candidates recorded by one iteration")
	allowDirty := fs.Bool("allow-dirty", false, "allow a checkout that already has non-ignored changes")
	commit := fs.Bool("commit", false, "create one local collection commit for each valid iteration")
	retainLogs := fs.Bool("retain-logs", false, "retain successful Codex JSONL logs (logs may contain sensitive content)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *duration < 0 {
		fmt.Fprintln(stderr, "error: duration must not be negative")
		return 1
	}
	if *queryLimit < 1 || *candidateLimit < 1 {
		fmt.Fprintln(stderr, "error: query-limit and candidate-limit must be positive")
		return 1
	}
	if configurable, ok := agent.(interface{ SetRetainLogs(bool) }); ok {
		configurable.SetRetainLogs(*retainLogs)
	}
	unlock, err := lockProgress(*progressPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer unlock()
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
	if preflight, ok := agent.(interface{ Preflight() error }); ok {
		if err := preflight.Preflight(); err != nil {
			fmt.Fprintf(stderr, "error: collection authentication preflight: %v\n", err)
			return 1
		}
	}
	p, err := readProgress(*progressPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: read collection progress: %v\n", err)
		return 1
	}
	deadline := now().Add(*duration)
	for {
		if !now().Before(deadline) {
			fmt.Fprintln(stdout, "collection stopped: soft duration reached; the latest checkpoint is preserved")
			return 0
		}
		detector := selectDetector(p.Detectors, *target)
		if detector == nil {
			return collectionSummary(p, stdout)
		}
		if detector.Target == 0 {
			detector.Target = defaultTarget
		}
		if detector.Iterations >= maxIterations {
			detector.State = stateBlocked
			if err := writeProgress(*progressPath, p); err != nil {
				fmt.Fprintf(stderr, "error: checkpoint collection progress: %v\n", err)
				return 1
			}
			if *single || *target != "" {
				return collectionSummary(p, stdout)
			}
			continue
		}
		code, checkpointed := runIteration(root, *progressPath, p, detector, agent, *queryLimit, *candidateLimit, *commit, stderr)
		if code != 0 {
			return code
		}
		if checkpointed {
			fmt.Fprintf(stdout, "checkpoint: %s iteration %d\n", detector.ID, detector.Iterations)
		}
		if *single || *target != "" {
			return collectionSummary(p, stdout)
		}
	}
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

func runIteration(root, progressPath string, p Progress, detector *DetectorProgress, agent Agent, queryLimit, candidateLimit int, commit bool, stderr io.Writer) (int, bool) {
	before, err := snapshot(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: snapshot collection state: %v\n", err)
		return 1, false
	}
	corpusDir := filepath.Join(root, "testdata", "corpus", detector.ID)
	history := append(append([]string{}, detector.Queries...), detector.Candidates...)
	history = append(history, detector.Rejections...)
	outcome, err := agent.Run(Iteration{
		DetectorID:        detector.ID,
		CorpusDir:         corpusDir,
		Iteration:         detector.Iterations + 1,
		QueryLimit:        queryLimit,
		CandidateLimit:    candidateLimit,
		PriorHistory:      history,
		MissingDimensions: []string{"source variation not yet recorded by collection progress"},
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: collection agent: %v\n", err)
		return 1, false
	}
	after, err := snapshot(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: snapshot collection state: %v\n", err)
		return 1, false
	}
	added, err := validateDelta(before, after, filepath.Join("testdata", "corpus", detector.ID))
	if err != nil {
		fmt.Fprintf(stderr, "error: unvalidated collection changes: %v\n", err)
		return 1, false
	}
	if outcome.Result != "accepted" && outcome.Result != "unsuccessful" {
		fmt.Fprintln(stderr, "error: collection agent: invalid outcome result")
		return 1, false
	}
	if len(outcome.Queries) > queryLimit || len(outcome.Candidates) > candidateLimit {
		fmt.Fprintln(stderr, "error: collection agent: outcome exceeds the configured query or candidate bound")
		return 1, false
	}
	if outcome.Result == "unsuccessful" && len(added) != 0 {
		fmt.Fprintln(stderr, "error: unvalidated collection changes: unsuccessful research changed the corpus")
		return 1, false
	}
	if outcome.Result == "accepted" && len(added) == 0 {
		fmt.Fprintln(stderr, "error: unvalidated collection changes: agent added no corpus example")
		return 1, false
	}
	if outcome.Result == "accepted" {
		if err := validateOutcome(outcome, detector.ID, added); err != nil {
			fmt.Fprintf(stderr, "error: unvalidated collection changes: %v\n", err)
			return 1, false
		}
	}
	if outcome.Result == "accepted" {
		if err := validateCorpus(detector.ID, added, before, after); err != nil {
			fmt.Fprintf(stderr, "error: unvalidated collection changes: %v\n", err)
			return 1, false
		}
	}
	detector.Iterations++
	detector.State = stateInProgress
	detector.Examples = append(detector.Examples, corpusExampleFiles(added)...)
	detector.Queries = append(detector.Queries, outcome.Queries...)
	detector.Candidates = append(detector.Candidates, outcome.Candidates...)
	detector.Rejections = append(detector.Rejections, outcome.Rejections...)
	if len(detector.Examples) >= detector.Target {
		detector.State = stateComplete
	}
	if detector.Iterations >= maxIterations && detector.State != stateComplete {
		detector.State = stateBlocked
	}
	if err := writeProgress(progressPath, p); err != nil {
		fmt.Fprintf(stderr, "error: checkpoint collection progress: %v\n", err)
		return 1, false
	}
	if commit {
		if err := commitCheckpoint(root, progressPath, detector.ID, detector.Iterations, outcome.Result, added); err != nil {
			fmt.Fprintf(stderr, "error: collection commit failed after a valid checkpoint: %v\n", err)
			fmt.Fprintln(stderr, "validated corpus and progress remain as uncommitted collection state. Commit them manually, discard the iteration deliberately, or rerun without --commit after reviewing Git status.")
			return 1, false
		}
	}
	return 0, true
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
	var complete, blocked, excluded, active int
	for _, d := range p.Detectors {
		switch d.State {
		case stateComplete:
			complete++
		case stateBlocked:
			blocked++
		case stateExcluded:
			excluded++
		default:
			active++
		}
	}
	fmt.Fprintf(stdout, "collection summary: %d complete, %d blocked, %d excluded, %d remaining\n", complete, blocked, excluded, active)
	if blocked > 0 {
		return 2
	}
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
	if p.Version != 1 || p.DetectorID != detectorID || p.Provider != "github" || p.Repository == "" ||
		p.RepositoryURL == "" || p.Commit == "" || p.OriginalPath == "" || p.Permalink == "" ||
		p.RetrievedAt == "" || p.SHA256 == "" || p.LicenseURL == "" || p.ProjectKind == "" ||
		len(p.VariationTags) == 0 || p.Rationale == "" {
		return errors.New("missing required provenance")
	}
	if !approvedLicenses[p.License] {
		return fmt.Errorf("license %q is not approved", p.License)
	}
	if filepath.IsAbs(p.OriginalPath) || strings.HasPrefix(filepath.Clean(p.OriginalPath), "..") {
		return errors.New("original path is unsafe")
	}
	if !strings.Contains(p.Permalink, "/blob/"+p.Commit+"/") {
		return errors.New("permalink is not pinned to immutable commit")
	}
	if len(p.SHA256) != 64 {
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
	lower := strings.ToLower(content)
	return strings.Contains(lower, "password=") || strings.Contains(lower, "authorization:") ||
		strings.Contains(lower, "private_key") || strings.Contains(lower, "ghp_") || strings.Contains(lower, "npm_")
}

func selectDetector(detectors []DetectorProgress, target string) *DetectorProgress {
	for i := range detectors {
		if detectors[i].ID == target && target != "" && isEligible(detectors[i]) {
			return &detectors[i]
		}
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
	if p.Version != 1 || len(p.Detectors) == 0 {
		return Progress{}, errors.New("invalid collection progress schema")
	}
	for _, detector := range p.Detectors {
		if detector.ID == "" || !validState(detector.State) || detector.Iterations < 0 || detector.Iterations > maxIterations ||
			(detector.Target != 0 && (detector.Target < 3 || detector.Target > 5)) {
			return Progress{}, errors.New("invalid detector progress")
		}
	}
	return p, nil
}

func validState(state string) bool {
	return state == statePending || state == stateInProgress || state == stateComplete || state == stateBlocked || state == stateExcluded
}

func writeProgress(path string, p Progress) error {
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
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func snapshot(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root || entry.IsDir() {
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
		return fmt.Errorf("agent outcome must be accepted")
	}
	declared := append([]string(nil), outcome.Added...)
	sort.Strings(declared)
	if len(declared) != len(added) {
		return fmt.Errorf("agent protocol does not match the added files")
	}
	for i := range added {
		if strings.TrimPrefix(added[i], "testdata/corpus/"+detectorID+"/") == declared[i] {
			continue
		}
		return fmt.Errorf("agent protocol does not match the added files")
	}
	hasProvenance := false
	for _, path := range declared {
		if filepath.Base(path) == "provenance.yaml" {
			hasProvenance = true
		}
	}
	if !hasProvenance {
		return fmt.Errorf("agent added no provenance record")
	}
	if len(corpusExampleFiles(added)) == 0 {
		return fmt.Errorf("agent added no corpus example")
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
