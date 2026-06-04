package llm

import (
	"fmt"
	"strings"
)

// Request separates stable system instruction, history, and runtime context.
type Request struct {
	System         string
	Messages       []Message
	RuntimeContext []string
}

// Validation ensures model calls always receive meaningful structured input.
func (r Request) Validate() error {
	if strings.TrimSpace(r.System) == "" && len(r.Messages) == 0 {
		return fmt.Errorf("llm request requires system instruction or at least one message")
	}

	for i, msg := range r.Messages {
		if err := msg.Validate(); err != nil {
			return fmt.Errorf("message %d: %w", i, err)
		}
	}

	return nil
}
