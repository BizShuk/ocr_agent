package task

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/bizshuk/agentsdk/core"
	"github.com/bizshuk/agentsdk/tool"
	"github.com/bizshuk/ocr_agent/svc/date"
	"github.com/bizshuk/ocr_agent/svc/filing"
	"github.com/bizshuk/ocr_agent/svc/target"
)

// TOOL_NAME is the agentSDK tool the model calls to file the image.
const TOOL_NAME = "mv"

// MvArgs is what the model passes. The file to move is never an argument:
// it is always the image being processed.
type MvArgs struct {
	Target   string         `json:"target" jsonschema_description:"name of the target folder"`
	Metadata map[string]any `json:"metadata,omitempty" jsonschema_description:"the image metadata described in the prompt"`
}

// MvOutput is the tool result: where the image went (or would go in a
// dry-run) and the metadata as filed.
type MvOutput struct {
	Target      string         `json:"target"`
	Destination string         `json:"destination"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Sidecar     bool           `json:"sidecar,omitempty"`
}

// mvSpec reflects MvArgs and narrows target to the task's target names, so
// the model can only name a configured folder.
func mvSpec(targets []target.Target) (core.ToolSpec, error) {
	schema := tool.SchemaFor[MvArgs]()
	if def, ok := schema.Definitions[strings.TrimPrefix(schema.Ref, "#/$defs/")]; ok {
		schema = def
	}
	prop, ok := schema.Properties.Get("target")
	if !ok {
		return core.ToolSpec{}, fmt.Errorf("%s schema has no target property", TOOL_NAME)
	}
	for _, t := range targets {
		prop.Enum = append(prop.Enum, t.Name)
	}
	params, err := json.Marshal(schema)
	if err != nil {
		return core.ToolSpec{}, fmt.Errorf("%s schema: %w", TOOL_NAME, err)
	}
	return core.ToolSpec{
		Name:        TOOL_NAME,
		Description: "File the image being processed into one target folder, with its metadata.",
		Risk:        core.RISK_LEVEL_HIGH,
		Parameters:  json.RawMessage(params),
	}, nil
}

// mv is the agentSDK tool bound to one image. It resolves the chosen target,
// picks a collision-free name, and moves the image — with its metadata as
// a sidecar for a dated contract — only when apply is set.
type mv struct {
	spec     core.ToolSpec
	contract Contract
	targets  []target.Target
	source   string
	apply    bool
}

var _ core.Tool = (*mv)(nil)

func (m *mv) Name() string        { return TOOL_NAME }
func (m *mv) Spec() core.ToolSpec { return m.spec }

func (m *mv) Call(ctx context.Context, raw json.RawMessage) (core.ToolResult, error) {
	return tool.CallWithRawMessage(ctx, TOOL_NAME, raw, m.execute)
}

func (m *mv) execute(_ context.Context, a MvArgs) (MvOutput, error) {
	chosen, ok := target.Find(m.targets, a.Target)
	if !ok {
		return MvOutput{}, fmt.Errorf("unknown target %q", a.Target)
	}
	day := ""
	if m.contract.Dated {
		var err error
		if day, err = date.Parse(a.Metadata[date.FIELD], time.Now()); err != nil {
			return MvOutput{}, fmt.Errorf("metadata: %w", err)
		}
		a.Metadata[date.FIELD] = day
	}
	dir, err := chosen.Resolve(day, m.source)
	if err != nil {
		return MvOutput{}, err
	}
	destination, err := filing.ChooseDestination(dir, filepath.Base(m.source))
	if err != nil {
		return MvOutput{}, err
	}
	out := MvOutput{Target: chosen.Name, Destination: destination, Metadata: a.Metadata, Sidecar: m.contract.Dated}
	switch {
	case !m.apply:
	case m.contract.Dated:
		err = filing.ArchiveWithSidecar(m.source, destination, a.Metadata)
	default:
		err = filing.Move(m.source, destination)
	}
	return out, err
}
