// Integration and system-level tests for the HTTP layer. Unlike the
// unit tests next to each package (internal/.../*_test.go), these spin
// up a real HTTP server via httptest.NewServer and drive it with an
// ordinary net/http.Client, so requests travel over an actual loopback
// socket rather than calling handlers in-process. This is what backs
// the report's integration- and system-level test cases: that the HTTP
// handlers correctly call the backend logic and return the expected
// output, and that the application stays stable under repeated
// requests.
package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryokotmng/software-quality-platform/internal/auth"
	"github.com/ryokotmng/software-quality-platform/internal/config"
	"github.com/ryokotmng/software-quality-platform/internal/store"
	"github.com/ryokotmng/software-quality-platform/internal/web"
)

// newIntegrationServer builds a full server (auth + store + web) backed
// by a temporary directory, seeds one user, and returns it running on a
// real local port.
func newIntegrationServer(t *testing.T) *httptest.Server {
	t.Helper()
	dir := t.TempDir()

	authSvc, err := auth.New(filepath.Join(dir, "users.json"))
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	if err := authSvc.EnsureUser("admin", "changeme"); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	runs, err := store.NewRunStore(filepath.Join(dir, "runs"))
	if err != nil {
		t.Fatalf("store.NewRunStore: %v", err)
	}

	srv := web.NewServer(config.Config{RepoDir: "."}, authSvc, runs)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

func postLogin(t *testing.T, baseURL, username, password string) (*http.Response, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp, err := http.Post(baseURL+"/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode /login response: %v", err)
	}
	return resp, decoded
}

// TestLoginIntegration exercises the login flow's two primary cases —
// valid credentials passing and invalid credentials being rejected —
// over a real HTTP connection to a running server.
func TestLoginIntegration(t *testing.T) {
	ts := newIntegrationServer(t)

	t.Run("valid credentials", func(t *testing.T) {
		resp, body := postLogin(t, ts.URL, "admin", "changeme")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if body["authenticated"] != true {
			t.Fatalf("authenticated = %v, want true", body["authenticated"])
		}
		t.Logf("POST /login (valid) -> %d %v", resp.StatusCode, body)
	})

	t.Run("invalid credentials", func(t *testing.T) {
		resp, body := postLogin(t, ts.URL, "admin", "wrong-password")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
		if body["authenticated"] != false {
			t.Fatalf("authenticated = %v, want false", body["authenticated"])
		}
		t.Logf("POST /login (invalid) -> %d %v", resp.StatusCode, body)
	})
}

// TestReportIntegration confirms that GET /report returns the expected
// JSON summary shape and values over a real HTTP connection.
func TestReportIntegration(t *testing.T) {
	ts := newIntegrationServer(t)

	resp, err := http.Get(ts.URL + "/report")
	if err != nil {
		t.Fatalf("GET /report: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var report struct {
		Total    int    `json:"total"`
		Passed   int    `json:"passed"`
		Failed   int    `json:"failed"`
		PassRate string `json:"pass_rate"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		t.Fatalf("decode /report response: %v", err)
	}
	t.Logf("GET /report -> %d %+v", resp.StatusCode, report)

	if report.Total != 5 || report.Passed != 4 || report.Failed != 1 || report.PassRate != "80%" {
		t.Fatalf("report = %+v, want {total:5 passed:4 failed:1 pass_rate:80%%}", report)
	}
}

// TestSystemStabilityUnderRepeatedRequests exercises both endpoints
// repeatedly over real HTTP connections and confirms the application
// keeps responding correctly throughout — a system-level check that the
// integrated application does not degrade or fail under sustained local
// use, distinct from testing any single request in isolation.
func TestSystemStabilityUnderRepeatedRequests(t *testing.T) {
	ts := newIntegrationServer(t)
	client := ts.Client()

	const n = 200
	start := time.Now()
	for i := 1; i <= n; i++ {
		reportResp, err := client.Get(ts.URL + "/report")
		if err != nil {
			t.Fatalf("request %d: GET /report: %v", i, err)
		}
		reportResp.Body.Close()
		if reportResp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: /report status = %d, want 200", i, reportResp.StatusCode)
		}

		loginResp, _ := postLogin(t, ts.URL, "admin", "changeme")
		if loginResp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: /login status = %d, want 200", i, loginResp.StatusCode)
		}

		if i%50 == 0 {
			t.Logf("completed %d/%d request pairs, elapsed=%s", i, n, time.Since(start).Round(time.Millisecond))
		}
	}

	t.Logf("system stability check complete: %d /report + %d /login requests succeeded, elapsed=%s",
		n, n, time.Since(start).Round(time.Millisecond))
}
