package llm

import "fmt"

// Separates conversation authorship.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

func (r Role) Validate() error {
	switch r {
	case RoleUser, RoleAssistant, RoleTool:
		return nil
	default:
		return fmt.Errorf("unknown llm role %q", string(r))
	}
}
