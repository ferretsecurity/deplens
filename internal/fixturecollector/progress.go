// Package fixturecollector contains the repository-owned contracts used by the
// fixturecollectorloop development command. It deliberately contains no code
// for searching or executing upstream projects.
package fixturecollector

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const ProgressSchemaVersion = 1

type Detector struct {
	ID             string   `yaml:"id"`
	Form           string   `yaml:"form"`
	Roles          []string `yaml:"roles"`
	FilenameRegex  string   `yaml:"filename-regex,omitempty"`
	PathGlob       string   `yaml:"path-glob,omitempty"`
	Analyzer       string   `yaml:"analyzer,omitempty"`
	AnalyzerConfig string   `yaml:"analyzer-configuration,omitempty"`
	Capabilities   []string `yaml:"capabilities"`
}

func (d Detector) ExtractsReferences() bool { return slices.Contains(d.Capabilities, "extract") }

type DetectorState string

const (
	StatePending    DetectorState = "pending"
	StateInProgress DetectorState = "in-progress"
	StateComplete   DetectorState = "complete"
	StateBlocked    DetectorState = "blocked"
	StateExcluded   DetectorState = "excluded"
)

type Settings struct {
	MinExamples             int      `yaml:"min-examples"`
	MaxExamples             int      `yaml:"max-examples"`
	MaxSearchQueries        int      `yaml:"max-search-queries"`
	MaxCandidateInspections int      `yaml:"max-candidate-inspections"`
	MaxExampleBytes         int64    `yaml:"max-example-bytes"`
	AllowedLicenses         []string `yaml:"allowed-licenses"`
}

type DetectorProgress struct {
	ID                  string        `yaml:"id"`
	State               DetectorState `yaml:"state"`
	IterationLimit      int           `yaml:"iteration-limit"`
	CompletedIterations int           `yaml:"completed-iterations,omitempty"`
	ExclusionReason     string        `yaml:"exclusion-reason,omitempty"`
	BlockedReason       string        `yaml:"blocked-reason,omitempty"`
}

type Progress struct {
	SchemaVersion        int                `yaml:"schema-version"`
	InventoryFingerprint string             `yaml:"inventory-fingerprint"`
	Settings             Settings           `yaml:"settings"`
	Detectors            []DetectorProgress `yaml:"detectors"`
}

func NewProgress(inventory []Detector) (Progress, error) {
	fingerprint, err := InventoryFingerprint(inventory)
	if err != nil {
		return Progress{}, err
	}
	p := Progress{
		SchemaVersion:        ProgressSchemaVersion,
		InventoryFingerprint: fingerprint,
		Settings: Settings{MinExamples: 3, MaxExamples: 5, MaxSearchQueries: 8, MaxCandidateInspections: 40, MaxExampleBytes: 2 << 20,
			AllowedLicenses: []string{"MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "0BSD", "CC0-1.0", "Unlicense"}},
		Detectors: make([]DetectorProgress, 0, len(inventory)),
	}
	for _, detector := range inventory {
		if !detector.ExtractsReferences() {
			p.Detectors = append(p.Detectors, DetectorProgress{ID: detector.ID, State: StatePending, IterationLimit: 7})
		}
	}
	return p, p.Validate()
}

func InventoryFingerprint(inventory []Detector) (string, error) {
	copy := append([]Detector(nil), inventory...)
	for i := range copy {
		if strings.TrimSpace(copy[i].ID) == "" {
			return "", fmt.Errorf("inventory detector %d: id is required", i)
		}
		copy[i].Roles = append([]string(nil), copy[i].Roles...)
		copy[i].Capabilities = append([]string(nil), copy[i].Capabilities...)
		slices.Sort(copy[i].Roles)
		slices.Sort(copy[i].Capabilities)
	}
	b, err := yaml.Marshal(copy)
	if err != nil {
		return "", fmt.Errorf("encode detector inventory: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func ParseProgress(data []byte) (Progress, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var p Progress
	if err := decoder.Decode(&p); err != nil {
		return Progress{}, fmt.Errorf("parse collection progress: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Progress{}, fmt.Errorf("parse collection progress: multiple YAML documents are not supported")
		}
		return Progress{}, fmt.Errorf("parse collection progress: %w", err)
	}
	if err := p.Validate(); err != nil {
		return Progress{}, err
	}
	return p, nil
}

func (p Progress) MarshalYAML() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return yaml.Marshal(p)
}

func (p Progress) Validate() error {
	if p.SchemaVersion != ProgressSchemaVersion {
		return fmt.Errorf("collection progress: schema-version must be %d", ProgressSchemaVersion)
	}
	if len(p.InventoryFingerprint) != 64 {
		return fmt.Errorf("collection progress: inventory-fingerprint must be a SHA-256 hex digest")
	}
	if _, err := hex.DecodeString(p.InventoryFingerprint); err != nil {
		return fmt.Errorf("collection progress: inventory-fingerprint must be a SHA-256 hex digest")
	}
	if p.Settings.MinExamples < 3 || p.Settings.MaxExamples > 5 || p.Settings.MinExamples > p.Settings.MaxExamples {
		return fmt.Errorf("collection progress: example-set bounds must be between three and five")
	}
	if p.Settings.MaxSearchQueries <= 0 || p.Settings.MaxCandidateInspections <= 0 || p.Settings.MaxExampleBytes <= 0 {
		return fmt.Errorf("collection progress: settings limits must be positive")
	}
	if len(p.Settings.AllowedLicenses) == 0 {
		return fmt.Errorf("collection progress: settings.allowed-licenses is required")
	}
	seen := map[string]bool{}
	for i, d := range p.Detectors {
		where := fmt.Sprintf("collection progress: detectors[%d]", i)
		if d.ID == "" || seen[d.ID] {
			return fmt.Errorf("%s: id must be unique and non-empty", where)
		}
		seen[d.ID] = true
		if d.IterationLimit < 1 || d.IterationLimit > 7 {
			return fmt.Errorf("%s: iteration-limit must be between 1 and 7", where)
		}
		if d.CompletedIterations < 0 || d.CompletedIterations > d.IterationLimit {
			return fmt.Errorf("%s: completed-iterations is outside its limit", where)
		}
		switch d.State {
		case StatePending, StateInProgress, StateComplete, StateBlocked, StateExcluded:
		default:
			return fmt.Errorf("%s: invalid state %q", where, d.State)
		}
		if d.State == StatePending && d.CompletedIterations != 0 {
			return fmt.Errorf("%s: pending detectors cannot have completed iterations", where)
		}
		if d.State == StateInProgress && d.CompletedIterations == d.IterationLimit {
			return fmt.Errorf("%s: in-progress detectors cannot exhaust their iteration limit", where)
		}
		if d.State == StateComplete && d.CompletedIterations == 0 {
			return fmt.Errorf("%s: complete detectors require a completed iteration", where)
		}
		if d.State == StateExcluded && d.ExclusionReason == "" {
			return fmt.Errorf("%s: excluded detectors require exclusion-reason", where)
		}
		if d.State != StateExcluded && d.ExclusionReason != "" {
			return fmt.Errorf("%s: exclusion-reason is only valid for excluded detectors", where)
		}
		if d.State == StateBlocked && d.BlockedReason == "" {
			return fmt.Errorf("%s: blocked detectors require blocked-reason", where)
		}
		if d.State != StateBlocked && d.BlockedReason != "" {
			return fmt.Errorf("%s: blocked-reason is only valid for blocked detectors", where)
		}
	}
	return nil
}

// ValidateInventory refuses to continue a reviewed collection plan after a
// semantic detector change. Reconciliation is deliberately a human action.
func (p Progress) ValidateInventory(inventory []Detector) error {
	if err := p.Validate(); err != nil {
		return err
	}
	fingerprint, err := InventoryFingerprint(inventory)
	if err != nil {
		return err
	}
	if p.InventoryFingerprint != fingerprint {
		return fmt.Errorf("collection progress: detector inventory fingerprint does not match the reviewed plan; initialize and review a new progress document")
	}
	expected := make(map[string]struct{}, len(inventory))
	for _, detector := range inventory {
		if !detector.ExtractsReferences() {
			expected[detector.ID] = struct{}{}
		}
	}
	if len(p.Detectors) != len(expected) {
		return fmt.Errorf("collection progress: detector coverage does not match the default detector capabilities")
	}
	for _, detector := range p.Detectors {
		if _, found := expected[detector.ID]; !found {
			return fmt.Errorf("collection progress: detector coverage does not match the default detector capabilities")
		}
	}
	return nil
}

func (p Progress) SelectDetector(id string) *DetectorProgress {
	if id != "" {
		for i := range p.Detectors {
			if p.Detectors[i].ID == id && selectable(p.Detectors[i]) {
				return &p.Detectors[i]
			}
		}
		return nil
	}
	for i := range p.Detectors {
		if p.Detectors[i].State == StateInProgress && selectable(p.Detectors[i]) {
			return &p.Detectors[i]
		}
	}
	for i := range p.Detectors {
		if p.Detectors[i].State == StatePending && selectable(p.Detectors[i]) {
			return &p.Detectors[i]
		}
	}
	return nil
}

func selectable(d DetectorProgress) bool {
	return d.CompletedIterations < d.IterationLimit && (d.State == StatePending || d.State == StateInProgress)
}
