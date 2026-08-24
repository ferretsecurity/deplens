package analyzerloop

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// GitCommitter stages only the accepted patch and reviewed ledger.
type GitCommitter struct {
	RepositoryRoot string
	LedgerPath     string
	RunID          string
}

func (c GitCommitter) Commit(ctx context.Context, item WorkItem, checkpoint Checkpoint) error {
	paths := append([]string(nil), checkpoint.ChangedPaths...)
	ledgerPath, err := filepath.Rel(c.RepositoryRoot, c.LedgerPath)
	if err != nil {
		return fmt.Errorf("resolve ledger path for commit: %w", err)
	}
	paths = append(paths, ledgerPath)
	args := append([]string{"add", "--"}, paths...)
	if output, err := git(ctx, c.RepositoryRoot, args...); err != nil {
		return fmt.Errorf("stage checkpoint paths: %w: %s", err, strings.TrimSpace(string(output)))
	}
	message := fmt.Sprintf("Implement analyzer %s\n\nRalph-Run: %s\nRalph-Attempt: %d\nRalph-Work-Item: %d\nRalph-Outcome: %s", item.ID, c.RunID, checkpoint.Attempt, item.Number, checkpoint.Role)
	if output, err := git(ctx, c.RepositoryRoot, "commit", "-m", message); err != nil {
		return fmt.Errorf("commit checkpoint: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// PreflightRun refuses unsafe or ambiguous repositories before an agent gets a
// worktree. It never attempts repair or cleanup.
func PreflightRun(ctx context.Context, repositoryRoot, corpusRoot string) error {
	status, err := git(ctx, repositoryRoot, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("read target worktree status: %w", err)
	}
	if strings.TrimSpace(string(status)) != "" {
		return errors.New("target worktree must be clean")
	}
	branch, err := git(ctx, repositoryRoot, "branch", "--show-current")
	if err != nil {
		return fmt.Errorf("read target branch: %w", err)
	}
	name := strings.TrimSpace(string(branch))
	if name == "" || name == "main" || name == "master" {
		return fmt.Errorf("run requires a dedicated non-default branch, found %q", name)
	}
	if output, err := git(ctx, repositoryRoot, "var", "GIT_AUTHOR_IDENT"); err != nil || strings.TrimSpace(string(output)) == "" {
		return errors.New("Git author identity is not configured")
	}
	corpusStatus, err := git(ctx, corpusRoot, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("read corpus worktree status: %w", err)
	}
	if strings.TrimSpace(string(corpusStatus)) != "" {
		return errors.New("corpus worktree must be clean")
	}
	return nil
}
