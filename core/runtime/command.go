package runtime

import (
	"fmt"

	"github.com/bogachenko/agentkit/core/session"
)

// Command gives runtime one explicit input object without hiding state, decision, or approvals.
type Command struct {
	SessionID session.ID
	State     State
	Decision  RouteDecision
	Approvals []Approval
}

// Validation blocks incomplete orchestration inputs before runtime mutates ledger or publishes events.
func (c Command) Validate() error {
	if err := c.SessionID.Validate(); err != nil {
		return err
	}

	if err := c.State.Validate(); err != nil {
		return fmt.Errorf("command state: %w", err)
	}

	if err := c.Decision.Validate(); err != nil {
		return fmt.Errorf("command decision: %w", err)
	}

	for index, approval := range c.Approvals {
		if err := approval.Validate(); err != nil {
			return fmt.Errorf("command approval %d: %w", index, err)
		}
	}

	return nil
}
