package llm

import (
	"context"
	"iter"

	"google.golang.org/adk/model"
)

// WHY: NonStreamingModel enforces non-streaming upstream calls while preserving ADK model.LLM compatibility.
type NonStreamingModel struct {
	inner model.LLM
}

// WHY: NewNonStreamingModel keeps nil wrappers out of ADK model configuration.
func NewNonStreamingModel(inner model.LLM) model.LLM {
	if inner == nil {
		return nil
	}

	return &NonStreamingModel{
		inner: inner,
	}
}

// WHY: Name preserves the wrapped model identity for logs, tracing, and ADK behavior.
func (m *NonStreamingModel) Name() string {
	return m.inner.Name()
}

// WHY: GenerateContent forces stream=false so callers cannot accidentally use streaming through this wrapper.
func (m *NonStreamingModel) GenerateContent(ctx context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		for response, err := range m.inner.GenerateContent(ctx, req, false) {
			if !yield(response, err) {
				return
			}
		}
	}
}
