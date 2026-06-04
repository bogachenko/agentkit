package capability

import (
	"fmt"
	"strings"
)

// ID gives every capability stable identity across registries, runtime policy, and adapters.
type ID string

// Validation prevents anonymous capabilities from entering runtime configuration.
func (id ID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("capability id is required")
	}

	return nil
}
