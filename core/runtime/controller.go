package runtime

import "context"

// Controller gives adapters and transports one stable entry point into runtime orchestration.
type Controller struct {
	Orchestrator Orchestrator
}

// HandleDecision keeps transport handlers thin and delegates deterministic runtime work to Orchestrator.
func (c Controller) HandleDecision(ctx context.Context, command Command) (DecisionResult, error) {
	return c.Orchestrator.HandleDecision(ctx, command)
}
