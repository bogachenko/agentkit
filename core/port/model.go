package port

import (
	"context"

	"github.com/bogachenko/agentkit/core/llm"
)

// Model hides concrete LLM providers behind provider-neutral request contract.
type Model interface {
	Generate(ctx context.Context, request llm.Request) (llm.Message, error)
}
