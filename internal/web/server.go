// Package web is the User Interface layer: a small net/http server that
// exposes the foundational dashboard and a protected endpoint to trigger
// a test run. It uses only the standard library (net/http, html/template).
package web

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/ryokotmng/software-quality-platform/internal/auth"
	"github.com/ryokotmng/software-quality-platform/internal/config"
	"github.com/ryokotmng/software-quality-platform/internal/orchestration"
	"github.com/ryokotmng/software-quality-platform/internal/reporting"
	"github.com/ryokotmng/software-quality-platform/internal/store"
)

// reportSample is a fixed set of outcomes used by GET /report so the
// endpoint produces a predictable summary for demonstration and
// screenshots, independent of any live test run.
var reportSample = []reporting.Outcome{
	{Name: "auth: register then authenticate", Passed: true},
	{Name: "auth: reject wrong password", Passed: true},
	{Name: "auth: reject unknown user", Passed: true},
	{Name: "orchestration: parse counts", Passed: true},
	{Name: "orchestration: capture failure output", Passed: false},
}

// Server wires the UI handlers to the application services.
type Server struct {
	cfg   config.Config
	auth  *auth.Service
	runs  *store.RunStore
	tmpl  *template.Template
	mux   *http.ServeMux
	runFn func(ctx context.Context, dir string, packages ...string) (orchestration.RunResult, error)
}

// NewServer builds a Server and registers its routes.
func NewServer(cfg config.Config, authSvc *auth.Service, runs *store.RunStore) *Server {
	s := &Server{
		cfg:   cfg,
		auth:  authSvc,
		runs:  runs,
		tmpl:  template.Must(template.New("dashboard").Parse(dashboardHTML)),
		mux:   http.NewServeMux(),
		runFn: orchestration.Run,
	}
	s.routes()
	return s
}

// ServeHTTP lets Server satisfy http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /", s.handleDashboard)
	s.mux.HandleFunc("GET /api/runs", s.handleListRuns)
	// Core-logic endpoints exposed for demonstration.
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("GET /report", s.handleReport)
	// Only authenticated users may trigger runs (FR-6).
	s.mux.HandleFunc("POST /api/runs/trigger", s.requireAuth(s.handleTrigger))
}

// handleLogin verifies credentials and reports whether access is
// granted. Success returns 200; failure returns 401 with an identical
// message regardless of why it failed (NFR-2).
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !s.auth.Valid(creds.Username, creds.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"authenticated": false,
			"error":         "invalid username or password",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"username":      creds.Username,
	})
}

// handleReport returns a summary (total, passed, failed, pass rate) of a
// fixed sample of test outcomes.
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, reporting.GenerateReport(reportSample))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// dashboardView is the template's data model.
type dashboardView struct {
	Latest *orchestration.RunResult
	Runs   []orchestration.RunResult
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	runs, err := s.runs.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	view := dashboardView{Runs: runs}
	if len(runs) > 0 {
		view.Latest = &runs[0]
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.runs.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

// handleTrigger runs the test suite, persists the result, and returns it.
func (s *Server) handleTrigger(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	result, err := s.runFn(ctx, s.cfg.RepoDir)
	if err != nil {
		http.Error(w, "failed to run tests: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.runs.Save(result); err != nil {
		http.Error(w, "failed to save result: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// requireAuth wraps a handler with HTTP Basic authentication validated
// against the auth service (standard route protection).
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			s.challenge(w)
			return
		}
		if _, err := s.auth.Authenticate(username, password); err != nil {
			s.challenge(w)
			return
		}
		next(w, r)
	}
}

func (s *Server) challenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Software Quality Platform"`)
	http.Error(w, "authentication required", http.StatusUnauthorized)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Software Quality Platform — Dashboard</title>
<style>
  body { font-family: system-ui, sans-serif; margin: 2rem auto; max-width: 60rem; color: #1a1a1a; }
  h1 { font-size: 1.4rem; }
  .status { display: inline-block; padding: .15rem .6rem; border-radius: .4rem; font-weight: 600; }
  .pass { background: #e5f6e5; color: #1a7f1a; }
  .fail { background: #fbe3e3; color: #b11; }
  .muted { color: #777; }
  table { border-collapse: collapse; width: 100%; margin-top: 1rem; }
  th, td { text-align: left; padding: .4rem .6rem; border-bottom: 1px solid #eee; font-variant-numeric: tabular-nums; }
  pre { background: #faf5f5; padding: .5rem; overflow-x: auto; white-space: pre-wrap; }
  details { margin-top: 1rem; }
</style>
</head>
<body>
  <h1>Software Quality Platform</h1>
  <p class="muted">Test Orchestration dashboard — latest test execution status and error logs.</p>

  {{if .Latest}}
    {{with .Latest}}
    <p>
      Latest run:
      {{if .Successful}}<span class="status pass">PASSED</span>{{else}}<span class="status fail">FAILED</span>{{end}}
      &nbsp;<span class="muted">{{.StartedAt.Format "2006-01-02 15:04:05 UTC"}} · {{.DurationMS}} ms</span>
    </p>
    <p>{{.Passed}} passed · {{.Failed}} failed · {{.Skipped}} skipped ({{.Total}} total)</p>
    {{if .Failures}}
      <details open>
        <summary>{{len .Failures}} failing test(s)</summary>
        {{range .Failures}}
          <p><strong>{{.Package}} · {{.Test}}</strong></p>
          <pre>{{.Output}}</pre>
        {{end}}
      </details>
    {{end}}
    {{end}}
  {{else}}
    <p class="muted">No test runs recorded yet. Trigger one with
      <code>POST /api/runs/trigger</code> (authenticated) or push to GitHub.</p>
  {{end}}

  <h2>Run history</h2>
  <table>
    <thead><tr><th>Started</th><th>Status</th><th>Passed</th><th>Failed</th><th>Skipped</th><th>Duration</th></tr></thead>
    <tbody>
      {{range .Runs}}
        <tr>
          <td>{{.StartedAt.Format "2006-01-02 15:04:05"}}</td>
          <td>{{if .Successful}}<span class="status pass">pass</span>{{else}}<span class="status fail">fail</span>{{end}}</td>
          <td>{{.Passed}}</td><td>{{.Failed}}</td><td>{{.Skipped}}</td><td>{{.DurationMS}} ms</td>
        </tr>
      {{else}}
        <tr><td colspan="6" class="muted">No runs yet.</td></tr>
      {{end}}
    </tbody>
  </table>
</body>
</html>`
