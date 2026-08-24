// Package analyzerloop contains the durable planning and execution boundaries
// for semantic-analyzer implementation work.
package analyzerloop

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	StatePending    = "pending"
	StateInProgress = "in_progress"
	StateCompleted  = "completed"
)

// PlanOptions identifies immutable inputs and the destination ledger.
type PlanOptions struct {
	CorpusRoot       string
	VerificationPath string
	LedgerPath       string
	DeplensCommit    string
	RulesSHA256      string
	CorpusCommit     string
}

// Ledger is the reviewed, durable work list consumed by the runner.
type Ledger struct {
	Version   int          `yaml:"version"`
	Corpus    CorpusStamp  `yaml:"corpus"`
	Deplens   DeplensStamp `yaml:"deplens"`
	WorkItems []WorkItem   `yaml:"work_items"`
}

type CorpusStamp struct {
	Path   string `yaml:"path"`
	Commit string `yaml:"commit,omitempty"`
}

type DeplensStamp struct {
	Commit      string `yaml:"commit"`
	RulesSHA256 string `yaml:"rules_sha256"`
}

// WorkItem is independently completable. Checkpoints are append-only
// accepted results; transient failures live only in the runtime journal.
type WorkItem struct {
	Number      int          `yaml:"number"`
	ID          string       `yaml:"id"`
	Detector    Detector     `yaml:"detector"`
	Candidates  []Candidate  `yaml:"candidates"`
	State       string       `yaml:"state"`
	Checkpoints []Checkpoint `yaml:"checkpoints,omitempty"`
}

type Detector struct {
	ID    string   `yaml:"id"`
	Form  string   `yaml:"form"`
	Roles []string `yaml:"roles"`
}

// Candidate identifies an original corpus source without copying its content.
type Candidate struct {
	ID           string `yaml:"candidate_id"`
	OriginalPath string `yaml:"original_path"`
	SourceSHA256 string `yaml:"source_sha256"`
}

type Checkpoint struct {
	Role         string   `yaml:"role"`
	Attempt      int      `yaml:"attempt"`
	Commit       string   `yaml:"commit,omitempty"`
	Fixtures     []string `yaml:"fixtures"`
	ChangedPaths []string `yaml:"changed_paths"`
}

type verificationLedger struct {
	Corpus struct {
		Path string `yaml:"path"`
	} `yaml:"corpus"`
	Deplens struct {
		Commit      string `yaml:"commit"`
		RulesSHA256 string `yaml:"rules_sha256"`
	} `yaml:"deplens"`
	WorkItems []verificationItem `yaml:"work_items"`
}

type verificationItem struct {
	ID         string      `yaml:"id"`
	Detector   Detector    `yaml:"detector"`
	Candidates []candidate `yaml:"candidates"`
	Result     string      `yaml:"result"`
}

type candidate struct {
	ID           string `yaml:"candidate_id"`
	OriginalPath string `yaml:"original_path"`
	SourceSHA256 string `yaml:"source_sha256"`
	Verdict      string `yaml:"verdict"`
}

// Plan creates a new ledger from verified corpus candidates. It refuses to
// overwrite existing work because committing the generated ledger is approval
// for a later run.
func Plan(options PlanOptions) (Ledger, error) {
	if options.LedgerPath == "" {
		return Ledger{}, errors.New("ledger path is required")
	}
	if _, err := os.Stat(options.LedgerPath); err == nil {
		return Ledger{}, fmt.Errorf("refusing to overwrite existing ledger %q", options.LedgerPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Ledger{}, fmt.Errorf("stat ledger %q: %w", options.LedgerPath, err)
	}
	if options.VerificationPath == "" {
		return Ledger{}, errors.New("corpus verification path is required")
	}
	data, err := os.ReadFile(options.VerificationPath)
	if err != nil {
		return Ledger{}, fmt.Errorf("read corpus verification ledger: %w", err)
	}
	var verified verificationLedger
	if err := yaml.Unmarshal(data, &verified); err != nil {
		return Ledger{}, fmt.Errorf("parse corpus verification ledger: %w", err)
	}
	if options.DeplensCommit != "" && verified.Deplens.Commit != options.DeplensCommit {
		return Ledger{}, fmt.Errorf("corpus targets deplens commit %q, current commit is %q", verified.Deplens.Commit, options.DeplensCommit)
	}
	if options.RulesSHA256 != "" && verified.Deplens.RulesSHA256 != options.RulesSHA256 {
		return Ledger{}, fmt.Errorf("corpus rules hash %q does not match current default rules %q", verified.Deplens.RulesSHA256, options.RulesSHA256)
	}

	corpusPath := verified.Corpus.Path
	if corpusPath == "" {
		return Ledger{}, errors.New("corpus verification ledger has no corpus path")
	}
	if !filepath.IsAbs(corpusPath) {
		corpusPath = filepath.Join(options.CorpusRoot, corpusPath)
	}
	items := make([]WorkItem, 0, len(verified.WorkItems))
	for _, item := range verified.WorkItems {
		if !eligible(item) {
			continue
		}
		candidates := make([]Candidate, 0, len(item.Candidates))
		for _, source := range item.Candidates {
			if err := verifyCandidate(corpusPath, item.ID, source); err != nil {
				return Ledger{}, err
			}
			candidates = append(candidates, Candidate{ID: source.ID, OriginalPath: source.OriginalPath, SourceSHA256: source.SourceSHA256})
		}
		items = append(items, WorkItem{ID: item.ID, Detector: item.Detector, Candidates: candidates, State: StatePending})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	for index := range items {
		items[index].Number = index + 1
	}
	ledger := Ledger{
		Version:   1,
		Corpus:    CorpusStamp{Path: corpusPath, Commit: options.CorpusCommit},
		Deplens:   DeplensStamp{Commit: verified.Deplens.Commit, RulesSHA256: verified.Deplens.RulesSHA256},
		WorkItems: items,
	}
	if len(items) == 0 {
		return ledger, nil
	}
	if err := CreateLedger(options.LedgerPath, ledger); err != nil {
		return Ledger{}, err
	}
	return ledger, nil
}

func eligible(item verificationItem) bool {
	if item.ID == "" || item.Result != "OK" || len(item.Candidates) != 3 {
		return false
	}
	for _, candidate := range item.Candidates {
		if candidate.Verdict != "valid" || candidate.ID == "" || candidate.OriginalPath == "" || candidate.SourceSHA256 == "" {
			return false
		}
	}
	return true
}

func verifyCandidate(corpusPath, detectorID string, candidate candidate) error {
	if filepath.IsAbs(candidate.OriginalPath) || escapes(candidate.OriginalPath) {
		return fmt.Errorf("detector %q candidate %q has unsafe original path %q", detectorID, candidate.ID, candidate.OriginalPath)
	}
	path := filepath.Join(corpusPath, detectorID, candidate.ID, candidate.OriginalPath)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read corpus source %q: %w", path, err)
	}
	actual := sha256Text(string(content))
	if actual != candidate.SourceSHA256 {
		return fmt.Errorf("corpus source %q hash mismatch", path)
	}
	return nil
}

func escapes(path string) bool {
	for _, part := range strings.FieldsFunc(filepath.ToSlash(path), func(r rune) bool { return r == '/' }) {
		if part == ".." {
			return true
		}
	}
	return false
}

func sha256Text(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum)
}

func LoadLedger(path string) (Ledger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Ledger{}, fmt.Errorf("read analyzer ledger: %w", err)
	}
	var ledger Ledger
	if err := yaml.Unmarshal(data, &ledger); err != nil {
		return Ledger{}, fmt.Errorf("parse analyzer ledger: %w", err)
	}
	if ledger.Version != 1 {
		return Ledger{}, fmt.Errorf("unsupported analyzer ledger version %d", ledger.Version)
	}
	return ledger, nil
}

func SaveLedger(path string, ledger Ledger) error {
	data, err := yaml.Marshal(ledger)
	if err != nil {
		return fmt.Errorf("encode analyzer ledger: %w", err)
	}
	return writeLedger(path, data, false)
}

// CreateLedger publishes a complete new ledger without ever replacing an
// existing approval record, even when planners race.
func CreateLedger(path string, ledger Ledger) error {
	data, err := yaml.Marshal(ledger)
	if err != nil {
		return fmt.Errorf("encode analyzer ledger: %w", err)
	}
	return writeLedger(path, data, true)
}

func writeLedger(path string, data []byte, noReplace bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create ledger directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".analyzer-implementation-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary ledger: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary ledger: %w", err)
	}
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return fmt.Errorf("set ledger permissions: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary ledger: %w", err)
	}
	if noReplace {
		if err := os.Link(temporaryPath, path); err != nil {
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("refusing to overwrite existing ledger %q", path)
			}
			return fmt.Errorf("create ledger atomically: %w", err)
		}
		return nil
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace ledger atomically: %w", err)
	}
	return nil
}

// ParseSelection accepts comma-separated numbers and inclusive ranges using
// the documented 1,3...7,12 syntax. The returned selection is unique and
// ascending.
func ParseSelection(value string, maximum int) ([]int, error) {
	if maximum < 1 {
		return nil, errors.New("ledger has no work items")
	}
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("selection is required")
	}
	selected := map[int]bool{}
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			return nil, errors.New("selection contains an empty item")
		}
		bounds := strings.Split(token, "...")
		if len(bounds) > 2 {
			return nil, fmt.Errorf("invalid selection range %q", token)
		}
		first, err := parseSelectionNumber(bounds[0], maximum)
		if err != nil {
			return nil, err
		}
		last := first
		if len(bounds) == 2 {
			last, err = parseSelectionNumber(bounds[1], maximum)
			if err != nil {
				return nil, err
			}
			if first > last {
				return nil, fmt.Errorf("selection range %q is descending", token)
			}
		}
		for number := first; number <= last; number++ {
			selected[number] = true
		}
	}
	result := make([]int, 0, len(selected))
	for number := range selected {
		result = append(result, number)
	}
	sort.Ints(result)
	return result, nil
}

func parseSelectionNumber(value string, maximum int) (int, error) {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || number < 1 || number > maximum {
		return 0, fmt.Errorf("selection number %q is outside 1...%d", value, maximum)
	}
	return number, nil
}
