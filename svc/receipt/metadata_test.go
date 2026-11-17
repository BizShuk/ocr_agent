package receipt

import (
	"testing"
	"time"
)

var metadataToday = time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)

// Taiwan receipts print the year in the 民國 (ROC) era. A bare era year is
// converted with +1911; a Gregorian year is left alone.
func TestValidateReceiptDateConvertsROCYear(t *testing.T) {
	cases := []struct {
		date string
		want string
	}{
		{"115-02-23", "2026-02-23"},
		{"0115-02-23", "2026-02-23"},
		{"104-02-23", "2015-02-23"},
		{"2026-02-23", "2026-02-23"},
		{"2015-02-23", "2015-02-23"},
	}
	for _, c := range cases {
		got, err := validateReceiptDate(c.date, metadataToday)
		if err != nil {
			t.Errorf("validateReceiptDate(%q) returned error: %v", c.date, err)
			continue
		}
		if got != c.want {
			t.Errorf("validateReceiptDate(%q) = %q, want %q", c.date, got, c.want)
		}
	}
}

func TestValidateReceiptDateRejectsInvalid(t *testing.T) {
	cases := []string{
		"",
		"0-02-23",
		"0000-02-23",
		"2026-13-01",
		"2026-02-30",
		"116-01-01", // ROC 116 is 2027, still in the future
		"2027-01-01",
		"23/02/2026",
	}
	for _, date := range cases {
		if got, err := validateReceiptDate(date, metadataToday); err == nil {
			t.Errorf("validateReceiptDate(%q) = %q, want error", date, got)
		}
	}
}
