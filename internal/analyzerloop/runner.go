package analyzerloop

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Role string

const (
	RoleImplementer Role = "implementer"
	RoleVerifier    Role = "verifier"
)

// Attempt is the intentionally small boundary shared with a fresh agent
// session. The executor receives no previous-agent transcript.
type Attempt struct {
	WorkItem WorkItem
	Role     Role
	Number   int
	Limit    int
}

type AttemptResult struct {
	Summary      string
	Fixtures     []string
	ChangedPaths []string
}

// Executor starts one fresh implementation or verification attempt.
type Executor interface {
	Execute(context.Context, Attempt) (AttemptResult, error)
}

type Journal interface {
	Record(JournalEntry) error
}

type JournalEntry struct {
	At       time.Time `json:"at"`
	WorkItem int       `json:"work_item"`
	Role     Role      `json:"role"`
	Attempt  int       `json:"attempt"`
	Outcome  string    `json:"outcome"`
	Message  string    `json:"message,omitempty"`
}

// Committer records each accepted checkpoint on the dedicated target branch.
// A commit failure is fatal because the persisted ledger can no longer be
// treated as an atomic checkpoint transaction.
type Committer interface {
	Commit(context.Context, WorkItem, Checkpoint) error
}

// MemoryJournal is useful for deterministic tests and callers that want no
// persistent runtime record.
type MemoryJournal struct {
	Entries []JournalEntry
}

func (j *MemoryJournal) Record(entry JournalEntry) error {
	j.Entries = append(j.Entries, entry)
	return nil
}

// Runner advances selected independent items. Failed attempts never mutate the
// reviewed ledger; accepted checkpoints are persisted before the next stage.
type Runner struct {
	LedgerPath          string
	Executor            Executor
	Journal             Journal
	Committer           Committer
	Progress            ProgressReporter
	ImplementerAttempts int
	VerifierAttempts    int
}

func (r Runner) Run(ctx context.Context, selection []int) error {
	if r.LedgerPath == "" || r.Executor == nil || r.Journal == nil {
		return errors.New("runner requires ledger path, executor, and journal")
	}
	ledger, err := LoadLedger(r.LedgerPath)
	if err != nil {
		return err
	}
	if len(selection) == 0 {
		return errors.New("run selection is empty")
	}
	for _, number := range selection {
		if number < 1 || number > len(ledger.WorkItems) {
			return fmt.Errorf("work item %d is not in the ledger", number)
		}
		item := &ledger.WorkItems[number-1]
		if item.Number != number {
			return fmt.Errorf("ledger item %d has inconsistent number %d", number, item.Number)
		}
		if item.State == StateCompleted {
			continue
		}
		if item.State != StatePending && item.State != StateInProgress {
			return fmt.Errorf("work item %d has invalid state %q", number, item.State)
		}
		report(r.Progress, func(progress ProgressReporter) { progress.WorkItemStarted(*item) })
		if item.State == StatePending {
			result, ok, err := r.accept(ctx, *item, RoleImplementer, r.implementerLimit())
			if err != nil {
				return err
			}
			if !ok {
				report(r.Progress, func(progress ProgressReporter) { progress.WorkItemFinished(*item, false) })
				continue
			}
			checkpoint := checkpoint(RoleImplementer, len(item.Checkpoints)+1, result)
			item.Checkpoints = append(item.Checkpoints, checkpoint)
			item.State = StateInProgress
			if err := SaveLedger(r.LedgerPath, ledger); err != nil {
				return err
			}
			if r.Committer != nil {
				if err := r.Committer.Commit(ctx, *item, checkpoint); err != nil {
					return fmt.Errorf("commit implementation checkpoint for work item %d: %w", item.Number, err)
				}
			}
		}
		result, ok, err := r.accept(ctx, *item, RoleVerifier, r.verifierLimit())
		if err != nil {
			return err
		}
		if !ok {
			report(r.Progress, func(progress ProgressReporter) { progress.WorkItemFinished(*item, false) })
			continue
		}
		checkpoint := checkpoint(RoleVerifier, len(item.Checkpoints)+1, result)
		item.Checkpoints = append(item.Checkpoints, checkpoint)
		item.State = StateCompleted
		if err := SaveLedger(r.LedgerPath, ledger); err != nil {
			return err
		}
		if r.Committer != nil {
			if err := r.Committer.Commit(ctx, *item, checkpoint); err != nil {
				return fmt.Errorf("commit verification checkpoint for work item %d: %w", item.Number, err)
			}
		}
		report(r.Progress, func(progress ProgressReporter) { progress.WorkItemFinished(*item, true) })
	}
	return nil
}

func (r Runner) accept(ctx context.Context, item WorkItem, role Role, limit int) (AttemptResult, bool, error) {
	for attempt := 1; attempt <= limit; attempt++ {
		current := Attempt{WorkItem: item, Role: role, Number: attempt, Limit: limit}
		started := time.Now()
		report(r.Progress, func(progress ProgressReporter) { progress.AttemptStarted(current) })
		result, err := r.Executor.Execute(ctx, current)
		if err == nil {
			err = validateAttemptResult(result)
		}
		report(r.Progress, func(progress ProgressReporter) { progress.AttemptFinished(current, result, err, time.Since(started)) })
		entry := JournalEntry{At: time.Now().UTC(), WorkItem: item.Number, Role: role, Attempt: attempt}
		if err != nil {
			entry.Outcome, entry.Message = "rejected", err.Error()
			if journalErr := r.Journal.Record(entry); journalErr != nil {
				return AttemptResult{}, false, fmt.Errorf("record rejected attempt: %w", journalErr)
			}
			if ctx.Err() != nil {
				return AttemptResult{}, false, ctx.Err()
			}
			continue
		}
		entry.Outcome, entry.Message = "accepted", result.Summary
		if journalErr := r.Journal.Record(entry); journalErr != nil {
			return AttemptResult{}, false, fmt.Errorf("record accepted attempt: %w", journalErr)
		}
		return result, true, nil
	}
	return AttemptResult{}, false, nil
}

func (r Runner) implementerLimit() int {
	if r.ImplementerAttempts > 0 {
		return r.ImplementerAttempts
	}
	return 2
}

func (r Runner) verifierLimit() int {
	if r.VerifierAttempts > 0 {
		return r.VerifierAttempts
	}
	return 2
}

func validateAttemptResult(result AttemptResult) error {
	if result.Summary == "" {
		return errors.New("attempt returned no summary")
	}
	if len(result.Fixtures) != 3 {
		return fmt.Errorf("attempt recorded %d fixtures, want exactly 3", len(result.Fixtures))
	}
	seen := map[string]bool{}
	for _, fixture := range result.Fixtures {
		first, _, _ := strings.Cut(filepath.ToSlash(fixture), "/")
		if fixture == "" || filepath.IsAbs(fixture) || escapes(fixture) || !slices.Contains([]string{"testdata"}, first) {
			return fmt.Errorf("attempt recorded unsafe fixture path %q", fixture)
		}
		if seen[fixture] {
			return fmt.Errorf("attempt recorded duplicate fixture %q", fixture)
		}
		seen[fixture] = true
	}
	return nil
}

func checkpoint(role Role, attempt int, result AttemptResult) Checkpoint {
	return Checkpoint{
		Role:         string(role),
		Attempt:      attempt,
		Fixtures:     append([]string(nil), result.Fixtures...),
		ChangedPaths: append([]string(nil), result.ChangedPaths...),
	}
}
