package session

import (
	"fmt"

	"github.com/bogachenko/agentkit/core/llm"
)

// WorkSummaryInput keeps user-visible work reporting based only on recorded session evidence.
type WorkSummaryInput struct {
	Messages []llm.Message
}

// Validation prevents work summaries from being generated without explicit conversation evidence.
func (i WorkSummaryInput) Validate() error {
	if len(i.Messages) == 0 {
		return fmt.Errorf("work summary input requires at least one message")
	}

	for index, message := range i.Messages {
		if err := message.Validate(); err != nil {
			return fmt.Errorf("work summary message %d: %w", index, err)
		}
	}

	return nil
}
