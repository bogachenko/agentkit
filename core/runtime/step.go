package runtime

import (
	"fmt"
	"strings"
	"time"
)

// Step records one explicit runtime action without mixing it with LLM message history.
type Step struct {
	ID          StepID
	Source      StepSource
	Status      StepStatus
	Description string
	Failure     *Failure
	StartedAt   time.Time
	FinishedAt  time.Time
}

// Validation keeps runtime steps auditable and structurally safe for ledger storage.
func (s Step) Validate() error {
	if err := s.ID.Validate(); err != nil {
		return err
	}

	if err := s.Source.Validate(); err != nil {
		return err
	}

	if err := s.Status.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(s.Description) == "" {
		return fmt.Errorf("step description is required for %q", string(s.ID))
	}

	switch s.Status {
	case StepStatusFailed, StepStatusBlocked:
		if s.Failure == nil {
			return fmt.Errorf("%s step %q requires failure", s.Status, string(s.ID))
		}

		if err := s.Failure.Validate(); err != nil {
			return fmt.Errorf("step %q failure: %w", string(s.ID), err)
		}

	default:
		if s.Failure != nil {
			return fmt.Errorf("%s step %q cannot include failure", s.Status, string(s.ID))
		}
	}

	if !s.StartedAt.IsZero() && !s.FinishedAt.IsZero() && s.FinishedAt.Before(s.StartedAt) {
		return fmt.Errorf("step %q finished before it started", string(s.ID))
	}

	return nil
}
