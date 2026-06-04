package tool

import (
	"fmt"
	"strings"
)

// Contract lets runtime, policies, adapters, and UIs inspect a tool without executing it.
type Contract struct {
	Name             Name
	Description      string
	InputSchema      map[string]any
	OutputSchema     map[string]any
	ReadOnly         bool
	RequiresApproval bool
}

// Validation blocks incomplete tool metadata before it reaches registries or runtime policy checks.
func (c Contract) Validate() error {
	if err := c.Name.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(c.Description) == "" {
		return fmt.Errorf("tool contract description is required for %q", string(c.Name))
	}

	if c.InputSchema == nil {
		return fmt.Errorf("tool contract input schema is required for %q", string(c.Name))
	}

	if c.OutputSchema == nil {
		return fmt.Errorf("tool contract output schema is required for %q", string(c.Name))
	}

	return nil
}
