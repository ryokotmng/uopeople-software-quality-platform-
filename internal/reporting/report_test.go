package reporting

import "testing"

func TestGenerateReport(t *testing.T) {
	got := GenerateReport([]Outcome{
		{Name: "TestA", Passed: true},
		{Name: "TestB", Passed: true},
		{Name: "TestC", Passed: true},
		{Name: "TestD", Passed: true},
		{Name: "TestE", Passed: false},
	})

	if got.Total != 5 || got.Passed != 4 || got.Failed != 1 {
		t.Errorf("counts = %d/%d/%d, want total=5 passed=4 failed=1", got.Total, got.Passed, got.Failed)
	}
	if got.PassRate != "80%" { // 4 of 5
		t.Errorf("PassRate = %q, want %q", got.PassRate, "80%")
	}
}

func TestGenerateReportRoundsPassRate(t *testing.T) {
	// 2 of 3 = 66.6...% should round to 67%.
	got := GenerateReport([]Outcome{
		{Name: "TestA", Passed: true},
		{Name: "TestB", Passed: true},
		{Name: "TestC", Passed: false},
	})
	if got.PassRate != "67%" {
		t.Errorf("PassRate = %q, want %q", got.PassRate, "67%")
	}
}

func TestGenerateReportEmptyReturnsZeroPercent(t *testing.T) {
	// With no test results the pass rate must be a safe "0%", never a
	// divide-by-zero or misleading value.
	got := GenerateReport(nil)
	if got.Total != 0 || got.Passed != 0 || got.Failed != 0 {
		t.Errorf("counts = %d/%d/%d, want all zero", got.Total, got.Passed, got.Failed)
	}
	if got.PassRate != "0%" {
		t.Errorf("PassRate = %q, want %q", got.PassRate, "0%")
	}
}
