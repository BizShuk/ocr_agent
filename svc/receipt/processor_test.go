package receipt

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bizshuk/agentsdk/core"
)

// The archive layout is <category>/<YYYY>/<MM>/, not a single <YYYY-MM>
// folder. MM keeps its leading zero so the folders sort chronologically.
func TestArchiveSubdirNestsYearAndMonth(t *testing.T) {
	cases := []struct {
		date string
		want string
	}{
		{"2026-08-04", filepath.Join("2026", "08")},
		{"2026-12-31", filepath.Join("2026", "12")},
		{"2016-04-01", filepath.Join("2016", "04")},
	}
	for _, c := range cases {
		if got := archiveSubdir(c.date); got != c.want {
			t.Errorf("archiveSubdir(%q) = %q, want %q", c.date, got, c.want)
		}
	}
}

type stubProvider struct{ text string }

func (s stubProvider) Generate(context.Context, core.ModelRequest) (core.ModelResult, error) {
	return core.ModelResult{Text: s.text}, nil
}

const generalReply = `{"category":"general_receipt","date":"2026-08-04","merchant":"Ah Luck Six","currency":"SGD","total_amount":2.5}`

// A dry-run validates and names the archive path but leaves the source alone.
func TestProcessDryRunLeavesSource(t *testing.T) {
	source, target := stageReceipt(t)
	got, err := NewGeneralProcessor(stubProvider{generalReply}).Process(context.Background(), source, target, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join("2026", "08", "2026-08-04_ah_luck_six_sgd2.50.jpg"); got.Destination != want {
		t.Fatalf("destination %q, want %q", got.Destination, want)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("dry-run must keep the source: %v", err)
	}
}

// Apply moves the image under <target>/<YYYY>/<MM>/ and writes its sidecar.
func TestProcessApplyArchivesWithSidecar(t *testing.T) {
	source, target := stageReceipt(t)
	got, err := NewGeneralProcessor(stubProvider{generalReply}).Process(context.Background(), source, target, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, got.Destination)); err != nil {
		t.Fatalf("archived image missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "2026", "08", got.Sidecar)); err != nil {
		t.Fatalf("sidecar missing: %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatal("source should be gone after apply")
	}
}

func stageReceipt(t *testing.T) (source, target string) {
	t.Helper()
	dir := t.TempDir()
	source = filepath.Join(dir, "IMG_1.jpg")
	if err := os.WriteFile(source, []byte{0xff, 0xd8}, 0o600); err != nil {
		t.Fatal(err)
	}
	return source, filepath.Join(dir, "general")
}
