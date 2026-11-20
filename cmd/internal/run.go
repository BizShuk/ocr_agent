package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/bizshuk/ocr_agent/svc/task"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// RunOptions are the per-run flags.
type RunOptions struct {
	Apply bool   // move files; false is a dry-run that still calls the model
	File  string // process this one image instead of the task source
}

// BindApply adds --apply.
func BindApply(f *pflag.FlagSet, o *RunOptions) {
	f.BoolVar(&o.Apply, "apply", false, "actually move files (default is a dry-run)")
}

// RunEach runs every task in order. A failed task does not stop the
// remaining ones; the joined error makes the command exit non-zero.
func RunEach(c *cobra.Command, tasks []Task, o RunOptions) error {
	var failures []error
	for _, t := range tasks {
		if err := RunTask(c, t, o); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", t.Name, err))
		}
	}
	return errors.Join(failures...)
}

// RunTask is one task end to end: list its images, build its provider and
// pipeline once, then process each image. Progress goes to stdout, per-image
// failures to stderr.
func RunTask(c *cobra.Command, t Task, o RunOptions) error {
	out, errOut := c.OutOrStdout(), c.ErrOrStderr()
	files, err := CollectFiles(t.Source, o.File, t.Contract.Recursive)
	if err != nil {
		return err
	}
	model, err := t.NewProvider()
	if err != nil {
		return err
	}
	pipeline, err := task.New(model, t.Description, t.Contract, t.Targets)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "=== %s | %d image(s) | %s -> %d target(s)\n", t, len(files), t.Source, len(t.Targets))

	var filed, skipped, failed int
	for _, file := range files {
		start := time.Now()
		result, err := pipeline.Process(c.Context(), file, o.Apply)
		elapsed := time.Since(start).Seconds()
		switch {
		case err != nil:
			fmt.Fprintf(errOut, "✗ %s: %s: %v\n", t.Name, filepath.Base(file), err)
			failed++
		case !result.Filed:
			fmt.Fprintf(out, "⊘ %s (%.1fs) no target chosen, left in place: %s\n", filepath.Base(file), elapsed, result.Reply)
			skipped++
		default:
			metadata, _ := json.Marshal(result.Metadata)
			fmt.Fprintf(out, "▶ %s (%.1fs) -> %s: %s\n  %s\n", filepath.Base(file), elapsed, result.Target, result.Destination, metadata)
			filed++
		}
	}

	mode := "dry-run, nothing moved; re-run with --apply"
	if o.Apply {
		mode = "applied"
	}
	fmt.Fprintf(out, "Filed: %d | Skipped: %d | Errors: %d (%s)\n\n", filed, skipped, failed, mode)
	return ErrFailed(failed)
}

// ErrFailed is the task-level error returned after per-file failures were
// already printed, so pm2 sees a non-zero exit.
func ErrFailed(n int) error {
	if n == 0 {
		return nil
	}
	return fmt.Errorf("%d image(s) failed", n)
}
