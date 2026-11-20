package cmd

import "github.com/bizshuk/ocr_agent/svc/task"

// ClassifyCmd runs classify tasks.
var ClassifyCmd = newTypeCmd(task.TYPE_CLASSIFY,
	"Run classify tasks: move each image into a model-chosen folder",
	`classify runs the one tasks[] entry of type classify whose source is --source.
Each image anywhere beneath the task source goes to the task's model with a
prompt built from the task description and its targets; the model answers by
calling the mv tool with one target, and the original image moves there
unchanged. An image for which the model calls no tool stays where it is.

The default is a dry-run that calls the model but moves nothing.

Examples:

  ocr_agent classify
  ocr_agent classify --source ~/projects/default/pkg/receipts/tmp --apply
  ocr_agent classify --file ~/Downloads/IMG_1260.HEIC`)
