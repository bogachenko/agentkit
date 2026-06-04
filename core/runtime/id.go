package runtime

import (
	"fmt"
	"strings"
)

// RunID gives every runtime execution a stable identity for tracing, ledger, and client events.
type RunID string

// Validation prevents anonymous runs from entering runtime state.
func (id RunID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("run id is required")
	}

	return nil
}

// StepID gives every runtime step a stable identity for ordering, tracing, and ledger links.
type StepID string

// Validation prevents anonymous steps from entering runtime state.
func (id StepID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("step id is required")
	}

	return nil
}
