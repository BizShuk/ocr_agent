package date

import (
	"testing"
	"time"
)

var today = time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)

// Taiwan receipts print the year in the 民國 (ROC) era. A bare era year is
// converted with +1911; a Gregorian year is left alone.
func TestParseConvertsROCYear(t *testing.T) {
	cases := map[string]string{
		"115-02-23":  "2026-02-23",
		"0115-02-23": "2026-02-23",
		"104-02-23":  "2015-02-23",
		"2026-02-23": "2026-02-23",
		"2015-02-23": "2015-02-23",
	}
	for in, want := range cases {
		got, err := Parse(in, today)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	for _, in := range []any{nil, 20260223, "", "0-02-23", "0000-02-23", "2026-13-01", "2026-02-30", "116-01-01", "2027-01-01", "23/02/2026"} {
		if got, err := Parse(in, today); err == nil {
			t.Errorf("Parse(%v) = %q, want error", in, got)
		}
	}
}
