package date

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// FIELD is the reply key a date task's prompt must fill.
const FIELD = "date"

// ROC_EPOCH_OFFSET converts a Taiwan 民國 (Republic of China era) year to a
// Gregorian year: 民國 115 + 1911 = 2026. Taiwan receipts and hospital slips
// routinely print the era year alone ("115/02/23"), so any year below the
// first Gregorian year the era can express is read as an era year.
const ROC_EPOCH_OFFSET = 1911

var datePattern = regexp.MustCompile(`^(\d{1,4})-(\d{2}-\d{2})$`)

// Parse validates value as a past-or-today YYYY-MM-DD (ROC years allowed)
// and returns it in Gregorian form.
func Parse(value any, today time.Time) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must use YYYY-MM-DD", FIELD)
	}
	fields := datePattern.FindStringSubmatch(text)
	if fields == nil {
		return "", fmt.Errorf("%s must use YYYY-MM-DD, got %q", FIELD, text)
	}
	year, err := strconv.Atoi(fields[1])
	if err != nil || year == 0 {
		return "", fmt.Errorf("%s is not a calendar date: %q", FIELD, text)
	}
	if year <= ROC_EPOCH_OFFSET {
		year += ROC_EPOCH_OFFSET
	}
	text = fmt.Sprintf("%04d-%s", year, fields[2])

	parsed, err := time.ParseInLocation("2006-01-02", text, today.Location())
	if err != nil {
		return "", fmt.Errorf("%s is not a calendar date: %q", FIELD, text)
	}
	if parsed.After(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())) {
		return "", fmt.Errorf("%s cannot be in the future: %s", FIELD, text)
	}
	return text, nil
}
