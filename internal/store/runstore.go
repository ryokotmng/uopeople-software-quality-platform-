// Package store persists test-run results (the data layer). The initial
// implementation writes one JSON file per run under a local directory.
//
// The rest of the platform depends only on this small API, so the file
// store can later be replaced by a database (see config.DatabaseURL)
// without changing the orchestration, reporting, or UI layers.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ryokotmng/software-quality-platform/internal/orchestration"
)

// RunStore persists RunResults as JSON files in a directory.
type RunStore struct {
	dir string
	mu  sync.Mutex
}

// NewRunStore returns a store writing to dir, creating it if needed.
func NewRunStore(dir string) (*RunStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &RunStore{dir: dir}, nil
}

// Save writes a single run result to its own JSON file.
func (s *RunStore) Save(r orchestration.RunResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	name := fmt.Sprintf("run-%s.json", slug(r.StartedAt.Format("20060102T150405.000")))
	tmp := filepath.Join(s.dir, name+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, name))
}

// List returns all persisted runs, most recent first.
func (s *RunStore) List() ([]orchestration.RunResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var runs []orchestration.RunResult
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "run-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue // a single unreadable file must not break the list
		}
		var r orchestration.RunResult
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		runs = append(runs, r)
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	return runs, nil
}

func slug(s string) string {
	return strings.NewReplacer(":", "", ".", "-").Replace(s)
}
