package runtime

import (
	"context"
	"fmt"

	"github.com/bogachenko/agentkit/core/llm"
)

type SemanticClassifierModel interface {
	GenerateSemanticClassifierOutput(ctx context.Context, messages []llm.Message) (llm.Message, error)
}

type SemanticRequestClassifier struct {
	Model SemanticClassifierModel
}

func (c SemanticRequestClassifier) Classify(ctx context.Context, input ClassifierInput) (ClassifierOutput, error) {
	if err := input.Validate(); err != nil {
		return ClassifierOutput{}, err
	}
	if c.Model == nil {
		return ClassifierOutput{}, fmt.Errorf("semantic request classifier model is required")
	}

	message, err := c.Model.GenerateSemanticClassifierOutput(ctx, BuildSemanticClassifierPrompt(input))
	if err != nil {
		return ClassifierOutput{}, err
	}
	output, err := ExtractSemanticClassifierOutput(message)
	if err != nil {
		return ClassifierOutput{}, err
	}
	return repairClassifierOutputForAvailableTools(input, output), nil
}
