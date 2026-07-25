// Command server is the entry point for the Software Quality Platform.
// It wires the configuration, authentication, data, and UI layers
// together and starts the HTTP dashboard.
package main

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/ryokotmng/software-quality-platform/internal/auth"
	"github.com/ryokotmng/software-quality-platform/internal/config"
	"github.com/ryokotmng/software-quality-platform/internal/store"
	"github.com/ryokotmng/software-quality-platform/internal/web"
)

func main() {
	cfg := config.Load()

	authSvc, err := auth.New(filepath.Join(cfg.DataDir, "users.json"))
	if err != nil {
		log.Fatalf("init auth: %v", err)
	}
	// Seed the single administrative account so the trigger endpoint is
	// usable out of the box (FR-6).
	if err := authSvc.EnsureUser(cfg.AdminUsername, cfg.AdminPassword); err != nil {
		log.Fatalf("seed admin user: %v", err)
	}

	runs, err := store.NewRunStore(filepath.Join(cfg.DataDir, "runs"))
	if err != nil {
		log.Fatalf("init run store: %v", err)
	}

	srv := web.NewServer(cfg, authSvc, runs)

	log.Printf("Software Quality Platform (%s) listening on %s", cfg.AppEnv, cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, srv); err != nil {
		log.Fatalf("server: %v", err)
	}
}
