// Package config loads the platform's runtime configuration from
// environment variables, optionally seeded from a local .env file.
//
// Keeping configuration in the environment (never in the source tree)
// gives a reproducible, secure baseline for a small team: the same
// binary runs in development, CI, and production purely by changing
// environment values. The loader uses only the standard library, so the
// platform runs anywhere Go 1.26+ is available (NFR-6).
package config

import (
	"bufio"
	"os"
	"strings"
)

// Config holds all runtime settings. Zero values fall back to
// development-friendly defaults so `go run ./cmd/server` works with no
// setup at all.
type Config struct {
	AppEnv string // development | ci | production

	// DataDir holds runtime state: the user store and persisted test-run
	// results. It is a local directory in the initial implementation.
	DataDir string

	// DatabaseURL is reserved for a future shared metrics store
	// (PostgreSQL). The initial implementation persists results as JSON
	// files under DataDir, so this is read but not yet required. Because
	// the data layer is accessed only through the store package, swapping
	// the file store for a database will not touch the other layers.
	DatabaseURL string

	Addr    string // listen address, e.g. ":8080"
	RepoDir string // repository the orchestrator runs tests against

	// Seed credentials for the single administrative account. Only
	// authenticated users may trigger test runs (FR-6).
	AdminUsername string
	AdminPassword string
}

// Load reads .env (if present) into the environment, then builds a
// Config. Real environment variables always take precedence over .env.
func Load() Config {
	loadDotEnv(".env")
	return Config{
		AppEnv:        getenv("APP_ENV", "development"),
		DataDir:       getenv("DATA_DIR", "data"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		Addr:          listenAddr(),
		RepoDir:       getenv("REPO_DIR", "."),
		AdminUsername: getenv("ADMIN_USERNAME", "admin"),
		AdminPassword: getenv("ADMIN_PASSWORD", "changeme"),
	}
}

// listenAddr resolves the HTTP listen address. PORT (a bare port number,
// as injected by many container and PaaS platforms) takes precedence;
// otherwise ADDR is used, defaulting to ":8080".
func listenAddr() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return getenv("ADDR", ":8080")
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv parses a simple KEY=VALUE file and sets any variables that
// are not already present in the environment.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no .env is fine
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}
