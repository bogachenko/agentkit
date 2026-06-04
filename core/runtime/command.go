package runtime

import (
	"fmt"

	"github.com/bogachenko/agentkit/core/session"
)

// DecisionCommand gives orchestration one explicit decision-validation input without hiding state or approvals.
type DecisionCommand struct {
	SessionID session.ID
	State     State
	Decision  RouteDecision
	Approvals []Approval
}

// Validation blocks incomplete decision inputs before runtime mutates ledger or publishes events.
func (c DecisionCommand) Validate() error {
	if err := c.SessionID.Validate(); err != nil {
		return err
	}

	if err := c.State.Validate(); err != nil {
		return fmt.Errorf("decision command state: %w", err)
	}

	if err := c.Decision.Validate(); err != nil {
		return fmt.Errorf("decision command decision: %w", err)
	}

	for index, approval := range c.Approvals {
		if err := approval.Validate(); err != nil {
			return fmt.Errorf("decision command approval %d: %w", index, err)
		}
	}

	return nil
}
