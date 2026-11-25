package task

import (
	"fmt"
	"strings"

	"github.com/bizshuk/ocr_agent/svc/target"
)

// Metadata formats the prompt states per contract.
const (
	METADATA_DATED = `a JSON object with the fields the instructions above ask for. It MUST include "date" as YYYY-MM-DD (the document's own date; a Taiwan ROC year such as 115-02-23 is accepted). Singapore, Taiwan, Hong Kong and Malaysia print dates day first (DD/MM/YYYY): "12/01/2026" is 12 January 2026, not 1 December 2026. If the document prints no date at all, answer "N/A" and the image is filed under the photo's own date instead. It is saved next to the image as its JSON sidecar.`
	METADATA_PLAIN = `a short JSON object describing the document, for example {"category": "...", "summary": "..."}. It is only logged.`
)

// Prompt composes the one instruction sent with each image: the task's
// description, then how to answer — call mv once with a target name and
// the metadata this contract needs.
func Prompt(description string, c Contract, targets []target.Target) string {
	metadata := METADATA_PLAIN
	if c.Dated {
		metadata = METADATA_DATED
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(description))
	fmt.Fprintf(&b, "\n\nAnswer by calling the %s tool exactly once with:\n", TOOL_NAME)
	b.WriteString("- target: the folder this document belongs in, one of:\n")
	for _, t := range targets {
		fmt.Fprintf(&b, "  - %s: %s\n", t.Name, t.Description)
	}
	fmt.Fprintf(&b, "- metadata: %s\n\n", metadata)
	fmt.Fprintf(&b, "If no target fits, do not call %s; reply with one sentence saying why.", TOOL_NAME)
	return b.String()
}
