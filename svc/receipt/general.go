package receipt

import (
	"fmt"

	"github.com/bizshuk/agentsdk/core"
	"github.com/bizshuk/ocr_agent/svc/classify"
)

// GENERAL_PROMPT is the Stage 2 contract for ordinary receipts.
const GENERAL_PROMPT = `Analyze this general receipt image. This is a general receipt from retail, dining, food, travel, or supermarket.

Respond with ONLY a JSON object in this format:
{
  "category": "general_receipt",
  "date": "YYYY-MM-DD",
  "time": "HH:MM",
  "merchant": "merchant_name_lowercase_underscore_separated",
  "merchant_uen": "uen_or_na",
  "payment_method": "payment_method",
  "reference_no": "reference_number_or_na",
  "items": [
    {
      "name": "string",
      "qty": 1,
      "unit_price": 0.00,
      "total_price": 0.00
    }
  ],
  "currency": "lowercase_3_letter_currency_code",
  "subtotal": 0.00,
  "gst": 0.00,
  "rounding": 0.00,
  "total_amount": 0.00
}

Rules:
- category: MUST be "general_receipt".
- date: YYYY-MM-DD (Gregorian), extract the transaction date.
- Taiwan receipts print the year in the 民國 (ROC) era: a 2-3 digit year such as "115/02/23", "1150223" or "民國115年2月23日". Convert it by ADDING 1911 (115 -> 2026, 104 -> 2015). Never read a 3-digit era year as a Gregorian year.
- time: HH:MM or "N/A" if not visible.
- merchant: The payee or merchant name, MUST be formatted as lowercase, words separated by underscores (e.g. "md40", "ah_luck_six", "citadines_kitchens").
- merchant_uen: The UEN/Registration ID of the merchant (if visible, e.g. "201725438K") or "N/A".
- payment_method: The payment method (e.g. "PayNow", "Visa", "NETS", "Cash") or "N/A".
- reference_no: The transaction reference number, STAN ID, or trace ID (if visible) or "N/A".
- items: An array containing each purchased item. If individual items are not listed (like in bank transfer slips), return an empty array ` + "`[]`" + `.
- currency: lowercase 3-letter currency code (e.g. "sgd", "usd", "twd").
- subtotal: The subtotal amount before tax/GST (if visible as a number, e.g. 12.50) or 0.00.
- gst: The tax or GST amount (if visible, e.g. 1.13) or 0.00.
- rounding: The rounding adjustment (if visible, e.g. -0.02) or 0.00.
- total_amount: The final paid amount (decimal string or number, no currency symbol, e.g. 12.50).

Response MUST be a valid JSON object only. Do not wrap in markdown or add explanations.`

// NewGeneralProcessor returns a Processor for ordinary receipts.
func NewGeneralProcessor(provider core.Provider) Processor {
	return Processor{
		Provider: provider,
		category: classify.CATEGORY_GENERAL,
		prompt:   GENERAL_PROMPT,
	}
}

func parseGeneralMetadata(
	values map[string]any,
	date string,
	extension string,
) (receiptMetadata, error) {
	merchantRaw, err := requiredString(values, "merchant")
	if err != nil {
		return receiptMetadata{}, err
	}
	merchant, err := sanitizeName(merchantRaw)
	if err != nil {
		return receiptMetadata{}, fmt.Errorf("merchant: %w", err)
	}
	currency, err := normalizeCurrency(values["currency"])
	if err != nil {
		return receiptMetadata{}, err
	}
	amount, err := formatAmount(values["total_amount"])
	if err != nil {
		return receiptMetadata{}, err
	}

	return receiptMetadata{
		values:      values,
		date:        date,
		description: fmt.Sprintf("%s | %s%s", merchantRaw, currency, amount),
		filename:    fmt.Sprintf("%s_%s_%s%s.%s", date, merchant, currency, amount, extension),
	}, nil
}
