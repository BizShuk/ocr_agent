package target

import (
	"path/filepath"
	"strings"
	"testing"
)

var testTargets = []Target{
	{Name: "general_inbox", Description: "ordinary receipts awaiting extraction", Path: "/data/general/tmp"},
	{Name: "general_archive", Description: "extracted ordinary receipts", Path: "/data/general/{yyyy}/{mm}"},
}

func TestValidateRejectsAmbiguousLists(t *testing.T) {
	cases := map[string][]Target{
		"empty":     nil,
		"no name":   {{Description: "d", Path: "/a"}},
		"duplicate": {{Name: "a", Description: "d", Path: "/a"}, {Name: "a", Description: "d", Path: "/b"}},
		"no desc":   {{Name: "a", Path: "/a"}},
		"relative":  {{Name: "a", Description: "d", Path: "rel"}},
	}
	for name, targets := range cases {
		if err := Validate(targets); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if err := Validate(testTargets); err != nil {
		t.Fatalf("valid list rejected: %v", err)
	}
}

func TestResolveFillsDatePlaceholders(t *testing.T) {
	got, err := testTargets[1].Resolve("2026-08-04", "/data/general/tmp/a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/data/general", "2026", "08"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if root := testTargets[1].Root(); root != "/data/general" {
		t.Fatalf("root %q, want /data/general", root)
	}
}

// A dated target cannot take a result without a date (classify has none).
func TestResolveNeedsDateForPlaceholders(t *testing.T) {
	_, err := testTargets[1].Resolve("", "/inbox/a.jpg")
	if err == nil || !strings.Contains(err.Error(), "needs a dated result") {
		t.Fatalf("got %v, want a needs-date error", err)
	}
}

// Filing a file into the folder it already sits in would loop it through
// the same domain on the next run.
func TestResolveRefusesCurrentFolder(t *testing.T) {
	_, err := testTargets[0].Resolve("", "/data/general/tmp/a.jpg")
	if err == nil || !strings.Contains(err.Error(), "current folder") {
		t.Fatalf("got %v, want a current-folder error", err)
	}
}
