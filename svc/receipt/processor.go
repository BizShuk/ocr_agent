package receipt

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/bizshuk/agentsdk/core"
	"github.com/bizshuk/ocr_agent/svc/classify"
	"github.com/bizshuk/ocr_agent/svc/filing"
	"github.com/bizshuk/ocr_agent/svc/imagefile"
	"github.com/bizshuk/ocr_agent/svc/vision"
)

// Processor extracts metadata from one image, validates it, and archives the
// image under <target>/<YYYY>/<MM>/ with a JSON sidecar.
type Processor struct {
	Provider core.Provider
	category classify.Category
	prompt   string
}

// Result describes the validated archive target for one receipt.
type Result struct {
	Date        string
	Description string
	Destination string // relative to the target directory
	Sidecar     string // base name
	Applied     bool
}

// Process validates the model output and archives source under target only
// when apply is true. A dry-run still calls the model and validates.
func (p Processor) Process(ctx context.Context, source, target string, apply bool) (Result, error) {
	img, err := imagefile.Load(ctx, source)
	if err != nil {
		return Result{}, err
	}
	raw, err := vision.Ask(ctx, p.Provider, p.prompt, img, vision.DEFAULT_MAX_TOKENS)
	if err != nil {
		return Result{}, err
	}
	metadata, err := parseReceiptMetadata(raw, p.category, filepath.Ext(source), time.Now())
	if err != nil {
		return Result{}, fmt.Errorf("validation: %w", err)
	}

	destination, err := filing.ChooseDestination(filepath.Join(target, archiveSubdir(metadata.date)), metadata.filename)
	if err != nil {
		return Result{}, err
	}
	if err := filing.EnsureWithin(target, destination); err != nil {
		return Result{}, err
	}
	relative, err := filepath.Rel(target, destination)
	if err != nil {
		relative = destination
	}
	result := Result{
		Date:        metadata.date,
		Description: metadata.description,
		Destination: relative,
		Sidecar:     filepath.Base(filing.SidecarPath(destination)),
		Applied:     apply,
	}
	if !apply {
		return result, nil
	}
	if err := filing.ArchiveWithSidecar(source, destination, metadata.values); err != nil {
		return Result{}, err
	}
	return result, nil
}

// archiveSubdir renders the nested year/month folder for a validated
// YYYY-MM-DD date, e.g. 2026-08-04 -> "2026/08". parseReceiptMetadata has
// already rejected any date that is not a real YYYY-MM-DD.
func archiveSubdir(date string) string {
	return filepath.Join(date[:4], date[5:7])
}
