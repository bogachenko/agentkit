package runtime

import (
	"fmt"

	"github.com/bogachenko/agentkit/core/llm"
)

// RunResult is the explicit terminal output of a full harness run.
type RunResult struct {
	RunID          RunID
	Status         RunStatus
	FinalMessage   *llm.Message
	Decision       *RouteDecision
	Failure        *Failure
	LedgerSummary  LedgerSummary
	StepsCompleted int
}

// Validation keeps successful, blocked, and failed run outputs structurally distinct.
func (r RunResult) Validate() error {
	if err := r.RunID.Validate(); err != nil {
		return err
	}

	if err := r.Status.Validate(); err != nil {
		return err
	}

	if r.StepsCompleted < 0 {
		return fmt.Errorf("run result steps completed cannot be negative")
	}

	switch r.Status {
	case RunStatusCompleted:
		if r.Failure != nil {
			return fmt.Errorf("completed run result cannot include failure")
		}

	case RunStatusBlocked, RunStatusFailed:
		if r.Failure == nil {
			return fmt.Errorf("%s run result requires failure", r.Status)
		}

		if err := r.Failure.Validate(); err != nil {
			return fmt.Errorf("run result failure: %w", err)
		}

	default:
		return fmt.Errorf("run result cannot use non-terminal status %q", string(r.Status))
	}

	if r.FinalMessage != nil {
		if err := r.FinalMessage.Validate(); err != nil {
			return fmt.Errorf("run result final message: %w", err)
		}
	}

	if r.Decision != nil {
		if err := r.Decision.Validate(); err != nil {
			return fmt.Errorf("run result decision: %w", err)
		}
	}

	return nil
}
