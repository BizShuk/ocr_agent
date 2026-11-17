// Package classify decides which document domain an image belongs to.
package classify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bizshuk/agentsdk/core"
	"github.com/bizshuk/ocr_agent/svc/imagefile"
	"github.com/bizshuk/ocr_agent/svc/vision"
)

// Category is the verdict a single image gets from the vision model. The
// three values are a closed set — anything else is a model fault and fails
// the file rather than routing it somewhere arbitrary.
type Category string

const (
	CATEGORY_GENERAL Category = "general_receipt"
	CATEGORY_MEDICAL Category = "medical_receipt"
	CATEGORY_NON     Category = "non_receipt"
)

// Valid reports whether c is one of the three known verdicts.
func (c Category) Valid() bool {
	return c == CATEGORY_GENERAL || c == CATEGORY_MEDICAL || c == CATEGORY_NON
}

// PROMPT is the classify domain's whole instruction to the model.
const PROMPT = `Analyze this image and identify its category.

Respond with ONLY a JSON object in this format:
{
  "category": "general_receipt" | "medical_receipt" | "non_receipt"
}

Rules:
- "medical_receipt": Receipts, bills, invoices from clinics, TCM (Traditional Chinese Medicine) practitioners, hospitals, dental, pharmacies, or any medical diagnosis document (like Medical Certificate / MC). Also include bank transfer screenshots where the payee is a medical clinic or doctor (e.g. Great Wall TCM, Family Clinic, etc.).
- "general_receipt": Any standard retail, dining, food court, supermarket, taxi, transport receipts, or general banking transfer screenshots to non-medical payees.
- "non_receipt": Any image that is not a receipt, bill, invoice, medical certificate, or payment transaction slip.

Response MUST be a valid JSON object only. Do not wrap in markdown or add explanations.`

// MAX_TOKENS caps the reply. The contract is a one-field JSON object, so a
// model that rambles past this budget is already off-script.
const MAX_TOKENS = 256

// Classifier asks a vision model which domain an image belongs to.
type Classifier struct {
	Provider core.Provider
}

// Classify sends one image and returns its category.
func (c Classifier) Classify(ctx context.Context, img imagefile.Image) (Category, error) {
	if c.Provider == nil {
		return "", errors.New("classifier has no provider")
	}
	reply, err := vision.Ask(ctx, c.Provider, PROMPT, img, MAX_TOKENS)
	if err != nil {
		return "", err
	}
	return parseCategory(reply)
}

func parseCategory(raw string) (Category, error) {
	body := vision.ExtractJSONObject(raw)
	if body == "" {
		return "", fmt.Errorf("model reply contains no JSON object: %q", vision.Truncate(raw, 200))
	}
	var reply struct {
		Category string `json:"category"`
	}
	if err := json.Unmarshal([]byte(body), &reply); err != nil {
		return "", fmt.Errorf("model reply is not a JSON object: %w", err)
	}
	category := Category(strings.TrimSpace(reply.Category))
	if !category.Valid() {
		return "", fmt.Errorf("unsupported category %q", reply.Category)
	}
	return category, nil
}
