package runtime

import (
	"fmt"

	"github.com/bogachenko/agentkit/core/tool"
)

// Policy stores deterministic runtime rules that must be enforced before execution.
type Policy struct {
	ToolContracts []tool.Contract
}

// ToolContract returns the declared contract for one tool without executing or selecting tools.
func (p Policy) ToolContract(name tool.Name) (tool.Contract, bool, error) {
	if err := name.Validate(); err != nil {
		return tool.Contract{}, false, err
	}

	for index, contract := range p.ToolContracts {
		if err := contract.Validate(); err != nil {
			return tool.Contract{}, false, fmt.Errorf("tool contract %d: %w", index, err)
		}

		if contract.Name == name {
			return contract, true, nil
		}
	}

	return tool.Contract{}, false, nil
}

// RequiresApproval checks declared tool metadata instead of inferring risk from user text.
func (p Policy) RequiresApproval(name tool.Name) (bool, error) {
	contract, exists, err := p.ToolContract(name)
	if err != nil {
		return false, err
	}

	if !exists {
		return false, ValidationError{
			Code:    ValidationCodeUnknownTool,
			Message: fmt.Sprintf("tool %q is not registered", string(name)),
		}
	}

	return contract.RequiresApproval, nil
}

// IsReadOnly checks declared tool metadata so runtime never guesses mutability from tool names.
func (p Policy) IsReadOnly(name tool.Name) (bool, error) {
	contract, exists, err := p.ToolContract(name)
	if err != nil {
		return false, err
	}

	if !exists {
		return false, ValidationError{
			Code:    ValidationCodeUnknownTool,
			Message: fmt.Sprintf("tool %q is not registered", string(name)),
		}
	}

	return contract.ReadOnly, nil
}
