package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ryokotmng/software-quality-platform/internal/auth"
	"github.com/ryokotmng/software-quality-platform/internal/config"
	"github.com/ryokotmng/software-quality-platform/internal/store"
)

// benchServer builds a Server backed by a temporary data directory with a
// single seeded user, for use in latency/throughput benchmarks.
func benchServer(b *testing.B) *Server {
	b.Helper()
	dir := b.TempDir()
	authSvc, err := auth.New(filepath.Join(dir, "users.json"))
	if err != nil {
		b.Fatalf("auth.New: %v", err)
	}
	if err := authSvc.EnsureUser("admin", "changeme"); err != nil {
		b.Fatalf("seed user: %v", err)
	}
	runs, err := store.NewRunStore(filepath.Join(dir, "runs"))
	if err != nil {
		b.Fatalf("store.NewRunStore: %v", err)
	}
	return NewServer(config.Config{RepoDir: "."}, authSvc, runs)
}

// BenchmarkLogin measures the latency of POST /login. This path is
// intentionally dominated by PBKDF2 password verification, so its
// ns/op reflects the deliberate security cost, not HTTP overhead.
func BenchmarkLogin(b *testing.B) {
	srv := benchServer(b)
	body := []byte(`{"username":"admin","password":"changeme"}`)
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d, want 200", rec.Code)
		}
	}
}

// BenchmarkReport measures the latency of GET /report, which performs
// only in-memory summarization and JSON encoding.
func BenchmarkReport(b *testing.B) {
	srv := benchServer(b)
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/report", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d, want 200", rec.Code)
		}
	}
}
