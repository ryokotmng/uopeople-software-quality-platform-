// Package tests holds integration tests that exercise more than one
// internal package together. Unit tests live next to the code they
// cover (internal/.../*_test.go); this directory is for cross-module
// checks, per the project's /tests convention.
package tests

import (
	"testing"
	"time"

	"github.com/ryokotmng/software-quality-platform/internal/orchestration"
	"github.com/ryokotmng/software-quality-platform/internal/store"
)

// TestRunStoreRoundTrip verifies that a result produced by the
// orchestration layer survives a save/list cycle through the data layer
// unchanged, and that runs come back most-recent-first.
func TestRunStoreRoundTrip(t *testing.T) {
	s, err := store.NewRunStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewRunStore: %v", err)
	}

	older := orchestration.RunResult{
		StartedAt: time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC),
		Total:     2, Passed: 2,
	}
	newer := orchestration.RunResult{
		StartedAt:  time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC),
		DurationMS: 42,
		Total:      3, Passed: 2, Failed: 1,
		Failures: []orchestration.SuiteFailure{{Package: "p", Test: "TestX", Output: "boom"}},
	}
	for _, r := range []orchestration.RunResult{older, newer} {
		if err := s.Save(r); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	runs, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2", len(runs))
	}
	if !runs[0].StartedAt.Equal(newer.StartedAt) {
		t.Errorf("first run = %v, want newest %v", runs[0].StartedAt, newer.StartedAt)
	}
	if runs[0].Failed != 1 || len(runs[0].Failures) != 1 || runs[0].Failures[0].Output != "boom" {
		t.Errorf("failure detail not preserved: %+v", runs[0])
	}
}
