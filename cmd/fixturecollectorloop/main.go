// fixturecollectorloop collects authentic dependency-source corpus examples.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	statePending    = "pending"
	stateInProgress = "in-progress"
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
}

type Iteration struct {
	DetectorID string
	CorpusDir  string
	Iteration  int
}

type Outcome struct {
	Result string
	Added  []string
}

type Agent interface {
	Run(Iteration) (Outcome, error)
}

type unavailableAgent struct{}

func (unavailableAgent) Run(Iteration) (Outcome, error) {
	return Outcome{}, errors.New("no Codex agent is configured; inject an agent through the command seam")
}

func main() { os.Exit(run(os.Args[1:], ".", os.Stdout, os.Stderr, unavailableAgent{})) }

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
	var detectors commaList
	fs.Var(&detectors, "detector", "detector identifier (repeatable or comma-separated)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if len(detectors) == 0 {
		fmt.Fprintln(stderr, "error: at least one --detector is required for this tracer bullet")
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
		p.Detectors[i] = DetectorProgress{ID: id, State: statePending, Examples: []string{}}
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
	if err := fs.Parse(args); err != nil {
		return 1
	}
	p, err := readProgress(*progressPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: read collection progress: %v\n", err)
		return 1
	}
	detector := selectDetector(p.Detectors, *target)
	if detector == nil {
		fmt.Fprintln(stdout, "collection complete: no eligible detector")
		return 0
	}
	before, err := snapshot(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: snapshot collection state: %v\n", err)
		return 1
	}
	corpusDir := filepath.Join(root, "testdata", "corpus", detector.ID)
	outcome, err := agent.Run(Iteration{DetectorID: detector.ID, CorpusDir: corpusDir, Iteration: detector.Iterations + 1})
	if err != nil {
		fmt.Fprintf(stderr, "error: collection agent: %v\n", err)
		return 1
	}
	after, err := snapshot(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: snapshot collection state: %v\n", err)
		return 1
	}
	added, err := validateDelta(before, after, filepath.Join("testdata", "corpus", detector.ID))
	if err != nil {
		fmt.Fprintf(stderr, "error: unvalidated collection changes: %v\n", err)
		return 1
	}
	if len(added) == 0 {
		fmt.Fprintln(stderr, "error: unvalidated collection changes: agent added no corpus example")
		return 1
	}
	if err := validateOutcome(outcome, detector.ID, added); err != nil {
		fmt.Fprintf(stderr, "error: unvalidated collection changes: %v\n", err)
		return 1
	}
	detector.Iterations++
	detector.State = stateInProgress
	detector.Examples = append(detector.Examples, corpusExampleFiles(added)...)
	if err := writeProgress(*progressPath, p); err != nil {
		fmt.Fprintf(stderr, "error: checkpoint collection progress: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "checkpoint: %s iteration %d (%d new files)\n", detector.ID, detector.Iterations, len(added))
	return 0
}

func selectDetector(detectors []DetectorProgress, target string) *DetectorProgress {
	for i := range detectors {
		if detectors[i].ID == target && target != "" {
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
		if detector.ID == "" || (detector.State != statePending && detector.State != stateInProgress) {
			return Progress{}, errors.New("invalid detector progress")
		}
	}
	return p, nil
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
