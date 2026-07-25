# Software Quality Platform

Course project repository for the UoPeople Software Engineering program.
The platform helps small software teams integrate automated testing and
continuous integration behind a single dashboard.

It is written in **Go** using **only the standard library** — no external
runtime dependencies — so it builds and runs anywhere the Go 1.26+
toolchain is available.

## Repository Structure

| Path         | Purpose                                                        |
|--------------|----------------------------------------------------------------|
| `/cmd`       | Application entry points (`cmd/server` runs the dashboard)     |
| `/internal`  | Core business logic, isolated per layer (see below)            |
| `/tests`     | Cross-module integration tests                                 |
| `/docs`      | Technical documentation and design notes                       |
| `/diagrams`  | Architecture and design diagrams                               |
| `/reports`   | Progress summaries and final writeups                          |

### `/internal` layers

| Package                  | Layer                | Responsibility                                   |
|--------------------------|----------------------|--------------------------------------------------|
| `internal/web`           | User Interface       | HTTP dashboard + protected trigger endpoint      |
| `internal/auth`          | Application Services | Registration & authentication (PBKDF2)           |
| `internal/orchestration` | Application Services | Runs `go test -json` and structures the results  |
| `internal/store`         | Data                 | Persists test-run results (JSON files)           |
| `internal/config`        | Cross-cutting        | Environment / `.env` configuration               |

## Running locally

```sh
cp .env.example .env          # optional; sensible defaults work without it
go run ./cmd/server           # serves the dashboard on http://localhost:8080
```

Trigger a test run (only authenticated users may — FR-6):

```sh
curl -u admin:changeme -X POST http://localhost:8080/api/runs/trigger
```

Then open <http://localhost:8080> to see the latest status, run history,
and failing-test logs.

### Core-logic endpoints

```sh
# Authenticate — returns 200 on success, 401 on failure.
curl -X POST http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"changeme"}'

# Report — total / passed / failed / pass rate for a fixed sample.
curl http://localhost:8080/report
```

## Testing & CI

```sh
go test ./...
```

On every push and pull request, GitHub Actions (`.github/workflows/ci.yml`)
builds the platform, runs `go vet`, and executes the test suites — the
same orchestration the dashboard triggers, run automatically.

## Branching Model

- `main` — stable, approved version.
- `development` — active branch for ongoing changes.
- `feature/*` — short-lived branches for individual pieces of work.

All changes flow through `development` and are merged into `main` only
after review, so the mainline stays stable and every update is traceable.
