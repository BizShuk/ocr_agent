package cmd

import (
	cmdinternal "github.com/bizshuk/ocr_agent/cmd/internal"
	"github.com/spf13/cobra"
)

// newTypeCmd builds the subcommand of one task type. It processes exactly
// one configured task of that type, picked by --source; it never walks the
// task list (that is `run`).
func newTypeCmd(typ, short, long string) *cobra.Command {
	var (
		o      cmdinternal.RunOptions
		source string
	)
	c := &cobra.Command{
		Use:   typ,
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			t, err := cmdinternal.FindTask(typ, source)
			if err != nil {
				return err
			}
			return cmdinternal.RunTask(c, t, o)
		},
	}
	f := c.Flags()
	f.StringVar(&source, "source", "", "source folder of the "+typ+" task to run (optional when only one "+typ+" task is configured)")
	f.StringVar(&o.File, "file", "", "process a single image instead of the task source")
	cmdinternal.BindApply(f, &o)
	return c
}
