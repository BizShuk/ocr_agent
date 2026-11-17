package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	cmdinternal "github.com/bizshuk/ocr_agent/cmd/internal"
	"github.com/bizshuk/ocr_agent/svc/classify"
	"github.com/bizshuk/ocr_agent/svc/imagefile"
	"github.com/bizshuk/ocr_agent/svc/receipt"
	"github.com/bizshuk/ocr_agent/svc/vision"
	"github.com/spf13/cobra"
)

const rule = "────────────────────────────────────────────────────────────"

// domainPrompts is the instruction each domain sends with an image.
var domainPrompts = map[string]string{
	cmdinternal.DOMAIN_CLASSIFY: classify.PROMPT,
	cmdinternal.DOMAIN_GENERAL:  receipt.GENERAL_PROMPT,
	cmdinternal.DOMAIN_MEDICAL:  receipt.MEDICAL_PROMPT,
}

var (
	analyzeFile     string
	analyzeOverride cmdinternal.ModelOverride
)

// AnalyzeCmd prints a domain's raw model reply without touching any file.
var AnalyzeCmd = &cobra.Command{
	Use:       "analyze <classify|general|medical>",
	Short:     "Print a domain's raw model reply for prompt tuning",
	ValidArgs: []string{cmdinternal.DOMAIN_CLASSIFY, cmdinternal.DOMAIN_GENERAL, cmdinternal.DOMAIN_MEDICAL},
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Long: `analyze sends images to a domain's configured model with that domain's
prompt and prints the raw reply. It never parses, renames, moves or writes files.

Examples:

  ocr_agent analyze general --file ~/Downloads/IMG_1260.JPG
  ocr_agent analyze medical --provider ollama --model qwen2.5vl:7b`,
	RunE: runAnalyze,
}

func init() {
	f := AnalyzeCmd.Flags()
	f.StringVar(&analyzeFile, "file", "", "analyze a single image instead of the domain source")
	cmdinternal.BindModelOverride(f, &analyzeOverride)
}

func runAnalyze(c *cobra.Command, args []string) error {
	name := args[0]
	d, err := cmdinternal.LoadDomain(name, analyzeOverride)
	if err != nil {
		return err
	}
	files, err := cmdinternal.CollectFiles(d.Source, analyzeFile, name == cmdinternal.DOMAIN_CLASSIFY)
	if err != nil {
		return err
	}
	model, err := d.NewProvider()
	if err != nil {
		return err
	}

	out := c.OutOrStdout()
	fmt.Fprintf(out, "[%s] %d image(s)\n%s\n", d, len(files), rule)
	failed := 0
	for _, file := range files {
		fmt.Fprintf(out, "\n▶ %s\n", filepath.Base(file))
		img, err := imagefile.Load(c.Context(), file)
		if err == nil {
			var reply string
			reply, err = vision.Ask(c.Context(), model, domainPrompts[name], img, vision.DEFAULT_MAX_TOKENS)
			if err == nil {
				fmt.Fprintln(out, strings.TrimSpace(reply))
			}
		}
		if err != nil {
			fmt.Fprintf(out, "  ⚠ %v\n", err)
			failed++
		}
		fmt.Fprintln(out, rule)
	}
	return cmdinternal.ErrFailed(failed)
}
