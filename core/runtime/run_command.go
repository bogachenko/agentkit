package runtime

import (
	"fmt"
	"strings"

	"github.com/bogachenko/agentkit/core/llm"
	"github.com/bogachenko/agentkit/core/session"
)

// RunCommand is the explicit external input for one full harness run.
type RunCommand struct {
	RunID          RunID
	SessionID      session.ID
	System         string
	UserInput      string
	History        []llm.Message
	RuntimeContext []string
	MaxSteps       int
	Approvals      []Approval
}

// Validation prevents harness runs from starting with incomplete identity, input, or approval state.
func (c RunCommand) Validate() error {
	if err := c.RunID.Validate(); err != nil {
		return err
	}

	if err := c.SessionID.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(c.UserInput) == "" {
		return fmt.Errorf("run command user input is required")
	}

	if c.MaxSteps <= 0 {
		return fmt.Errorf("run command max steps must be positive")
	}

	for index, message := range c.History {
		if err := message.Validate(); err != nil {
			return fmt.Errorf("run command history message %d: %w", index, err)
		}
	}

	for index, approval := range c.Approvals {
		if err := approval.Validate(); err != nil {
			return fmt.Errorf("run command approval %d: %w", index, err)
		}

		if approval.RunID != c.RunID {
			return fmt.Errorf("run command approval %q run id does not match command run id", string(approval.ID))
		}
	}

	return nil
}
