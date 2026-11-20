package cmd

import (
	cmdinternal "github.com/bizshuk/ocr_agent/cmd/internal"
	"github.com/spf13/cobra"
)

var runFlags cmdinternal.RunOptions

// RunCmd runs every task in config order, whatever its type.
var RunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run every task in tasks[] order",
	Long: `run executes every tasks[] entry in config order, each with its own
provider, model, source and targets. A failed task does not stop the remaining
ones; the command exits non-zero when any task fails and stays a dry-run unless
--apply is provided.`,
	RunE: func(c *cobra.Command, _ []string) error {
		tasks, err := cmdinternal.LoadTasks()
		if err != nil {
			return err
		}
		return cmdinternal.RunEach(c, tasks, runFlags)
	},
}

func init() {
	cmdinternal.BindApply(RunCmd.Flags(), &runFlags)
}
