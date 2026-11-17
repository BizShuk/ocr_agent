package classify

import (
	"context"
	"errors"
	"testing"

	"github.com/bizshuk/agentsdk/core"
	"github.com/bizshuk/ocr_agent/svc/imagefile"
)

// parseCategory is unexported, so this file stays in package classify
// rather than classify_test — model-output parsing is the one thing worth
// testing from the inside, because every reply is untrusted input.
func TestParseCategoryAcceptsKnownVerdicts(t *testing.T) {
	cases := map[string]Category{
		`{"category":"general_receipt"}`:                               CATEGORY_GENERAL,
		`{"category":"medical_receipt"}`:                               CATEGORY_MEDICAL,
		`{"category":"non_receipt"}`:                                   CATEGORY_NON,
		"```json\n{\"category\":\"non_receipt\"}\n```":                 CATEGORY_NON,
		"Sure!\n{\"category\": \"medical_receipt\"}\nHope that helps.": CATEGORY_MEDICAL,
		`{"category":" general_receipt "}`:                             CATEGORY_GENERAL,
	}
	for raw, want := range cases {
		got, err := parseCategory(raw)
		if err != nil {
			t.Fatalf("parseCategory(%q): unexpected error %v", raw, err)
		}
		if got != want {
			t.Fatalf("parseCategory(%q) = %q, want %q", raw, got, want)
		}
	}
}

// Anything outside the closed set must fail the file rather than route it
// somewhere arbitrary — a hallucinated category is not a filing decision.
func TestParseCategoryRejectsBadOutput(t *testing.T) {
	for _, raw := range []string{
		``,
		`no json here`,
		`{"category":"receipt"}`,
		`{"category":""}`,
		`{"kategorie":"non_receipt"}`,
		`{"category": 3}`,
		`["non_receipt"]`,
	} {
		if got, err := parseCategory(raw); err == nil {
			t.Fatalf("parseCategory(%q) = %q, want error", raw, got)
		}
	}
}

// stubProvider is a core.Provider that returns a canned reply, so the
// classifier can be exercised without a running model. core.Provider is a
// single Generate method, so that is the whole stub.
type stubProvider struct {
	text string
	err  error
	seen core.ModelRequest
}

func (s *stubProvider) Generate(_ context.Context, req core.ModelRequest) (core.ModelResult, error) {
	s.seen = req
	if s.err != nil {
		return core.ModelResult{}, s.err
	}
	return core.ModelResult{Text: s.text}, nil
}

// The request must carry the prompt AND the image; sending text alone is
// exactly the bug that made the SDK's ollama adapter unusable for this.
func TestClassifyAttachesPromptAndImage(t *testing.T) {
	stub := &stubProvider{text: `{"category":"general_receipt"}`}
	got, err := Classifier{Provider: stub}.Classify(
		context.Background(),
		imagefile.Image{Data: []byte{0xff, 0xd8}, MIME: "image/jpeg"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != CATEGORY_GENERAL {
		t.Fatalf("got %q, want %q", got, CATEGORY_GENERAL)
	}

	if len(stub.seen.Messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(stub.seen.Messages))
	}
	parts := stub.seen.Messages[0].Parts
	if len(parts) != 2 {
		t.Fatalf("want 2 parts (text + image), got %d", len(parts))
	}
	if parts[0].Kind != core.PART_KIND_PLAIN_TEXT || parts[0].Text != PROMPT {
		t.Fatalf("first part must be the classify prompt, got %+v", parts[0])
	}
	if parts[1].Kind != core.PART_KIND_IMAGE || parts[1].ImageMIME != "image/jpeg" {
		t.Fatalf("second part must be the image, got %+v", parts[1])
	}
}

func TestClassifyWithoutProvider(t *testing.T) {
	if _, err := (Classifier{}).Classify(context.Background(), imagefile.Image{}); err == nil {
		t.Fatal("expected an error when no provider is configured")
	}
}

func TestClassifyPropagatesProviderError(t *testing.T) {
	stub := &stubProvider{err: errors.New("connection refused")}
	_, err := Classifier{Provider: stub}.Classify(context.Background(), imagefile.Image{Data: []byte{1}})
	if err == nil {
		t.Fatal("expected the provider error to surface")
	}
}
