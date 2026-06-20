package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/bogachenko/agentkit/core/llm"
	"github.com/bogachenko/agentkit/core/port"
)

type SemanticClassifierModelAdapter struct {
	Model          port.Model
	SystemPrompt   string
	RuntimeContext []string
}

func (a SemanticClassifierModelAdapter) GenerateSemanticClassifierOutput(ctx context.Context, messages []llm.Message) (llm.Message, error) {
	if a.Model == nil {
		return llm.Message{}, fmt.Errorf("semantic classifier model adapter model is required")
	}
	if len(messages) == 0 {
		return llm.Message{}, fmt.Errorf("semantic classifier model adapter messages are required")
	}
	for i, message := range messages {
		if err := message.Validate(); err != nil {
			return llm.Message{}, fmt.Errorf("semantic classifier model adapter message %d: %w", i, err)
		}
	}

	system := strings.TrimSpace(a.SystemPrompt)
	if system == "" {
		system = defaultSemanticClassifierSystemPrompt()
	}

	return a.Model.Generate(ctx, llm.Request{
		System:         system,
		Messages:       messages,
		RuntimeContext: a.RuntimeContext,
	})
}

func defaultSemanticClassifierSystemPrompt() string {
	return "You are a strict semantic request classifier. Return only the requested classifier output. Do not answer the user directly."
}

func NewSemanticRequestClassifierFromModel(model port.Model) SemanticRequestClassifier {
	return SemanticRequestClassifier{
		Model: SemanticClassifierModelAdapter{Model: model},
	}
}
