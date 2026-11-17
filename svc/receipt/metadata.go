// Package receipt extracts, validates and names receipt metadata for the
// general and medical domains, then archives each image by month.
package receipt

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bizshuk/ocr_agent/svc/classify"
	"github.com/bizshuk/ocr_agent/svc/vision"
)

var (
	currencyPattern = regexp.MustCompile(`^[A-Za-z]{3}$`)
	datePattern     = regexp.MustCompile(`^(\d{1,4})-(\d{2}-\d{2})$`)
	namePattern     = regexp.MustCompile(`[^a-z0-9_]+`)
	underscoreRun   = regexp.MustCompile(`_+`)
	amountPattern   = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$`)
)

type receiptMetadata struct {
	values      map[string]any
	date        string
	description string
	filename    string
}

func parseReceiptMetadata(
	raw string,
	category classify.Category,
	extension string,
	today time.Time,
) (receiptMetadata, error) {
	body := vision.ExtractJSONObject(raw)
	if body == "" {
		return receiptMetadata{}, fmt.Errorf(
			"model reply contains no JSON object: %q",
			vision.Truncate(raw, 200),
		)
	}

	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	values := make(map[string]any)
	if err := decoder.Decode(&values); err != nil {
		return receiptMetadata{}, fmt.Errorf("model reply is not a JSON object: %w", err)
	}

	modelCategory, err := requiredString(values, "category")
	if err != nil {
		return receiptMetadata{}, err
	}
	if modelCategory != string(category) {
		return receiptMetadata{}, fmt.Errorf(
			"category must be %s, got %q",
			category,
			modelCategory,
		)
	}

	date, err := validateReceiptDate(values["date"], today)
	if err != nil {
		return receiptMetadata{}, err
	}
	extension = strings.ToLower(strings.TrimPrefix(extension, "."))
	if extension == "" {
		return receiptMetadata{}, fmt.Errorf("source image has no extension")
	}

	switch category {
	case classify.CATEGORY_GENERAL:
		return parseGeneralMetadata(values, date, extension)
	case classify.CATEGORY_MEDICAL:
		return parseMedicalMetadata(values, date, extension)
	default:
		return receiptMetadata{}, fmt.Errorf("unsupported processing category %q", category)
	}
}

func requiredString(values map[string]any, field string) (string, error) {
	value, ok := values[field]
	if !ok || value == nil {
		return "", fmt.Errorf("missing field %q", field)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be text", field)
	}
	text = strings.TrimSpace(text)
	switch strings.ToLower(text) {
	case "", "n/a", "null", "unknown":
		return "", fmt.Errorf("missing field %q", field)
	}
	return text, nil
}

// ROC_EPOCH_OFFSET converts a Taiwan 民國 (Republic of China era) year to a
// Gregorian year: 民國 115 + 1911 = 2026. Taiwan receipts and hospital slips
// routinely print the era year alone ("115/02/23"), so any year below the
// first Gregorian year the era can express is read as an era year.
const ROC_EPOCH_OFFSET = 1911

func validateReceiptDate(value any, today time.Time) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("date must use YYYY-MM-DD")
	}
	fields := datePattern.FindStringSubmatch(text)
	if fields == nil {
		return "", fmt.Errorf("date must use YYYY-MM-DD")
	}
	year, err := strconv.Atoi(fields[1])
	if err != nil || year == 0 {
		return "", fmt.Errorf("date is not a calendar date")
	}
	if year <= ROC_EPOCH_OFFSET {
		year += ROC_EPOCH_OFFSET
	}
	text = fmt.Sprintf("%04d-%s", year, fields[2])

	parsed, err := time.ParseInLocation("2006-01-02", text, today.Location())
	if err != nil {
		return "", fmt.Errorf("date is not a calendar date")
	}
	todayDate := time.Date(
		today.Year(),
		today.Month(),
		today.Day(),
		0,
		0,
		0,
		0,
		today.Location(),
	)
	if parsed.After(todayDate) {
		return "", fmt.Errorf("date cannot be in the future")
	}
	return text, nil
}

func normalizeCurrency(value any) (string, error) {
	text, ok := value.(string)
	if !ok || !currencyPattern.MatchString(text) {
		return "", fmt.Errorf("currency must be three ASCII letters")
	}
	return strings.ToLower(text), nil
}

func formatAmount(value any) (string, error) {
	var text string
	switch typed := value.(type) {
	case json.Number:
		text = typed.String()
	case string:
		text = strings.TrimSpace(typed)
	default:
		return "", fmt.Errorf("amount must be numeric")
	}
	if !amountPattern.MatchString(text) {
		return "", fmt.Errorf("amount must be numeric")
	}

	amount, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(amount, 0) || math.IsNaN(amount) {
		return "", fmt.Errorf("amount must be finite")
	}
	if amount < 0 {
		return "", fmt.Errorf("amount must be non-negative")
	}

	mantissa := text
	exponent := 0
	if split := strings.IndexAny(mantissa, "eE"); split >= 0 {
		parsedExponent, err := strconv.Atoi(mantissa[split+1:])
		if err != nil {
			return "", fmt.Errorf("amount must be numeric")
		}
		exponent = parsedExponent
		mantissa = mantissa[:split]
	}
	fractionDigits := 0
	if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
		fractionDigits = len(mantissa) - dot - 1
	}
	if exponent-fractionDigits < -2 {
		return "", fmt.Errorf("amount must use at most two decimals")
	}
	return fmt.Sprintf("%.2f", amount), nil
}

func sanitizeName(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = namePattern.ReplaceAllString(value, "_")
	value = underscoreRun.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "", fmt.Errorf("name must contain an ASCII identifier")
	}
	return value, nil
}
