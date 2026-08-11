package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// SourceCandidate is immutable factual evidence acquired by Go. Selector
// adapters receive only its JSON packet representation, never this structure.
type SourceCandidate struct {
	ID            string
	Provider      string
	Repository    string
	RepositoryURL string
	DefaultBranch string
	Commit        string
	OriginalPath  string
	RetrievedAt   string
	Source        []byte
	SourceSHA256  string
	License       LicenseEvidence
}

type LicenseEvidence struct {
	SPDX      string
	Path      string
	Permalink string
	SHA256    string
	Bytes     []byte
}

// Selection is the entire model-controlled protocol. It deliberately has no
// paths, hashes, or other factual fields.
type Selection struct {
	Selected []SelectedCandidate `json:"selected"`
}

type SelectedCandidate struct {
	ID        string `json:"id"`
	Rationale string `json:"rationale"`
}

type AcceptedCandidate struct {
	Candidate SourceCandidate
	Directory string
	Rationale string
}

type selectionPacketCandidate struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Path       string `json:"path"`
	Commit     string `json:"commit"`
	ByteLength int    `json:"byte_length"`
	SHA256     string `json:"sha256"`
	License    string `json:"license"`
	Source     string `json:"source_untrusted_data"`
}

type selectionPacketData struct {
	Candidates []selectionPacketCandidate `json:"candidates"`
}

func stableCandidateID(provider, repository, commit, originalPath string) string {
	identity := strings.ToLower(strings.TrimSpace(provider)) + "\x00" +
		strings.ToLower(strings.TrimSpace(repository)) + "\x00" +
		strings.ToLower(strings.TrimSpace(commit)) + "\x00" +
		filepath.ToSlash(strings.TrimPrefix(filepath.Clean(originalPath), "./"))
	sum := sha256.Sum256([]byte(identity))
	return "candidate-" + hex.EncodeToString(sum[:16])
}

func selectionPacket(candidates []SourceCandidate) ([]byte, error) {
	sorted := append([]SourceCandidate(nil), candidates...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	packet := selectionPacketData{Candidates: make([]selectionPacketCandidate, 0, len(sorted))}
	for _, c := range sorted {
		packet.Candidates = append(packet.Candidates, selectionPacketCandidate{
			ID:         c.ID,
			Repository: c.Repository,
			Path:       c.OriginalPath,
			Commit:     c.Commit,
			ByteLength: len(c.Source),
			SHA256:     c.SourceSHA256,
			License:    c.License.SPDX,
			Source:     string(c.Source),
		})
	}
	return json.Marshal(packet)
}

func validateCandidate(c SourceCandidate) error {
	if c.ID != stableCandidateID(c.Provider, c.Repository, c.Commit, c.OriginalPath) {
		return errors.New("candidate ID does not match immutable identity")
	}
	if c.Provider != "github" || c.Repository == "" || c.Commit == "" || c.RepositoryURL == "" || c.DefaultBranch == "" || c.RetrievedAt == "" {
		return errors.New("candidate lacks immutable acquisition facts")
	}
	if filepath.IsAbs(c.OriginalPath) || strings.HasPrefix(filepath.Clean(c.OriginalPath), "..") || !utf8.Valid(c.Source) || len(c.Source) == 0 || len(c.Source) > maxExampleSize || isLFSPointer(string(c.Source)) || containsUnsafeContent(string(c.Source)) {
		return errors.New("candidate source is not qualified")
	}
	if hash(string(c.Source)) != strings.ToLower(c.SourceSHA256) {
		return errors.New("candidate source hash does not match exact bytes")
	}
	if !approvedLicenses[c.License.SPDX] || c.License.Path == "" || !utf8.Valid(c.License.Bytes) || hash(string(c.License.Bytes)) != strings.ToLower(c.License.SHA256) || !strings.Contains(c.License.Permalink, "/blob/"+c.Commit+"/") {
		return errors.New("candidate license evidence is invalid")
	}
	return nil
}

func materializeCandidates(corpusDir, detectorID string, candidates []SourceCandidate, selection Selection) ([]AcceptedCandidate, error) {
	byID := make(map[string]SourceCandidate, len(candidates))
	for _, c := range candidates {
		if err := validateCandidate(c); err != nil {
			return nil, err
		}
		if _, exists := byID[c.ID]; exists {
			return nil, errors.New("duplicate candidate ID")
		}
		byID[c.ID] = c
	}
	selected := append([]SelectedCandidate(nil), selection.Selected...)
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	if len(selected) < 3 || len(selected) > 5 {
		return nil, errors.New("fresh selection must contain three through five candidates")
	}
	accepted := make([]AcceptedCandidate, 0, len(selected))
	for _, chosen := range selected {
		c, ok := byID[chosen.ID]
		if !ok {
			return nil, fmt.Errorf("selector chose unknown candidate %q", chosen.ID)
		}
		if chosen.Rationale == "" || len([]rune(chosen.Rationale)) > 1000 || !utf8.ValidString(chosen.Rationale) || strings.ContainsAny(chosen.Rationale, "\x00\r") {
			return nil, errors.New("selector rationale is invalid")
		}
		directory := c.ID
		root := filepath.Join(corpusDir, directory)
		if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("example directory already exists: %s", directory)
		}
		sourcePath := filepath.Join(root, filepath.FromSlash(c.OriginalPath))
		if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(sourcePath, c.Source, 0o644); err != nil {
			return nil, err
		}
		p := provenanceV2From(c, detectorID, chosen.Rationale)
		contents, err := yaml.Marshal(p)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(root, "provenance.yaml"), contents, 0o644); err != nil {
			return nil, err
		}
		accepted = append(accepted, AcceptedCandidate{Candidate: c, Directory: directory, Rationale: chosen.Rationale})
	}
	return accepted, nil
}

// validateAcceptedCandidates is deliberately separate from materialization so
// the wrapper rechecks Go's own writes against immutable acquisition evidence.
func validateAcceptedCandidates(accepted []AcceptedCandidate, detectorID string, after map[string]string) error {
	for _, item := range accepted {
		if err := validateCandidate(item.Candidate); err != nil {
			return err
		}
		root := "testdata/corpus/" + detectorID + "/" + item.Directory
		sourcePath := root + "/" + filepath.ToSlash(item.Candidate.OriginalPath)
		if after[sourcePath] != string(item.Candidate.Source) {
			return fmt.Errorf("%s does not match acquired exact source bytes", sourcePath)
		}
		p, err := parseProvenance(after[root+"/provenance.yaml"])
		if err != nil {
			return err
		}
		if err := validateProvenance(p, detectorID); err != nil {
			return err
		}
		if p.CandidateID != item.Candidate.ID || p.SHA256 != item.Candidate.SourceSHA256 || p.License.SHA256 != item.Candidate.License.SHA256 || p.License.SPDX != item.Candidate.License.SPDX || p.Rationale != item.Rationale {
			return errors.New("provenance does not match Go-owned acquisition facts")
		}
	}
	return nil
}
