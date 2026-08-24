package analyzerloop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanCreatesSortedEligibleLedger(t *testing.T) {
	root := t.TempDir()
	corpus := filepath.Join(root, "corpus")
	writeTestFile(t, filepath.Join(corpus, "testdata", "corpus", "zulu", "candidate-z", "zulu.lock"), "zulu")
	writeTestFile(t, filepath.Join(corpus, "testdata", "corpus", "alpha", "candidate-a", "alpha.lock"), "alpha")
	writeTestFile(t, filepath.Join(corpus, "testdata", "corpus", "ignored", "candidate-i", "ignored.lock"), "ignored")

	verification := `version: 1
corpus:
  path: testdata/corpus
deplens:
  commit: test-commit
  rules_sha256: rules-hash
work_items:
  - id: zulu
    detector: {id: zulu, form: lockfile, roles: [resolution]}
    result: OK
    candidates:
      - candidate_id: candidate-z
        original_path: zulu.lock
        source_sha256: ` + sha256Text("zulu") + `
        verdict: valid
      - candidate_id: candidate-z
        original_path: zulu.lock
        source_sha256: ` + sha256Text("zulu") + `
        verdict: valid
      - candidate_id: candidate-z
        original_path: zulu.lock
        source_sha256: ` + sha256Text("zulu") + `
        verdict: valid
  - id: alpha
    detector: {id: alpha, form: manifest, roles: [declaration]}
    result: OK
    candidates:
      - candidate_id: candidate-a
        original_path: alpha.lock
        source_sha256: ` + sha256Text("alpha") + `
        verdict: valid
      - candidate_id: candidate-a
        original_path: alpha.lock
        source_sha256: ` + sha256Text("alpha") + `
        verdict: valid
      - candidate_id: candidate-a
        original_path: alpha.lock
        source_sha256: ` + sha256Text("alpha") + `
        verdict: valid
  - id: ignored
    detector: {id: ignored, form: manifest, roles: [declaration]}
    result: ISSUES
    candidates:
      - candidate_id: candidate-i
        original_path: ignored.lock
        source_sha256: ` + sha256Text("ignored") + `
        verdict: valid
`
	writeTestFile(t, filepath.Join(corpus, ".deplens", "corpus-verification.yaml"), verification)

	ledgerPath := filepath.Join(root, "ledger.yaml")
	ledger, err := Plan(PlanOptions{CorpusRoot: corpus, VerificationPath: filepath.Join(corpus, ".deplens", "corpus-verification.yaml"), LedgerPath: ledgerPath, DeplensCommit: "test-commit", RulesSHA256: "rules-hash"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(ledger.WorkItems) != 2 || ledger.WorkItems[0].ID != "alpha" || ledger.WorkItems[1].Number != 2 {
		t.Fatalf("unexpected planned items: %#v", ledger.WorkItems)
	}
	if ledger.WorkItems[0].State != StatePending || len(ledger.WorkItems[0].Checkpoints) != 0 {
		t.Fatalf("unexpected initial item: %#v", ledger.WorkItems[0])
	}
	if _, err := os.Stat(ledgerPath); err != nil {
		t.Fatalf("ledger was not written: %v", err)
	}
}

func TestPlanRefusesExistingLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.yaml")
	writeTestFile(t, path, "already here")
	_, err := Plan(PlanOptions{LedgerPath: path})
	if err == nil {
		t.Fatal("Plan() succeeded with an existing ledger")
	}
}

func TestParseSelectionNormalizesRanges(t *testing.T) {
	got, err := ParseSelection("3,1...2,3", 3)
	if err != nil {
		t.Fatalf("ParseSelection() error = %v", err)
	}
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("selection = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("selection = %v, want %v", got, want)
		}
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
