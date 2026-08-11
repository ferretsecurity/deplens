package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestComposedResearcherMaterializesThreeSelectedCandidatesFromIDsOnly(t *testing.T) {
	iteration := Iteration{DetectorID: "example-detector", CorpusDir: filepath.Join(t.TempDir(), "corpus"), Iteration: 1}
	candidates := []SourceCandidate{
		candidate("github", "Owner/one", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "go.mod", "module one\n"),
		candidate("github", "Owner/two", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "nested/go.mod", "module two\n"),
		candidate("github", "Owner/three", "cccccccccccccccccccccccccccccccccccccccc", "go.mod", "module three\n"),
	}
	acquisition := fakeAcquisition{input: ResearchInput{Candidates: candidates}}
	selector := fakeSelector{result: ResearchResult{Selection: Selection{Selected: []SelectedCandidate{
		{ID: candidates[2].ID, Rationale: "nested module"},
		{ID: candidates[0].ID, Rationale: "simple module"},
		{ID: candidates[1].ID, Rationale: "second module"},
	}}}}

	result, err := newComposedResearcher(&acquisition, &selector).Research(context.Background(), iteration)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome.Result != "accepted" || len(result.Accepted) != 3 {
		t.Fatalf("result = %+v", result)
	}
	var packet struct {
		Candidates []struct {
			ID string `json:"id"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(selector.packet, &packet); err != nil || len(packet.Candidates) != 3 {
		t.Fatalf("packet = %s, err = %v", selector.packet, err)
	}
	for _, accepted := range result.Accepted {
		if _, err := os.Stat(filepath.Join(iteration.CorpusDir, accepted.Directory, "provenance.yaml")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestComposedResearcherPreservesSelectedSurvivorsWhenPeerFailsFinalValidation(t *testing.T) {
	iteration := Iteration{DetectorID: "example-detector", CorpusDir: filepath.Join(t.TempDir(), "corpus"), Iteration: 1}
	good := candidate("github", "Owner/good", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "go.mod", "module good\n")
	goodTwo := candidate("github", "Owner/good-two", "cccccccccccccccccccccccccccccccccccccccc", "go.mod", "module good two\n")
	bad := candidate("github", "Owner/bad", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "go.mod", "module bad\n")
	if err := os.MkdirAll(filepath.Join(iteration.CorpusDir, bad.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	selector := fakeSelector{result: ResearchResult{Selection: Selection{Selected: []SelectedCandidate{
		{ID: good.ID, Rationale: "valid survivor"}, {ID: goodTwo.ID, Rationale: "second survivor"}, {ID: bad.ID, Rationale: "bad peer"},
	}}}}

	result, err := newComposedResearcher(&fakeAcquisition{input: ResearchInput{Candidates: []SourceCandidate{good, goodTwo, bad}}}, &selector).Research(context.Background(), iteration)
	if err != nil {
		t.Fatal(err)
	}
	if !selector.called || result.Outcome.Result != "accepted" || len(result.Accepted) != 2 || result.Accepted[0].Candidate.ID != good.ID {
		t.Fatalf("result = %+v", result)
	}
	if !sameStrings(result.Outcome.Rejections, []string{"final-validation-duplicate-identity"}) {
		t.Fatalf("rejections = %v", result.Outcome.Rejections)
	}
}

func TestComposedResearcherPreservesSurvivorsWhenSelectedPeerDuplicatesSourceContent(t *testing.T) {
	iteration := Iteration{DetectorID: "example-detector", CorpusDir: filepath.Join(t.TempDir(), "corpus"), Iteration: 1}
	first := candidate("github", "Owner/first", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "go.mod", "module shared\n")
	duplicate := candidate("github", "Owner/duplicate", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "go.mod", "module shared\n")
	third := candidate("github", "Owner/third", "cccccccccccccccccccccccccccccccccccccccc", "go.mod", "module distinct\n")
	selector := fakeSelector{result: ResearchResult{Selection: Selection{Selected: []SelectedCandidate{
		{ID: first.ID, Rationale: "first source"},
		{ID: duplicate.ID, Rationale: "duplicate source"},
		{ID: third.ID, Rationale: "distinct source"},
	}}}}

	result, err := newComposedResearcher(&fakeAcquisition{input: ResearchInput{Candidates: []SourceCandidate{first, duplicate, third}}}, &selector).Research(context.Background(), iteration)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome.Result != "accepted" || len(result.Accepted) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if !sameStrings(result.Outcome.Rejections, []string{"final-validation-duplicate-content"}) {
		t.Fatalf("rejections = %v", result.Outcome.Rejections)
	}
}

func TestComposedResearcherRejectsContentAlreadyInCorpus(t *testing.T) {
	iteration := Iteration{DetectorID: "example-detector", CorpusDir: filepath.Join(t.TempDir(), "corpus"), Iteration: 1}
	existing := candidate("github", "Owner/existing", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "go.mod", "module shared\n")
	existingTwo := candidate("github", "Owner/existing-two", "cccccccccccccccccccccccccccccccccccccccc", "go.mod", "module existing two\n")
	existingThree := candidate("github", "Owner/existing-three", "dddddddddddddddddddddddddddddddddddddddd", "go.mod", "module existing three\n")
	duplicate := candidate("github", "Owner/duplicate", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "go.mod", "module shared\n")
	survivor := candidate("github", "Owner/survivor", "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", "go.mod", "module survivor\n")
	survivorTwo := candidate("github", "Owner/survivor-two", "ffffffffffffffffffffffffffffffffffffffff", "go.mod", "module survivor two\n")

	firstSelector := fakeSelector{result: ResearchResult{Selection: Selection{Selected: []SelectedCandidate{
		{ID: existing.ID, Rationale: "existing source"},
		{ID: existingTwo.ID, Rationale: "existing source two"},
		{ID: existingThree.ID, Rationale: "existing source three"},
	}}}}
	if _, err := newComposedResearcher(&fakeAcquisition{input: ResearchInput{Candidates: []SourceCandidate{existing, existingTwo, existingThree}}}, &firstSelector).Research(context.Background(), iteration); err != nil {
		t.Fatal(err)
	}

	selector := fakeSelector{result: ResearchResult{Selection: Selection{Selected: []SelectedCandidate{
		{ID: duplicate.ID, Rationale: "duplicate source"},
		{ID: survivor.ID, Rationale: "surviving source"},
		{ID: survivorTwo.ID, Rationale: "second surviving source"},
	}}}}
	result, err := newComposedResearcher(&fakeAcquisition{input: ResearchInput{Candidates: []SourceCandidate{duplicate, survivor, survivorTwo}}}, &selector).Research(context.Background(), iteration)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome.Result != "accepted" || len(result.Accepted) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if !sameStrings(result.Outcome.Rejections, []string{"final-validation-duplicate-content"}) {
		t.Fatalf("rejections = %v", result.Outcome.Rejections)
	}
}

func TestTargetedCommandCollectsQualifiedBatchWithoutSelectorWorkspaceWrites(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "collection.yaml")
	if got := run([]string{"initialize-progress", "--progress", progress, "--detector", "example-detector"}, root, os.Stdout, os.Stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize = %d", got)
	}
	initializeGitRepository(t, root)
	commitGitChanges(t, root)
	candidates := []SourceCandidate{
		candidate("github", "Owner/one", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "go.mod", "module one\n"),
		candidate("github", "Owner/two", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "nested/go.mod", "module two\n"),
		candidate("github", "Owner/three", "cccccccccccccccccccccccccccccccccccccccc", "go.mod", "module three\n"),
	}
	selector := fakeSelector{result: ResearchResult{Selection: Selection{Selected: []SelectedCandidate{
		{ID: candidates[1].ID, Rationale: "nested"}, {ID: candidates[2].ID, Rationale: "third"}, {ID: candidates[0].ID, Rationale: "first"},
	}}}}
	researcher := newComposedResearcher(&fakeAcquisition{input: ResearchInput{Candidates: candidates}}, &selector)
	if got := run([]string{"run", "--single", "--progress", progress}, root, os.Stdout, os.Stderr, researcher); got != 0 {
		t.Fatalf("run = %d", got)
	}
	p, err := readProgress(progress)
	if err != nil {
		t.Fatal(err)
	}
	if p.Detectors[0].State != stateComplete || len(p.Detectors[0].Examples) != 3 || len(p.Detectors[0].History) != 1 || len(p.Detectors[0].History[0].AcceptedIDs) != 3 {
		t.Fatalf("progress = %+v", p.Detectors[0])
	}
	if _, err := os.Stat(filepath.Join(root, "selection-wrote-here")); !os.IsNotExist(err) {
		t.Fatalf("selector wrote workspace: %v", err)
	}
}

func candidate(provider, repository, commit, path, source string) SourceCandidate {
	sourceHash := sha256.Sum256([]byte(source))
	license := []byte("MIT License\n")
	licenseHash := sha256.Sum256(license)
	c := SourceCandidate{Provider: provider, Repository: repository, RepositoryURL: "https://github.com/" + repository, DefaultBranch: "main", Commit: commit, OriginalPath: path, Source: []byte(source), SourceSHA256: fmtHash(sourceHash), RetrievedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339), License: LicenseEvidence{SPDX: "MIT", Path: "LICENSE", Permalink: "https://github.com/" + repository + "/blob/" + commit + "/LICENSE", SHA256: fmtHash(licenseHash), Bytes: license}}
	c.ID = stableCandidateID(c.Provider, c.Repository, c.Commit, c.OriginalPath)
	return c
}

func fmtHash(sum [sha256.Size]byte) string { return fmt.Sprintf("%x", sum) }

func TestComposedResearcherReplacesAcquisitionAndSelectorIndependently(t *testing.T) {
	acquisition := fakeAcquisition{input: ResearchInput{SelectionPacket: []byte(`{"candidates":["candidate-1"]}`)}}
	selector := fakeSelector{result: ResearchResult{Outcome: Outcome{Result: "unsuccessful"}}}
	researcher := newComposedResearcher(&acquisition, &selector)
	iteration := Iteration{DetectorID: "example-detector", Iteration: 1}

	result, err := researcher.Research(context.Background(), iteration)
	if err != nil {
		t.Fatal(err)
	}
	if !acquisition.called || !selector.called {
		t.Fatalf("acquisition called = %t, selector called = %t", acquisition.called, selector.called)
	}
	if selector.packet == nil || result.Outcome.Result != "unsuccessful" {
		t.Fatalf("selector packet = %q, result = %+v", selector.packet, result)
	}
}

func TestSelectionPacketPacksReferencesAndCandidatesDeterministically(t *testing.T) {
	first := candidate("github", "owner/first", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "go.mod", "module first\n")
	second := candidate("github", "owner/second", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "go.mod", "module second\n")
	third := candidate("github", "owner/third", "cccccccccccccccccccccccccccccccccccccccc", "go.mod", "module third\n")
	first.DiscoveringQuery, second.DiscoveringQuery, third.DiscoveringQuery = "one", "two", "one"
	ref := AcceptedCorpusReference{Candidate: candidate("github", "owner/ref", "dddddddddddddddddddddddddddddddddddddddd", "go.mod", "module reference\n"), Rationale: "already accepted"}

	packet, err := buildSelectionPacket(SelectionPacketOptions{
		Candidates:         []SourceCandidate{third, first, second},
		AcceptedReferences: []AcceptedCorpusReference{ref},
		QueryPlan:          []string{"one", "two"},
		PacketTokens:       10000,
		HeadroomBytes:      128,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.OmittedIDs) != 0 || len(packet.Candidates) != 3 {
		t.Fatalf("packet candidates = %v, omitted = %v", packet.Candidates, packet.OmittedIDs)
	}
	if packet.Candidates[0].ID > packet.Candidates[1].ID || packet.Candidates[1].ID > packet.Candidates[2].ID {
		t.Fatalf("serialized candidates are not stable-ID sorted: %#v", packet.Candidates)
	}
	var decoded struct {
		AcceptedCorpus []struct {
			Mandatory bool   `json:"mandatory"`
			Source    string `json:"source_untrusted_data"`
		} `json:"accepted_corpus_references"`
		Candidates []struct {
			Source string `json:"source_untrusted_data"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(packet.Bytes, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.AcceptedCorpus) != 1 || !decoded.AcceptedCorpus[0].Mandatory || decoded.AcceptedCorpus[0].Source != "module reference\n" || len(decoded.Candidates) != 3 {
		t.Fatalf("decoded packet = %#v", decoded)
	}
}

func TestSelectionValidationUsesPacketMembershipAndPartialBounds(t *testing.T) {
	selection := Selection{Selected: []SelectedCandidate{{ID: "candidate-a", Rationale: "useful\nvariation"}}}
	if err := validateSelection(selection, map[string]struct{}{"candidate-a": {}}, 2); err != nil {
		t.Fatalf("partial selection rejected: %v", err)
	}
	if err := validateSelection(selection, map[string]struct{}{"candidate-a": {}}, 0); err == nil {
		t.Fatal("fresh one-candidate selection was accepted")
	}
	if err := validateSelection(Selection{Selected: []SelectedCandidate{{ID: "candidate-a", Rationale: "one"}, {ID: "candidate-a", Rationale: "two"}, {ID: "candidate-b", Rationale: "three"}}}, map[string]struct{}{"candidate-a": {}, "candidate-b": {}}, 0); err == nil {
		t.Fatal("duplicate selection ID was accepted")
	}
	if err := validateSelection(Selection{Selected: []SelectedCandidate{{ID: "candidate-a", Rationale: "bad\x01control"}, {ID: "candidate-b", Rationale: "two"}, {ID: "candidate-c", Rationale: "three"}}}, map[string]struct{}{"candidate-a": {}, "candidate-b": {}, "candidate-c": {}}, 0); err == nil {
		t.Fatal("forbidden rationale control was accepted")
	}
}

type fakeAcquisition struct {
	called bool
	input  ResearchInput
}

func (f *fakeAcquisition) Acquire(_ context.Context, _ Iteration) (ResearchInput, error) {
	f.called = true
	return f.input, nil
}

type fakeSelector struct {
	called bool
	packet []byte
	result ResearchResult
}

func (f *fakeSelector) Select(_ context.Context, _ Iteration, packet []byte) (ResearchResult, error) {
	f.called = true
	f.packet = packet
	return f.result, nil
}
