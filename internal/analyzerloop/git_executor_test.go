package analyzerloop

import (
	"strings"
	"testing"
)

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

func TestRecognizedCandidateRequiresCompleteExtractionWithDependencies(t *testing.T) {
	unsupported := []byte(`{"sources":[{"detector":"demo","analysis":{"presence":"present","extraction":"unsupported"}}]}`)
	if recognizedCandidate(unsupported, "demo") {
		t.Fatal("unsupported source was accepted")
	}
	completeWithoutDependencies := []byte(`{"sources":[{"detector":"demo","analysis":{"presence":"present","extraction":"complete"}}]}`)
	if recognizedCandidate(completeWithoutDependencies, "demo") {
		t.Fatal("dependency-free source was accepted")
	}
	complete := []byte(`{"sources":[{"detector":"demo","analysis":{"presence":"present","extraction":"complete"},"dependencies":[{"name":"example"}]}]}`)
	if !recognizedCandidate(complete, "demo") {
		t.Fatal("complete source was rejected")
	}
}

func TestParseEngineResultReadsCodexAgentMessage(t *testing.T) {
	output := []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"<analyzerloop-result>{\"summary\":\"done\",\"fixtures\":[\"testdata/a\",\"testdata/b\",\"testdata/c\"]}</analyzerloop-result>"}}`)
	result, err := parseEngineResult(output)
	if err != nil {
		t.Fatalf("parseEngineResult: %v", err)
	}
	if result.Summary != "done" || len(result.Fixtures) != 3 {
		t.Fatalf("result = %#v", result)
	}
}

func TestPromptSeparatesImplementerAndVerifierResponsibilities(t *testing.T) {
	workItem := WorkItem{Number: 1, ID: "demo"}
	implementer := prompt("/corpus", Attempt{WorkItem: workItem, Role: RoleImplementer})
	if !strings.Contains(implementer, "Implement a real source analyzer") || !strings.Contains(implementer, "Create exactly three") {
		t.Fatalf("implementer prompt missing extraction requirements: %q", implementer)
	}
	verifier := prompt("/corpus", Attempt{WorkItem: workItem, Role: RoleVerifier})
	if !strings.Contains(verifier, "Do not add, remove, or replace fixtures") || strings.Contains(verifier, "Create exactly three") {
		t.Fatalf("verifier prompt has incorrect fixture instructions: %q", verifier)
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
