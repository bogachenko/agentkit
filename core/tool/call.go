package tool

import "fmt"

type Call struct {
	Name        Name
	Args        map[string]any
	RuntimeData map[string]any
}

func NewCall(name Name, args map[string]any) Call {
	if args == nil {
		args = map[string]any{}
	}
	return Call{Name: name, Args: args}
}

func (c Call) Validate() error {
	if err := c.Name.Validate(); err != nil {
		return err
	}
	if c.Args == nil {
		return fmt.Errorf("tool call args must be an empty map, not nil")
	}
	return nil
}
