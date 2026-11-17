package receipt

import (
	"fmt"

	"github.com/bizshuk/agentsdk/core"
	"github.com/bizshuk/ocr_agent/svc/classify"
)

// MEDICAL_PROMPT is the Stage 2 contract for medical receipts and MCs.
const MEDICAL_PROMPT = `Analyze this medical receipt or medical certificate image. This is from a clinic, doctor, Traditional Chinese Medicine (TCM) practitioner, hospital, or pharmacy.

Respond with ONLY a JSON object in this format:
{
  "category": "medical_receipt",
  "date": "YYYY-MM-DD",
  "time": "HH:MM",
  "clinic_name": "clinic_name_lowercase_underscore_separated",
  "clinic_uen": "uen_or_na",
  "patient_name": "patient_name",
  "patient_id": "patient_id_or_na",
  "doctor_name": "doctor_name_or_na",
  "doctor_mcr": "doctor_mcr_or_na",
  "invoice_no": "invoice_no_or_na",
  "receipt_no": "receipt_no_or_na",
  "payment_method": "payment_method",
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
  "total_amount": 0.00,
  "is_medical_certificate": true/false,
  "mc_start_date": "YYYY-MM-DD or null",
  "mc_end_date": "YYYY-MM-DD or null",
  "mc_days": 1 or null,
  "diagnosis": "string or null"
}

Rules:
- category: MUST be "medical_receipt".
- date: YYYY-MM-DD (Gregorian). Extract the invoice date or the date the document was issued (e.g. 看診日期, 就醫日期, 開立日期).
- Taiwan documents print the year in the 民國 (ROC) era: a 2-3 digit year such as "115/02/23", "1150223" or "民國115年2月23日". Convert it by ADDING 1911 (115 -> 2026, 104 -> 2015). Never read a 3-digit era year as a Gregorian year, and never reuse the patient birth year (生日, also printed in ROC era) as the visit date.
- time: HH:MM or "N/A" if not visible.
- clinic_name: The clinic or hospital name EXACTLY as printed in the document header, transliterated; never guess a sibling branch of the same hospital group. MUST be formatted as lowercase, words separated by underscores (e.g. "great_wall_tcm", "my_family_clinic", "wanfang_hospital").
- clinic_uen: The UEN/Registration ID of the clinic (if visible, e.g. "201725438K") or "N/A". A Taiwan 批價序號 / 收據號碼 / 醫令序號 is a document serial number, not a UEN — put it in receipt_no and return "N/A" here.
- patient_name: The patient name (if visible, e.g. "Liu Teng-Yuan") or "N/A". On Chinese documents this is the 姓名 / 病患姓名 field, NOT the 醫師 field.
- patient_id: The patient's NRIC/ID number (if visible, e.g. "S8669082I") or "N/A". On Taiwan documents use the 身分證 number; if only 病歷號 is printed use that.
- doctor_name: The doctor/physician/consultant's name (if visible) or "N/A". On Chinese documents this is the 醫師 / 主治醫師 field.
- doctor_mcr: The doctor's Medical Council Registration (MCR) number (if visible, e.g. "M04788J") or "N/A". A Taiwan 病歷號 belongs to the patient, never here.
- invoice_no: The invoice number (if visible, e.g. "IN74687") or "N/A".
- receipt_no: The receipt number (if visible, e.g. "RT-TLH0073009") or "N/A".
- payment_method: The payment method (e.g. "PayNow", "NETS", "Visa", "Cash") or "N/A".
- items: An array containing medical items, consultation fees, or prescriptions. For Medical Certificates or bank slips, this can be empty ` + "`[]`" + `.
- currency: lowercase 3-letter currency code (e.g. "sgd", "usd", "twd").
- subtotal: Subtotal amount before GST (if visible, default 0.00).
- gst: GST amount (if visible, default 0.00).
- rounding: Rounding adjustments (if visible, default 0.00).
- total_amount: The final amount paid. If this is a Medical Certificate (MC) with no charge, or a consultation tax invoice showing $0.00, return 0.00.
- is_medical_certificate: true if the document is a Medical Certificate (MC/Unfit for work certificate). otherwise false.
- mc_start_date: YYYY-MM-DD, the start date of medical leave (if this is an MC) or null.
- mc_end_date: YYYY-MM-DD, the end date of medical leave (if this is an MC) or null.
- mc_days: The number of days of medical leave (if this is an MC) or null.
- diagnosis: The diagnosis or reason for consultation (if visible, e.g. "左侧落枕3天" or "左膝内侧痛。") or "N/A".

Response MUST be a valid JSON object only. Do not wrap in markdown or add explanations.`

// NewMedicalProcessor returns a Processor for medical receipts and MCs.
func NewMedicalProcessor(provider core.Provider) Processor {
	return Processor{
		Provider: provider,
		category: classify.CATEGORY_MEDICAL,
		prompt:   MEDICAL_PROMPT,
	}
}

func parseMedicalMetadata(
	values map[string]any,
	date string,
	extension string,
) (receiptMetadata, error) {
	clinicRaw, err := requiredString(values, "clinic_name")
	if err != nil {
		return receiptMetadata{}, err
	}
	clinic, err := sanitizeName(clinicRaw)
	if err != nil {
		return receiptMetadata{}, fmt.Errorf("clinic_name: %w", err)
	}
	isMC, ok := values["is_medical_certificate"].(bool)
	if !ok {
		return receiptMetadata{}, fmt.Errorf("is_medical_certificate must be boolean")
	}
	if isMC {
		return receiptMetadata{
			values:      values,
			date:        date,
			description: fmt.Sprintf("%s | MC", clinicRaw),
			filename:    fmt.Sprintf("%s_%s_mc.%s", date, clinic, extension),
		}, nil
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
		description: fmt.Sprintf("%s | %s%s", clinicRaw, currency, amount),
		filename:    fmt.Sprintf("%s_%s_%s%s.%s", date, clinic, currency, amount, extension),
	}, nil
}
