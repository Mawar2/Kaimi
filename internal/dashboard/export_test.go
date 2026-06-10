package dashboard

import (
	"testing"
	"time"
)

// TestFYQuarter checks the US federal fiscal-year quarter mapping (FY starts
// Oct 1): Oct-Dec = Q1 of FY+1, Jan-Mar = Q2, Apr-Jun = Q3, Jul-Sep = Q4.
func TestFYQuarter(t *testing.T) {
	tests := []struct {
		date    string
		wantFY  int
		wantQ   int
		wantLbl string
	}{
		{"2025-10-01", 2026, 1, "FY26 Q1"}, // FY boundary: Oct rolls to next FY
		{"2025-12-31", 2026, 1, "FY26 Q1"},
		{"2026-01-01", 2026, 2, "FY26 Q2"},
		{"2026-03-31", 2026, 2, "FY26 Q2"},
		{"2026-04-01", 2026, 3, "FY26 Q3"},
		{"2026-06-30", 2026, 3, "FY26 Q3"},
		{"2026-07-01", 2026, 4, "FY26 Q4"},
		{"2026-09-30", 2026, 4, "FY26 Q4"},
	}
	for _, tt := range tests {
		d, _ := time.Parse("2006-01-02", tt.date)
		fy, q := fyQuarter(d)
		if fy != tt.wantFY || q != tt.wantQ {
			t.Errorf("fyQuarter(%s) = FY%d Q%d, want FY%d Q%d", tt.date, fy, q, tt.wantFY, tt.wantQ)
		}
		if got := fyQuarterLabel(d); got != tt.wantLbl {
			t.Errorf("fyQuarterLabel(%s) = %q, want %q", tt.date, got, tt.wantLbl)
		}
	}
}
