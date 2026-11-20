// Package cmd holds the cobra commands. They parse flags, resolve the
// task config and print; every filing decision lives in svc/.
package cmd

import (
	"context"

	gosdkcmd "github.com/bizshuk/gosdk/cmd"
	cmdinternal "github.com/bizshuk/ocr_agent/cmd/internal"
	"github.com/spf13/cobra"
)

// APP_NAME anchors the gosdk config directory: ~/.config/ocr_agent.
const APP_NAME = cmdinternal.APP_NAME

// RootCmd is the ocr_agent command tree.
var RootCmd = &cobra.Command{
	Use:   APP_NAME,
	Short: "Vision-model OCR agent that classifies and archives document images",
	Long: `ocr_agent reads document images, asks a vision model what they are,
and files them. Configuration is a tasks[] list: each task has a name, a
description, a type (classify or date — one subcommand each), a provider and
model, a timeout, a source folder, and the targets the model may choose from.

Configuration is read from ./settings.json, ./.env and ~/.config/ocr_agent/.
Provider credentials such as MINIMAX_API_KEY come from .env or the environment.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	RootCmd.AddCommand(
		gosdkcmd.ConfigCmd,
		ClassifyCmd,
		DateCmd,
		RunCmd,
	)
}

// ExecuteContext runs the command tree with ctx.
func ExecuteContext(ctx context.Context) error {
	return RootCmd.ExecuteContext(ctx)
}
