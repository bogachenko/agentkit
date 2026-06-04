package runtime

import "fmt"

// RunStatus makes lifecycle transitions explicit instead of relying on booleans.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusBlocked   RunStatus = "blocked"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

// Validation blocks unknown run states before orchestration logic consumes them.
func (s RunStatus) Validate() error {
	switch s {
	case RunStatusPending, RunStatusRunning, RunStatusBlocked, RunStatusCompleted, RunStatusFailed:
		return nil
	default:
		return fmt.Errorf("unknown run status %q", string(s))
	}
}

// StepStatus makes step lifecycle auditable without inferring state from missing fields.
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusBlocked   StepStatus = "blocked"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

// Validation blocks invalid step states before they reach runtime ledger or client events.
func (s StepStatus) Validate() error {
	switch s {
	case StepStatusPending, StepStatusRunning, StepStatusBlocked, StepStatusCompleted, StepStatusFailed, StepStatusSkipped:
		return nil
	default:
		return fmt.Errorf("unknown step status %q", string(s))
	}
}
