// Package vision sends one prompt plus one image to a vision-capable model
// and hands back the raw reply. Every reply is untrusted text; callers parse
// and validate it.
package vision

import (
	"context"
	"errors"
	"fmt"

	"github.com/bizshuk/agentsdk/core"
	"github.com/bizshuk/ocr_agent/svc/imagefile"
)

// DEFAULT_MAX_TOKENS caps a reply when the caller passes no budget.
const DEFAULT_MAX_TOKENS = 4096

// Ask sends prompt and img as one user message and returns the reply text.
func Ask(ctx context.Context, provider core.Provider, prompt string, img imagefile.Image, maxTokens int) (string, error) {
	if provider == nil {
		return "", errors.New("vision model has no provider")
	}
	if maxTokens <= 0 {
		maxTokens = DEFAULT_MAX_TOKENS
	}
	result, err := provider.Generate(ctx, core.ModelRequest{
		MaxTokens: maxTokens,
		Messages: []core.Message{{
			Role: core.ROLE_USER,
			Parts: []core.Part{
				{Kind: core.PART_KIND_PLAIN_TEXT, Text: prompt},
				{Kind: core.PART_KIND_IMAGE, Image: img.Data, ImageMIME: img.MIME},
			},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("model request: %w", err)
	}
	return result.Text, nil
}
