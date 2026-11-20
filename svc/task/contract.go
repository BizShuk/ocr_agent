// Package task runs one configured task over its images: a single model
// call sends the image with a prompt composed from the task description,
// its targets and the type's result format, and the model answers by
// calling the mv tool, which files the image.
package task

// Contract is what a task type fixes in code. Everything else — the
// description, model and folders — is the task's own config.
type Contract struct {
	Type      string
	Recursive bool // walk the source recursively (phone-sync inboxes nest folders)
	Dated     bool // metadata must carry a date; the image moves with a JSON sidecar into {yyyy}/{mm}
}

// Task types. Each is one subcommand.
const (
	TYPE_CLASSIFY = "classify"
	TYPE_DATE     = "date"
)

var contracts = []Contract{
	{Type: TYPE_CLASSIFY, Recursive: true},
	{Type: TYPE_DATE, Dated: true},
}

// Lookup returns the contract of a task type.
func Lookup(typ string) (Contract, bool) {
	for _, c := range contracts {
		if c.Type == typ {
			return c, true
		}
	}
	return Contract{}, false
}

// Types lists every task type in a stable order.
func Types() []string {
	types := make([]string, 0, len(contracts))
	for _, c := range contracts {
		types = append(types, c.Type)
	}
	return types
}
