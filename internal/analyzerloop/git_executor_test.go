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

func TestAgentEventExtractsFollowDetails(t *testing.T) {
	message, command := agentEvent([]byte(`{"type":"item.completed","item":{"type":"agent_message","text":"checking fixture"}}`))
	if message != "checking fixture" || command != (agentCommand{}) {
		t.Fatalf("agent message = (%q, %#v)", message, command)
	}
	message, command = agentEvent([]byte(`{"type":"item.completed","item":{"type":"command_execution","command":"go test ./...","exit_code":1}}`))
	if message != "" || command.Command != "go test ./..." || command.ExitCode != 1 {
		t.Fatalf("agent command = (%q, %#v)", message, command)
	}
}
