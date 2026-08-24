package analyzerloop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileJournal appends content-free attempt outcomes under the ignored runtime
// directory. It intentionally does not retain corpus source text.
type FileJournal struct {
	path string
	mu   sync.Mutex
}

func NewFileJournal(path string) (*FileJournal, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create runtime journal directory: %w", err)
	}
	return &FileJournal{path: path}, nil
}

func (j *FileJournal) Record(entry JournalEntry) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	file, err := os.OpenFile(j.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open runtime journal: %w", err)
	}
	defer file.Close()
	encoded, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode runtime journal entry: %w", err)
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("append runtime journal entry: %w", err)
	}
	return nil
}
