package session

import (
	"fmt"

	"github.com/bogachenko/agentkit/core/llm"
)

// TitleInput keeps session title generation isolated from runtime and business logic.
type TitleInput struct {
	Messages []llm.Message
}

// Validation requires explicit conversation evidence before probabilistic title generation.
func (i TitleInput) Validate() error {
	if len(i.Messages) == 0 {
		return fmt.Errorf("title input requires at least one message")
	}

	for index, message := range i.Messages {
		if err := message.Validate(); err != nil {
			return fmt.Errorf("title message %d: %w", index, err)
		}
	}

	return nil
}
