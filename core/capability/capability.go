package capability

import "fmt"

// Capability lets implementations expose metadata without allowing core to execute domain logic.
type Capability interface {
	Metadata() Metadata
}

// ValidateCapability keeps registry inputs explicit and prevents nil capability implementations.
func ValidateCapability(value Capability) error {
	if value == nil {
		return fmt.Errorf("capability is required")
	}

	return value.Metadata().Validate()
}
