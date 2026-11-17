// Package cmd holds the cobra commands. They parse flags, resolve the
// per-domain config and print; every filing decision lives in svc/.
package cmd

import (
	"context"

	gosdkcmd "github.com/bizshuk/gosdk/cmd"
	cmdinternal "github.com/bizshuk/ocr_agent/cmd/internal"
	receiptcmd "github.com/bizshuk/ocr_agent/cmd/receipt"
	"github.com/spf13/cobra"
)

// APP_NAME anchors the gosdk config directory: ~/.config/ocr_agent.
const APP_NAME = cmdinternal.APP_NAME

// RootCmd is the ocr_agent command tree.
var RootCmd = &cobra.Command{
	Use:   APP_NAME,
	Short: "Vision-model OCR agent that classifies and archives document images",
	Long: `ocr_agent reads document images, asks a vision model what they are,
and files them. Each domain (classify, general, medical) has its own config
block at domain.<name> choosing provider, model, source folder and target path.

Configuration is read from ./settings.json, ./.env and ~/.config/ocr_agent/,
with OS environment variables on top (DOMAIN_GENERAL_PROVIDER overrides
domain.general.provider).`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	RootCmd.AddCommand(
		gosdkcmd.ConfigCmd,
		ClassifyCmd,
		receiptcmd.GeneralCmd,
		receiptcmd.MedicalCmd,
		ProcessCmd,
		AnalyzeCmd,
	)
}

// ExecuteContext runs the command tree with ctx.
func ExecuteContext(ctx context.Context) error {
	return RootCmd.ExecuteContext(ctx)
}
