package runtime

import (
	"fmt"
	"strings"

	"github.com/bogachenko/agentkit/core/tool"
)

// RouteDecision represents an already selected next branch that runtime can validate deterministically.
type RouteDecision struct {
	Kind     RouteKind
	ToolName tool.Name
	ToolArgs map[string]any
	Reason   string
	Failure  *Failure
}

// Validation keeps route decisions structural and prevents hidden fallback behavior.
func (d RouteDecision) Validate() error {
	if err := d.Kind.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(d.Reason) == "" {
		return fmt.Errorf("route decision reason is required")
	}

	switch d.Kind {
	case RouteKindCallTool, RouteKindRequireApproval:
		if err := d.ToolName.Validate(); err != nil {
			return fmt.Errorf("route decision tool name: %w", err)
		}

		if d.Failure != nil {
			return fmt.Errorf("%s route decision cannot include failure", d.Kind)
		}

	case RouteKindBlocked:
		if d.Failure == nil {
			return fmt.Errorf("blocked route decision requires failure")
		}

		if err := d.Failure.Validate(); err != nil {
			return fmt.Errorf("blocked route decision failure: %w", err)
		}

		if strings.TrimSpace(string(d.ToolName)) != "" {
			return fmt.Errorf("blocked route decision cannot include tool name")
		}

		if len(d.ToolArgs) > 0 {
			return fmt.Errorf("blocked route decision cannot include tool args")
		}
	default:
		if strings.TrimSpace(string(d.ToolName)) != "" {
			return fmt.Errorf("%s route decision cannot include tool name", d.Kind)
		}

		if len(d.ToolArgs) > 0 {
			return fmt.Errorf("%s route decision cannot include tool args", d.Kind)
		}

		if d.Failure != nil {
			return fmt.Errorf("%s route decision cannot include failure", d.Kind)
		}
	}

	return nil
}
