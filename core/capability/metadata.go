package capability

import (
	"fmt"
	"strings"

	"github.com/bogachenko/agentkit/core/tool"
)

// Permission declares an external access requirement without embedding provider-specific auth logic.
type Permission string

// Validation prevents empty permission declarations from weakening capability contracts.
func (p Permission) Validate() error {
	if strings.TrimSpace(string(p)) == "" {
		return fmt.Errorf("capability permission is required")
	}

	return nil
}

// SourceRequirement declares required external data sources without coupling core to their implementations.
type SourceRequirement struct {
	Name     string
	Required bool
}

// Validation prevents unnamed data dependencies from entering capability metadata.
func (s SourceRequirement) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("capability source name is required")
	}

	return nil
}

// Metadata is the deterministic source of truth for what a capability exposes to runtime.
type Metadata struct {
	ID              ID
	Name            string
	Description     string
	Tools           []tool.Contract
	RequiredSources []SourceRequirement
	Permissions     []Permission
}

// Validation rejects incomplete or ambiguous capability declarations before runtime uses them.
func (m Metadata) Validate() error {
	if err := m.ID.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("capability name is required for %q", string(m.ID))
	}

	if strings.TrimSpace(m.Description) == "" {
		return fmt.Errorf("capability description is required for %q", string(m.ID))
	}

	if len(m.Tools) == 0 {
		return fmt.Errorf("capability %q must expose at least one tool contract", string(m.ID))
	}

	toolNames := map[tool.Name]struct{}{}
	for index, contract := range m.Tools {
		if err := contract.Validate(); err != nil {
			return fmt.Errorf("capability %q tool %d: %w", string(m.ID), index, err)
		}

		if _, exists := toolNames[contract.Name]; exists {
			return fmt.Errorf("capability %q has duplicate tool %q", string(m.ID), string(contract.Name))
		}

		toolNames[contract.Name] = struct{}{}
	}

	sourceNames := map[string]struct{}{}
	for index, source := range m.RequiredSources {
		if err := source.Validate(); err != nil {
			return fmt.Errorf("capability %q source %d: %w", string(m.ID), index, err)
		}

		normalized := strings.TrimSpace(source.Name)
		if _, exists := sourceNames[normalized]; exists {
			return fmt.Errorf("capability %q has duplicate source %q", string(m.ID), normalized)
		}

		sourceNames[normalized] = struct{}{}
	}

	permissions := map[Permission]struct{}{}
	for index, permission := range m.Permissions {
		if err := permission.Validate(); err != nil {
			return fmt.Errorf("capability %q permission %d: %w", string(m.ID), index, err)
		}

		if _, exists := permissions[permission]; exists {
			return fmt.Errorf("capability %q has duplicate permission %q", string(m.ID), string(permission))
		}

		permissions[permission] = struct{}{}
	}

	return nil
}
