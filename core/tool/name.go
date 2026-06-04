package tool

import (
	"fmt"
	"strings"
)

// Identity across contracts, runtime ledger, and adapters.
type Name string

// Validation prevents anonymous tools from entering registries or runtime decisions.
func (n Name) Validate() error {
	if strings.TrimSpace(string(n)) == "" {
		return fmt.Errorf("tool name is required")
	}

	return nil
}
