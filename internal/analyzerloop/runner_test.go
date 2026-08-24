package analyzerloop

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRunnerRequiresImplementerThenVerifierBeforeCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.yaml")
	ledger := Ledger{Version: 1, WorkItems: []WorkItem{{Number: 1, ID: "demo", State: StatePending}}}
	if err := SaveLedger(path, ledger); err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{results: []AttemptResult{
		{Summary: "implementation", Fixtures: []string{"testdata/demo/a", "testdata/demo/b", "testdata/demo/c"}},
		{Summary: "verification", Fixtures: []string{"testdata/demo/a", "testdata/demo/b", "testdata/demo/c"}},
	}}
	runner := Runner{LedgerPath: path, Executor: executor, Journal: &MemoryJournal{}}
	if err := runner.Run(context.Background(), []int{1}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	updated, err := LoadLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	item := updated.WorkItems[0]
	if item.State != StateCompleted || len(item.Checkpoints) != 2 {
		t.Fatalf("unexpected completed work item: %#v", item)
	}
	if executor.attempts[0].Role != RoleImplementer || executor.attempts[1].Role != RoleVerifier {
		t.Fatalf("unexpected attempts: %#v", executor.attempts)
	}
}

func TestRunnerLeavesItemPendingWhenImplementerFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.yaml")
	if err := SaveLedger(path, Ledger{Version: 1, WorkItems: []WorkItem{{Number: 1, ID: "demo", State: StatePending}}}); err != nil {
		t.Fatal(err)
	}
	runner := Runner{LedgerPath: path, Executor: &fakeExecutor{err: context.DeadlineExceeded}, Journal: &MemoryJournal{}}
	if err := runner.Run(context.Background(), []int{1}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	updated, err := LoadLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	if updated.WorkItems[0].State != StatePending || len(updated.WorkItems[0].Checkpoints) != 0 {
		t.Fatalf("failure was persisted in the ledger: %#v", updated.WorkItems[0])
	}
}

type fakeExecutor struct {
	results  []AttemptResult
	attempts []Attempt
	err      error
}

func (e *fakeExecutor) Execute(_ context.Context, attempt Attempt) (AttemptResult, error) {
	e.attempts = append(e.attempts, attempt)
	if e.err != nil {
		return AttemptResult{}, e.err
	}
	result := e.results[0]
	e.results = e.results[1:]
	return result, nil
}
