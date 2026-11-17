package cmd

import (
	"errors"
	"fmt"

	cmdinternal "github.com/bizshuk/ocr_agent/cmd/internal"
	receiptcmd "github.com/bizshuk/ocr_agent/cmd/receipt"
	svcreceipt "github.com/bizshuk/ocr_agent/svc/receipt"
	"github.com/spf13/cobra"
)

var processApply bool

// ProcessCmd runs classify, general and medical in order, each with its own
// domain config.
var ProcessCmd = &cobra.Command{
	Use:   "process",
	Short: "Run classify, general and medical in order",
	Long: `process runs classify, general and medical in order, each domain with its
own provider and folders from config. A failed domain does not stop the
remaining ones; the command exits non-zero when any domain fails and stays a
dry-run unless --apply is provided.`,
	RunE: runProcess,
}

func init() {
	ProcessCmd.Flags().BoolVar(&processApply, "apply", false, "apply every domain (default runs every domain as a dry-run)")
}

func runProcess(c *cobra.Command, _ []string) error {
	out := c.OutOrStdout()
	var failures []error

	fmt.Fprintln(out, "=== classify ===")
	if err := runClassify(c, classifyOptions{apply: processApply}); err != nil {
		failures = append(failures, fmt.Errorf("classify: %w", err))
	}

	fmt.Fprintln(out, "\n=== general ===")
	if err := receiptcmd.RunExtract(c, cmdinternal.DOMAIN_GENERAL, svcreceipt.NewGeneralProcessor, receiptcmd.ExtractOptions{Apply: processApply}); err != nil {
		failures = append(failures, fmt.Errorf("general: %w", err))
	}

	fmt.Fprintln(out, "\n=== medical ===")
	if err := receiptcmd.RunExtract(c, cmdinternal.DOMAIN_MEDICAL, svcreceipt.NewMedicalProcessor, receiptcmd.ExtractOptions{Apply: processApply}); err != nil {
		failures = append(failures, fmt.Errorf("medical: %w", err))
	}

	if len(failures) > 0 {
		return fmt.Errorf("pipeline failed: %w", errors.Join(failures...))
	}
	return nil
}
