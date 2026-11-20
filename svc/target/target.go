// Package target holds the destination folders a task offers its model. The
// model picks one by name through the mv tool; it never sees or writes a
// path.
package target

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bizshuk/ocr_agent/svc/filing"
)

// Path placeholders filled from a dated prompt result (YYYY-MM-DD).
const (
	PLACEHOLDER_YEAR  = "{yyyy}"
	PLACEHOLDER_MONTH = "{mm}"
)

// Target is one destination folder. Path is absolute and may contain the
// {yyyy} / {mm} placeholders, which only a dated result can fill.
type Target struct {
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description"`
	Path        string `mapstructure:"path"`
}

// Validate rejects a list the model could not choose from unambiguously.
func Validate(targets []Target) error {
	if len(targets) == 0 {
		return errors.New("no targets configured")
	}
	seen := map[string]bool{}
	for i, t := range targets {
		switch {
		case t.Name == "":
			return fmt.Errorf("target %d: name is empty", i)
		case seen[t.Name]:
			return fmt.Errorf("target %q: duplicate name", t.Name)
		case strings.TrimSpace(t.Description) == "":
			return fmt.Errorf("target %q: description is empty; the model chooses by it", t.Name)
		case !filepath.IsAbs(t.Path):
			return fmt.Errorf("target %q: path must be absolute, got %q", t.Name, t.Path)
		}
		seen[t.Name] = true
	}
	return nil
}

// Find returns the target called name.
func Find(targets []Target, name string) (Target, bool) {
	for _, t := range targets {
		if t.Name == name {
			return t, true
		}
	}
	return Target{}, false
}

// Root is the fixed part of Path, up to the first placeholder segment. A
// resolved folder always lies inside it.
func (t Target) Root() string {
	segments := strings.Split(t.Path, string(filepath.Separator))
	for i, s := range segments {
		if strings.Contains(s, "{") {
			return filepath.Join(string(filepath.Separator), filepath.Join(segments[:i]...))
		}
	}
	return t.Path
}

// Resolve fills the placeholders from date (YYYY-MM-DD, empty when the
// result has none) and returns the folder for a file currently at source.
// Choosing the folder the file already sits in is refused: that file would
// be picked up again by the next run of the same domain.
func (t Target) Resolve(date, source string) (string, error) {
	dir := t.Path
	if strings.Contains(dir, PLACEHOLDER_YEAR) || strings.Contains(dir, PLACEHOLDER_MONTH) {
		if len(date) < len("YYYY-MM") {
			return "", fmt.Errorf("target %q needs a dated result for %s", t.Name, t.Path)
		}
		dir = strings.ReplaceAll(dir, PLACEHOLDER_YEAR, date[:4])
		dir = strings.ReplaceAll(dir, PLACEHOLDER_MONTH, date[5:7])
	}
	if err := filing.EnsureWithin(t.Root(), dir); err != nil {
		return "", err
	}
	if filepath.Clean(dir) == filepath.Dir(source) {
		return "", fmt.Errorf("target %q is the file's current folder", t.Name)
	}
	return dir, nil
}
