// Package orchestration is the Test Orchestration service (application
// layer). It runs a project's automated test suites and produces a
// structured, serializable result.
//
// It is deliberately thin: glue over the Go toolchain's `go test -json`
// output rather than a competing test framework. The service is
// decoupled from the reporting and data layers so it can be triggered by
// any front end (CLI, CI, or the HTTP dashboard) without change.
package orchestration

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// SuiteFailure describes a single failing or erroring test.
type SuiteFailure struct {
	Package string `json:"package"`
	Test    string `json:"test"`
	Output  string `json:"output"`
}

// RunResult is the outcome of one orchestrated test run.
type RunResult struct {
	StartedAt  time.Time      `json:"started_at"`
	DurationMS int64          `json:"duration_ms"`
	Total      int            `json:"total"`
	Passed     int            `json:"passed"`
	Failed     int            `json:"failed"`
	Skipped    int            `json:"skipped"`
	Failures   []SuiteFailure `json:"failures"`
}

// Successful reports whether the run had no failing or erroring tests.
func (r RunResult) Successful() bool { return r.Failed == 0 }

// event mirrors the JSON objects emitted by `go test -json`.
type event struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

// Run executes the given packages' tests under dir using the Go
// toolchain and returns a structured result. When no packages are
// given it defaults to "./..." (the whole module).
//
// A non-zero exit from `go test` because tests failed is expected and
// not treated as an error; only a failure to launch the toolchain is.
func Run(ctx context.Context, dir string, packages ...string) (RunResult, error) {
	if len(packages) == 0 {
		packages = []string{"./..."}
	}
	args := append([]string{"test", "-json"}, packages...)

	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout

	started := time.Now().UTC()
	err := cmd.Run()
	elapsed := time.Since(started)

	// An ExitError means the command ran but reported failures — that is
	// a normal outcome we still want to parse. Any other error (e.g. the
	// toolchain is missing) is a real failure to launch.
	var exitErr *exec.ExitError
	if err != nil && !asExitError(err, &exitErr) {
		return RunResult{}, err
	}

	result := parseEvents(stdout.Bytes())
	result.StartedAt = started
	result.DurationMS = elapsed.Milliseconds()
	return result, nil
}

// asExitError reports whether err is an *exec.ExitError, storing it.
func asExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*target = e
		return true
	}
	return false
}

// parseEvents turns a `go test -json` event stream into a RunResult.
// Test-level pass/fail/skip events are counted; output lines belonging
// to failing tests are collected into their SuiteFailure.
func parseEvents(data []byte) RunResult {
	var result RunResult
	output := map[string]*strings.Builder{}
	key := func(e event) string { return e.Package + "\x00" + e.Test }

	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var e event
		if err := json.Unmarshal(line, &e); err != nil {
			continue // ignore non-JSON build noise
		}

		if e.Action == "output" && e.Test != "" {
			b := output[key(e)]
			if b == nil {
				b = &strings.Builder{}
				output[key(e)] = b
			}
			b.WriteString(e.Output)
			continue
		}

		if e.Test == "" {
			continue // package-level summary events are not counted
		}

		switch e.Action {
		case "pass":
			result.Passed++
		case "skip":
			result.Skipped++
		case "fail":
			result.Failed++
			out := ""
			if b := output[key(e)]; b != nil {
				out = strings.TrimSpace(b.String())
			}
			result.Failures = append(result.Failures, SuiteFailure{
				Package: e.Package,
				Test:    e.Test,
				Output:  out,
			})
		}
	}

	result.Total = result.Passed + result.Failed + result.Skipped
	return result
}
