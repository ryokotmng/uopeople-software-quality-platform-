package orchestration

import "testing"

// A representative slice of a `go test -json` stream: one passing test,
// one failing test (with output), and one skipped test.
const sampleEvents = `
{"Action":"run","Package":"example/pkg","Test":"TestA"}
{"Action":"pass","Package":"example/pkg","Test":"TestA","Elapsed":0.01}
{"Action":"run","Package":"example/pkg","Test":"TestB"}
{"Action":"output","Package":"example/pkg","Test":"TestB","Output":"    want 1, got 2\n"}
{"Action":"fail","Package":"example/pkg","Test":"TestB","Elapsed":0.02}
{"Action":"run","Package":"example/pkg","Test":"TestC"}
{"Action":"skip","Package":"example/pkg","Test":"TestC","Elapsed":0.00}
{"Action":"pass","Package":"example/pkg","Elapsed":0.03}
`

func TestParseEventsCounts(t *testing.T) {
	r := parseEvents([]byte(sampleEvents))

	if r.Total != 3 {
		t.Errorf("Total = %d, want 3", r.Total)
	}
	if r.Passed != 1 || r.Failed != 1 || r.Skipped != 1 {
		t.Errorf("passed/failed/skipped = %d/%d/%d, want 1/1/1", r.Passed, r.Failed, r.Skipped)
	}
	if r.Successful() {
		t.Error("Successful() = true, want false (one test failed)")
	}
}

func TestParseEventsCapturesFailureOutput(t *testing.T) {
	r := parseEvents([]byte(sampleEvents))

	if len(r.Failures) != 1 {
		t.Fatalf("len(Failures) = %d, want 1", len(r.Failures))
	}
	f := r.Failures[0]
	if f.Test != "TestB" || f.Package != "example/pkg" {
		t.Errorf("failure identity = %s/%s, want example/pkg/TestB", f.Package, f.Test)
	}
	if f.Output != "want 1, got 2" {
		t.Errorf("failure output = %q, want %q", f.Output, "want 1, got 2")
	}
}

func TestParseEventsIgnoresBuildNoise(t *testing.T) {
	// Non-JSON lines (e.g. compiler errors) must not break parsing.
	r := parseEvents([]byte("FAIL\tbroken build\n" + sampleEvents))
	if r.Total != 3 {
		t.Errorf("Total = %d, want 3 (noise ignored)", r.Total)
	}
}
