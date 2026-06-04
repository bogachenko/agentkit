package session

import (
	"fmt"

	"github.com/bogachenko/agentkit/core/llm"
)

// CompactionInput makes history compaction depend only on explicit session data.
type CompactionInput struct {
	PreviousSummary string
	Messages        []llm.Message
}

// Validation blocks malformed conversation history before probabilistic compaction is called.
func (i CompactionInput) Validate() error {
	for index, message := range i.Messages {
		if err := message.Validate(); err != nil {
			return fmt.Errorf("compaction message %d: %w", index, err)
		}
	}

	return nil
}

// CompactionResult makes rewritten history and summary explicit and auditable.
type CompactionResult struct {
	Changed            bool
	Summary            string
	Messages           []llm.Message
	LastCompactedIndex int
}

// Validation prevents invalid compacted history from replacing durable session context.
func (r CompactionResult) Validate() error {
	if r.LastCompactedIndex < 0 {
		return fmt.Errorf("last compacted index cannot be negative")
	}

	for index, message := range r.Messages {
		if err := message.Validate(); err != nil {
			return fmt.Errorf("compacted message %d: %w", index, err)
		}
	}

	return nil
}
