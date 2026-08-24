package analyzerloop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileJournalUsesPrivatePermissionsAndJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run", "journal.jsonl")
	journal, err := NewFileJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Record(JournalEntry{At: time.Unix(0, 0), WorkItem: 1, Role: RoleImplementer, Attempt: 1, Outcome: "accepted"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("journal mode = %o, want 600", info.Mode().Perm())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"outcome":"accepted"`) || !strings.HasSuffix(string(content), "\n") {
		t.Fatalf("unexpected journal content %q", content)
	}
}
