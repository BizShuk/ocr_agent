package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	cmdinternal "github.com/bizshuk/ocr_agent/cmd/internal"
	"github.com/bizshuk/ocr_agent/svc/classify"
	"github.com/bizshuk/ocr_agent/svc/filing"
	"github.com/bizshuk/ocr_agent/svc/imagefile"
	"github.com/spf13/cobra"
)

type classifyOptions struct {
	apply    bool
	file     string
	override cmdinternal.ModelOverride
}

var classifyFlags classifyOptions

// ClassifyCmd routes images from the classify source into the general or
// medical domain's source folder.
var ClassifyCmd = &cobra.Command{
	Use:   "classify",
	Short: "Route images into the general or medical domain",
	Long: `classify shows every supported image anywhere beneath domain.classify.source
to domain.classify's model, which answers general_receipt, medical_receipt or
non_receipt. A receipt moves into domain.general.source or domain.medical.source;
a non_receipt stays where it is.

The default is a dry-run that moves nothing — pass --apply to act.

Examples:

  ocr_agent classify
  ocr_agent classify --apply
  ocr_agent classify --file ~/Downloads/IMG_1260.HEIC
  ocr_agent classify --provider ollama --model qwen2.5vl:3b`,
	RunE: func(c *cobra.Command, _ []string) error {
		return runClassify(c, classifyFlags)
	},
}

func init() {
	f := ClassifyCmd.Flags()
	f.BoolVar(&classifyFlags.apply, "apply", false, "actually move files (default is a dry-run)")
	f.StringVar(&classifyFlags.file, "file", "", "classify a single image instead of the whole source")
	cmdinternal.BindModelOverride(f, &classifyFlags.override)
}

func runClassify(c *cobra.Command, o classifyOptions) error {
	d, err := cmdinternal.LoadDomain(cmdinternal.DOMAIN_CLASSIFY, o.override)
	if err != nil {
		return err
	}
	routes, err := classifyRoutes(d.Source)
	if err != nil {
		return err
	}
	files, err := cmdinternal.CollectFiles(d.Source, o.file, true)
	if err != nil {
		return err
	}
	model, err := d.NewProvider()
	if err != nil {
		return err
	}
	classifier := classify.Classifier{Provider: model}

	out := c.OutOrStdout()
	fmt.Fprintf(out, "[%s] %d image(s) | %s\n\n", d, len(files), d.Source)

	var moved, skipped, failed int
	for _, file := range files {
		fmt.Fprintf(out, "▶ %s\n", filepath.Base(file))
		start := time.Now()
		result, err := classifyOne(c.Context(), classifier, routes, file, o.apply)
		switch {
		case err != nil:
			fmt.Fprintf(out, "  ✗ %v\n", err)
			failed++
		case result == "":
			fmt.Fprintf(out, "  ⊘ non_receipt — left in place\n")
			skipped++
		default:
			fmt.Fprintf(out, "  → %s\n", result)
			moved++
		}
		fmt.Fprintf(out, "  ⏱ %.1fs\n", time.Since(start).Seconds())
	}

	fmt.Fprintf(out, "\nMoved: %d  |  Skipped: %d  |  Errors: %d\n", moved, skipped, failed)
	if !o.apply && moved > 0 {
		fmt.Fprintln(out, "\n⚠ Dry-run — nothing was moved. Re-run with --apply.")
	}
	return cmdinternal.ErrFailed(failed)
}

// classifyRoutes maps each receipt verdict to the source folder of the
// domain that extracts it. A route folder inside the classify source would
// be walked again on the next run and re-classified, so that is refused.
func classifyRoutes(classifySource string) (map[classify.Category]string, error) {
	routes := map[classify.Category]string{}
	for category, name := range map[classify.Category]string{
		classify.CATEGORY_GENERAL: cmdinternal.DOMAIN_GENERAL,
		classify.CATEGORY_MEDICAL: cmdinternal.DOMAIN_MEDICAL,
	} {
		d, err := cmdinternal.LoadDomain(name, cmdinternal.ModelOverride{})
		if err != nil {
			return nil, err
		}
		inside, err := filing.Within(classifySource, d.Source)
		if err != nil {
			return nil, err
		}
		if inside {
			return nil, fmt.Errorf("%s (%s) must not lie inside %s (%s)",
				cmdinternal.DomainKey(name, cmdinternal.FIELD_SOURCE), d.Source, cmdinternal.DomainKey(cmdinternal.DOMAIN_CLASSIFY, cmdinternal.FIELD_SOURCE), classifySource)
		}
		routes[category] = d.Source
	}
	return routes, nil
}

// classifyOne handles a single image end to end. It returns the
// destination description on a move, "" for a non_receipt, and an error
// that fails only this file — the batch keeps going.
func classifyOne(ctx context.Context, c classify.Classifier, routes map[classify.Category]string, file string, apply bool) (string, error) {
	img, err := imagefile.Load(ctx, file)
	if err != nil {
		return "", err
	}
	category, err := c.Classify(ctx, img)
	if err != nil {
		return "", err
	}
	dir, ok := routes[category]
	if !ok {
		return "", nil
	}
	dest, err := filing.ChooseDestination(dir, filepath.Base(file))
	if err != nil {
		return "", err
	}
	desc := fmt.Sprintf("%s -> %s", category, dest)
	if !apply {
		return desc, nil
	}
	if err := filing.Move(file, dest); err != nil {
		return "", err
	}
	return desc + " ✔", nil
}
