package task

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bizshuk/agentsdk/core"
	"github.com/bizshuk/ocr_agent/svc/target"
)

// stubModel answers every request with calls, recording what it was sent.
type stubModel struct {
	calls []core.ToolCall
	text  string
	seen  core.ModelRequest
}

func (s *stubModel) Generate(_ context.Context, req core.ModelRequest) (core.ModelResult, error) {
	s.seen = req
	return core.ModelResult{Text: s.text, ToolCalls: s.calls}, nil
}

func mvCall(args map[string]any) []core.ToolCall {
	return []core.ToolCall{{ID: "1", Name: TOOL_NAME, Args: args}}
}

func stage(t *testing.T) (source, root string) {
	t.Helper()
	root = t.TempDir()
	source = filepath.Join(root, "inbox", "IMG_1.jpg")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte{0xff, 0xd8}, 0o600); err != nil {
		t.Fatal(err)
	}
	return source, root
}

func targets(root string) []target.Target {
	return []target.Target{
		{Name: "special", Description: "kept documents", Path: filepath.Join(root, "special")},
		{Name: "archive", Description: "dated archive", Path: filepath.Join(root, "archive", "{yyyy}", "{mm}")},
	}
}

func pipeline(t *testing.T, model core.Provider, typ, root string) Pipeline {
	t.Helper()
	c, _ := Lookup(typ)
	p, err := New(model, "DESCRIPTION-X", c, targets(root))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// One request carries the composed prompt, the image and the mv tool whose
// target is an enum of the task's target names.
func TestRequestCarriesPromptImageAndMvTool(t *testing.T) {
	source, root := stage(t)
	model := &stubModel{}
	if _, err := pipeline(t, model, TYPE_DATE, root).Process(context.Background(), source, false); err != nil {
		t.Fatal(err)
	}
	parts := model.seen.Messages[0].Parts
	prompt := parts[0].Text
	for _, want := range []string{"DESCRIPTION-X", "special: kept documents", "archive: dated archive", `"date" as YYYY-MM-DD`, "sidecar"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
	if parts[1].Kind != core.PART_KIND_IMAGE {
		t.Fatal("image part missing")
	}
	if len(model.seen.Tools) != 1 || model.seen.Tools[0].Name != TOOL_NAME {
		t.Fatalf("tools %+v, want one mv", model.seen.Tools)
	}
	params, _ := json.Marshal(model.seen.Tools[0].Parameters)
	if !strings.Contains(string(params), `"enum":["special","archive"]`) {
		t.Fatalf("mv schema lacks the target enum: %s", params)
	}
}

// A date task moves the image, keeping its name, into the dated folder with
// the metadata as sidecar.
func TestDateMovesImageWithSidecar(t *testing.T) {
	source, root := stage(t)
	model := &stubModel{calls: mvCall(map[string]any{"target": "archive", "metadata": map[string]any{"date": "115-08-01", "merchant": "Ah Luck Six"}})}
	got, err := pipeline(t, model, TYPE_DATE, root).Process(context.Background(), source, true)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "archive", "2026", "08")
	if !got.Filed || got.Destination != filepath.Join(dir, "IMG_1.jpg") {
		t.Fatalf("got %+v", got)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "IMG_1.json"))
	if err != nil {
		t.Fatalf("sidecar missing: %v", err)
	}
	var sidecar map[string]any
	if err := json.Unmarshal(raw, &sidecar); err != nil {
		t.Fatal(err)
	}
	if sidecar["date"] != "2026-08-01" || sidecar["merchant"] != "Ah Luck Six" || sidecar["original_filename"] != "IMG_1.jpg" {
		t.Fatalf("sidecar %v", sidecar)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("source should be gone after apply")
	}
}

// A classify task moves the original only, never a sidecar.
func TestClassifyMovesOriginalOnly(t *testing.T) {
	source, root := stage(t)
	model := &stubModel{calls: mvCall(map[string]any{"target": "special", "metadata": map[string]any{"category": "letter"}})}
	if _, err := pipeline(t, model, TYPE_CLASSIFY, root).Process(context.Background(), source, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "special", "IMG_1.jpg")); err != nil {
		t.Fatalf("image not moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "special", "IMG_1.json")); !os.IsNotExist(err) {
		t.Fatal("classify must not write a sidecar")
	}
}

// Dry-run, no tool call, and a rejected call all leave the image in place.
func TestImageStaysUnlessFiled(t *testing.T) {
	cases := map[string]struct {
		typ     string
		model   *stubModel
		apply   bool
		wantErr string
	}{
		"dry-run":           {TYPE_DATE, &stubModel{calls: mvCall(map[string]any{"target": "archive", "metadata": map[string]any{"date": "2026-08-01"}})}, false, ""},
		"no call":           {TYPE_DATE, &stubModel{text: "not a document"}, true, ""},
		"missing date":      {TYPE_DATE, &stubModel{calls: mvCall(map[string]any{"target": "archive", "metadata": map[string]any{"merchant": "x"}})}, true, "date must use YYYY-MM-DD"},
		"unknown target":    {TYPE_CLASSIFY, &stubModel{calls: mvCall(map[string]any{"target": "elsewhere"})}, true, "unknown target"},
		"no target arg":     {TYPE_CLASSIFY, &stubModel{calls: mvCall(map[string]any{})}, true, "target"},
		"dated by classify": {TYPE_CLASSIFY, &stubModel{calls: mvCall(map[string]any{"target": "archive"})}, true, "needs a dated result"},
		"two calls":         {TYPE_CLASSIFY, &stubModel{calls: append(mvCall(map[string]any{"target": "special"}), mvCall(map[string]any{"target": "special"})...)}, true, "2 times"},
	}
	for name, c := range cases {
		source, root := stage(t)
		got, err := pipeline(t, c.model, c.typ, root).Process(context.Background(), source, c.apply)
		if c.wantErr == "" && err != nil {
			t.Errorf("%s: unexpected error %v", name, err)
		}
		if c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)) {
			t.Errorf("%s: got %v, want %q", name, err, c.wantErr)
		}
		if name == "no call" && (got.Filed || got.Reply != "not a document") {
			t.Errorf("no call: got %+v", got)
		}
		if _, err := os.Stat(source); err != nil {
			t.Errorf("%s: image must stay: %v", name, err)
		}
	}
}
