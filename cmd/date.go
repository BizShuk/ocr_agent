package cmd

import "github.com/bizshuk/ocr_agent/svc/task"

// DateCmd runs date tasks.
var DateCmd = newTypeCmd(task.TYPE_DATE,
	"Run date tasks: move each image and its JSON sidecar into a dated folder",
	`date runs the one tasks[] entry of type date whose source is --source. Each
image in the task source (not recursive) goes to the task's model with a prompt
built from the task description and its targets; the model answers by calling
the mv tool with one target and the image metadata, which must carry a "date"
(YYYY-MM-DD, ROC years converted, no future dates). {yyyy}/{mm} in the target
path come from that date, and the image moves there with the metadata as its
.json sidecar. An image for which the model calls no tool stays where it is.

The default is a dry-run that calls the model but moves nothing.

Examples:

  ocr_agent date --source ~/projects/default/pkg/receipts/medical/tmp --apply
  ocr_agent date --source ~/projects/default/pkg/receipts/general/tmp --file ~/Downloads/IMG_1260.JPG`)
