package receipt

import (
	cmdinternal "github.com/bizshuk/ocr_agent/cmd/internal"
	svcreceipt "github.com/bizshuk/ocr_agent/svc/receipt"
	"github.com/spf13/cobra"
)

var medicalFlags ExtractOptions

// MedicalCmd extracts metadata from medical receipts and MCs and archives them.
var MedicalCmd = &cobra.Command{
	Use:   "medical",
	Short: "Extract and archive medical receipts and certificates",
	Long: `medical reads domain.medical.source, validates the model's metadata and
archives each image as <target>/<YYYY>/<MM>/<date>_<clinic>_<currency><amount>.<ext>
(or <date>_<clinic>_mc.<ext> for a medical certificate) with a JSON sidecar.
It is a dry-run unless --apply is provided.`,
	RunE: func(c *cobra.Command, _ []string) error {
		return RunExtract(c, cmdinternal.DOMAIN_MEDICAL, svcreceipt.NewMedicalProcessor, medicalFlags)
	},
}

func init() {
	bindExtractFlags(MedicalCmd, &medicalFlags)
}
