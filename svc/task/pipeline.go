package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bizshuk/agentsdk/core"
	"github.com/bizshuk/agentsdk/tool"
	"github.com/bizshuk/ocr_agent/svc/imagefile"
	"github.com/bizshuk/ocr_agent/svc/target"
)

// MAX_TOKENS caps the reply: one tool call carrying the metadata.
const MAX_TOKENS = 4096

// Pipeline processes images for one task. The prompt and the mv tool spec
// are built once; each image is one model call.
type Pipeline struct {
	provider core.Provider
	contract Contract
	targets  []target.Target
	prompt   string
	spec     core.ToolSpec
}

// Outcome is one image's result. When the model called no tool, Filed is
// false and Reply holds whatever it said instead.
type Outcome struct {
	Filed bool
	Reply string
	MvOutput
}

// New builds a pipeline. targets must already be validated.
func New(provider core.Provider, description string, c Contract, targets []target.Target) (Pipeline, error) {
	if provider == nil {
		return Pipeline{}, errors.New("task has no provider")
	}
	spec, err := mvSpec(targets)
	if err != nil {
		return Pipeline{}, err
	}
	return Pipeline{
		provider: provider,
		contract: c,
		targets:  targets,
		prompt:   Prompt(description, c, targets),
		spec:     spec,
	}, nil
}

// Process sends source with the prompt and the mv tool, then dispatches the
// model's mv call through an agentSDK registry. A dry-run (apply=false)
// still makes the call and resolves the destination; only the move is skipped.
func (p Pipeline) Process(ctx context.Context, source string, apply bool) (Outcome, error) {
	img, err := imagefile.Load(ctx, source)
	if err != nil {
		return Outcome{}, err
	}
	reply, err := p.provider.Generate(ctx, core.ModelRequest{
		MaxTokens: MAX_TOKENS,
		Tools:     []core.ToolSpec{p.spec},
		Messages: []core.Message{{
			Role: core.ROLE_USER,
			Parts: []core.Part{
				{Kind: core.PART_KIND_PLAIN_TEXT, Text: p.prompt},
				{Kind: core.PART_KIND_IMAGE, Image: img.Data, ImageMIME: img.MIME},
			},
		}},
	})
	if err != nil {
		return Outcome{}, fmt.Errorf("model request: %w", err)
	}
	reply = reply.NormalizeContent()

	var calls []core.ToolCall
	for _, call := range reply.ToolCalls {
		if call.Name == TOOL_NAME {
			calls = append(calls, call)
		}
	}
	switch len(calls) {
	case 0:
		return Outcome{Reply: reply.Text}, nil
	case 1:
	default:
		return Outcome{}, fmt.Errorf("model called %s %d times", TOOL_NAME, len(calls))
	}

	registry := tool.NewRegistry()
	registry.Register(&mv{spec: p.spec, contract: p.contract, targets: p.targets, source: source, apply: apply})
	result := registry.Call(ctx, calls[0])
	if !result.OK {
		return Outcome{}, fmt.Errorf("%s: %s", TOOL_NAME, result.Error)
	}
	raw, ok := result.Output.(json.RawMessage)
	if !ok {
		return Outcome{}, fmt.Errorf("%s returned %T", TOOL_NAME, result.Output)
	}
	out := Outcome{Filed: true}
	if err := json.Unmarshal(raw, &out.MvOutput); err != nil {
		return Outcome{}, fmt.Errorf("decode %s result: %w", TOOL_NAME, err)
	}
	return out, nil
}
