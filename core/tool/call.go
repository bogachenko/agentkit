package tool

import "fmt"

// Call represents an explicit tool invocation request without embedding execution behavior.
type Call struct {
	Name Name
	Args map[string]any
}

// Constructor normalizes empty arguments so runtime and adapters receive a stable shape.
func NewCall(name Name, args map[string]any) Call {
	if args == nil {
		args = map[string]any{}
	}

	return Call{
		Name: name,
		Args: args,
	}
}

// Validation keeps malformed tool calls out of runtime execution and ledger state.
func (c Call) Validate() error {
	if err := c.Name.Validate(); err != nil {
		return err
	}

	if c.Args == nil {
		return fmt.Errorf("tool call args must be an empty map, not nil")
	}

	return nil
}
