package receipt

import (
	cmdinternal "github.com/bizshuk/ocr_agent/cmd/internal"
	svcreceipt "github.com/bizshuk/ocr_agent/svc/receipt"
	"github.com/spf13/cobra"
)

var generalFlags ExtractOptions

// GeneralCmd extracts metadata from general receipts and archives them.
var GeneralCmd = &cobra.Command{
	Use:   "general",
	Short: "Extract and archive general receipts",
	Long: `general reads domain.general.source, validates the model's metadata and
archives each image as <target>/<YYYY>/<MM>/<date>_<merchant>_<currency><amount>.<ext>
with a JSON sidecar. It is a dry-run unless --apply is provided.`,
	RunE: func(c *cobra.Command, _ []string) error {
		return RunExtract(c, cmdinternal.DOMAIN_GENERAL, svcreceipt.NewGeneralProcessor, generalFlags)
	},
}

func init() {
	bindExtractFlags(GeneralCmd, &generalFlags)
}
