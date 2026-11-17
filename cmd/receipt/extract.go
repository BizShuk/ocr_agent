// Package receipt holds the receipt-extraction cobra commands (general and
// medical) and their shared extract runner.
package receipt

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/bizshuk/agentsdk/core"
	cmdinternal "github.com/bizshuk/ocr_agent/cmd/internal"
	svcreceipt "github.com/bizshuk/ocr_agent/svc/receipt"
	"github.com/spf13/cobra"
)

// ExtractOptions are the flags shared by the general and medical commands.
type ExtractOptions struct {
	Apply    bool
	File     string
	Override cmdinternal.ModelOverride
}

func bindExtractFlags(c *cobra.Command, o *ExtractOptions) {
	f := c.Flags()
	f.BoolVar(&o.Apply, "apply", false, "actually archive files (default is a dry-run)")
	f.StringVar(&o.File, "file", "", "process a single image instead of the domain source")
	cmdinternal.BindModelOverride(f, &o.Override)
}

// RunExtract is one extract domain end to end: read domain.<name>.source,
// validate each model reply, and archive into domain.<name>.target.
func RunExtract(c *cobra.Command, name string, newProcessor func(core.Provider) svcreceipt.Processor, o ExtractOptions) error {
	d, err := cmdinternal.LoadDomain(name, o.Override)
	if err != nil {
		return err
	}
	if err := d.RequireTarget(); err != nil {
		return err
	}
	files, err := cmdinternal.CollectFiles(d.Source, o.File, false)
	if err != nil {
		return err
	}
	model, err := d.NewProvider()
	if err != nil {
		return err
	}
	processor := newProcessor(model)

	out := c.OutOrStdout()
	fmt.Fprintf(out, "[%s] %d image(s) | %s -> %s\n\n", d, len(files), d.Source, d.Target)

	var processed, failed int
	for _, file := range files {
		fmt.Fprintf(out, "▶ %s\n", filepath.Base(file))
		start := time.Now()
		result, err := processor.Process(c.Context(), file, d.Target, o.Apply)
		if err != nil {
			fmt.Fprintf(out, "  ✗ %v\n", err)
			failed++
		} else {
			fmt.Fprintf(out, "  ✓ %s | %s\n", result.Date, result.Description)
			fmt.Fprintf(out, "    → %s\n", result.Destination)
			if result.Applied {
				fmt.Fprintf(out, "    ✔ moved + %s\n", result.Sidecar)
			}
			processed++
		}
		fmt.Fprintf(out, "  ⏱ %.1fs\n", time.Since(start).Seconds())
	}

	fmt.Fprintf(out, "\nProcessed: %d  |  Errors: %d\n", processed, failed)
	if !o.Apply && processed > 0 {
		fmt.Fprintln(out, "\n⚠ Dry-run — nothing was moved. Re-run with --apply.")
	}
	return cmdinternal.ErrFailed(failed)
}
