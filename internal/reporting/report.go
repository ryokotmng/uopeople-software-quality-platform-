// Package reporting summarizes a set of test outcomes into a compact
// quality report (the Metrics & Reporting module). It is deliberately a
// pure function over its input so it is trivial to unit-test and
// produces stable, reproducible output for documentation.
package reporting

import (
	"fmt"
	"math"
)

// Outcome is the result of a single test.
type Outcome struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
}

// Report is the computed summary of a set of outcomes.
type Report struct {
	Total    int    `json:"total"`
	Passed   int    `json:"passed"`
	Failed   int    `json:"failed"`
	PassRate string `json:"pass_rate"` // e.g. "80%"; "0%" when there are no tests
}

// GenerateReport counts the passed and failed tests and computes the
// pass rate. When there are no outcomes it returns "0%" rather than
// dividing by zero, so the summary is always well-defined.
func GenerateReport(outcomes []Outcome) Report {
	r := Report{Total: len(outcomes)}
	for _, o := range outcomes {
		if o.Passed {
			r.Passed++
		} else {
			r.Failed++
		}
	}
	if r.Total == 0 {
		r.PassRate = "0%"
		return r
	}
	pct := int(math.Round(float64(r.Passed) / float64(r.Total) * 100))
	r.PassRate = fmt.Sprintf("%d%%", pct)
	return r
}
