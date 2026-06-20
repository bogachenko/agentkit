package runtime

import (
	"context"
	"fmt"

	coresession "github.com/bogachenko/agentkit/core/session"
)

type SemanticMemorySnapshot struct {
	RunLedger  *RunLedger
	ActiveTask ActiveTaskState
}

func (s SemanticMemorySnapshot) IsZero() bool {
	return (s.RunLedger == nil || s.RunLedger.IsZero()) && s.ActiveTask.IsZero()
}

type SemanticMemoryStore interface {
	LoadSemanticMemory(ctx context.Context, sessionID coresession.ID) (SemanticMemorySnapshot, error)
	SaveSemanticMemory(ctx context.Context, sessionID coresession.ID, snapshot SemanticMemorySnapshot) error
}

func validateSemanticMemorySessionID(sessionID coresession.ID) error {
	if err := sessionID.Validate(); err != nil {
		return fmt.Errorf("semantic memory session id is required: %w", err)
	}
	return nil
}
