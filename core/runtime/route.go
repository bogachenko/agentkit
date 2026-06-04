package runtime

import "fmt"

// RouteKind describes the next runtime branch without embedding semantic intent classification.
type RouteKind string

const (
	RouteKindRespond         RouteKind = "respond"
	RouteKindCallTool        RouteKind = "call_tool"
	RouteKindRequireApproval RouteKind = "require_approval"
	RouteKindBlocked         RouteKind = "blocked"
	RouteKindComplete        RouteKind = "complete"
)

// Validation prevents unknown branches from reaching the orchestrator.
func (k RouteKind) Validate() error {
	switch k {
	case RouteKindRespond, RouteKindCallTool, RouteKindRequireApproval, RouteKindBlocked, RouteKindComplete:
		return nil
	default:
		return fmt.Errorf("unknown route kind %q", string(k))
	}
}
