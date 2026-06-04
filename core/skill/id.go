package skill

import (
	"fmt"
	"strings"
)

// ID gives every skill stable identity across registries, agent builders, and audit logs.
type ID string

// Validation prevents anonymous skills from entering deterministic registries.
func (id ID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("skill id is required")
	}

	return nil
}
