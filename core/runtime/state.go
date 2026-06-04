package runtime

import (
	"fmt"
	"time"
)

// State is the deterministic source of truth for a single runtime execution.
type State struct {
	RunID     RunID
	Status    RunStatus
	Steps     []Step
	Decision  *RouteDecision
	Failure   *Failure
	StartedAt time.Time
	UpdatedAt time.Time
}

// Validation prevents inconsistent runtime state from driving orchestration.
func (s State) Validate() error {
	if err := s.RunID.Validate(); err != nil {
		return err
	}

	if err := s.Status.Validate(); err != nil {
		return err
	}

	for index, step := range s.Steps {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("step %d: %w", index, err)
		}
	}

	switch s.Status {
	case RunStatusFailed, RunStatusBlocked:
		if s.Failure == nil {
			return fmt.Errorf("%s run %q requires failure", s.Status, string(s.RunID))
		}

		if err := s.Failure.Validate(); err != nil {
			return fmt.Errorf("run %q failure: %w", string(s.RunID), err)
		}

	default:
		if s.Failure != nil {
			return fmt.Errorf("%s run %q cannot include failure", s.Status, string(s.RunID))
		}
	}

	if s.Decision != nil {
		if err := s.Decision.Validate(); err != nil {
			return fmt.Errorf("route decision: %w", err)
		}
	}

	if !s.StartedAt.IsZero() && !s.UpdatedAt.IsZero() && s.UpdatedAt.Before(s.StartedAt) {
		return fmt.Errorf("run %q updated before it started", string(s.RunID))
	}

	return nil
}
