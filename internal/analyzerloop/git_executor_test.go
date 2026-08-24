package analyzerloop

import "testing"

func TestAllowedChangedPathsRejectsHarnessAndCorpusChanges(t *testing.T) {
	if err := allowedChangedPaths([]string{"internal/analyze/demo.go", "testdata/demo/fixture", "README.md"}); err != nil {
		t.Fatalf("allowed paths rejected: %v", err)
	}
	if err := allowedChangedPaths([]string{"cmd/analyzerloop/main.go"}); err == nil {
		t.Fatal("harness change was allowed")
	}
	if err := allowedChangedPaths([]string{"testdata/corpus/demo/original"}); err == nil {
		t.Fatal("original corpus copy was allowed")
	}
}

func TestRecognizedCandidateRejectsUnsupportedExtraction(t *testing.T) {
	unsupported := []byte(`{"sources":[{"detector":"demo","analysis":{"presence":"present","extraction":"unsupported"}}]}`)
	if recognizedCandidate(unsupported, "demo") {
		t.Fatal("unsupported source was accepted")
	}
	complete := []byte(`{"sources":[{"detector":"demo","analysis":{"presence":"present","extraction":"complete"}}]}`)
	if !recognizedCandidate(complete, "demo") {
		t.Fatal("complete source was rejected")
	}
}
