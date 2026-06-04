package runtime

import (
	"context"
	"fmt"

	"github.com/bogachenko/agentkit/core/port"
	"github.com/bogachenko/agentkit/core/session"
)

// Orchestrator applies deterministic runtime validation and records the result without choosing actions.
type Orchestrator struct {
	Validator   Validator
	Ledger      *Ledger
	Publisher   port.Publisher
	Clock       port.Clock
	IDGenerator port.IDGenerator
}

// DecisionResult exposes whether the supplied decision was accepted or converted into a blocked route.
type DecisionResult struct {
	Decision RouteDecision
	Accepted bool
	Blocked  bool
}

// Validate keeps controller outputs structurally safe for callers and tests.
func (r DecisionResult) Validate() error {
	if err := r.Decision.Validate(); err != nil {
		return err
	}

	if r.Accepted && r.Blocked {
		return fmt.Errorf("decision result cannot be both accepted and blocked")
	}

	if !r.Accepted && !r.Blocked {
		return fmt.Errorf("decision result must be accepted or blocked")
	}

	if r.Blocked && r.Decision.Kind != RouteKindBlocked {
		return fmt.Errorf("blocked decision result must contain blocked route decision")
	}

	return nil
}

// HandleDecision validates one explicit route decision and records the accepted or blocked outcome.
func (o Orchestrator) HandleDecision(ctx context.Context, command DecisionCommand) (DecisionResult, error) {
	if err := o.validateDependencies(); err != nil {
		return DecisionResult{}, err
	}

	if err := command.Validate(); err != nil {
		return DecisionResult{}, err
	}

	decision := command.Decision
	accepted := true
	blocked := false

	if err := o.Validator.ValidateDecision(ValidationInput{
		State:     command.State,
		Decision:  command.Decision,
		Approvals: command.Approvals,
	}); err != nil {
		decision = o.Validator.BlockedDecision(err)
		accepted = false
		blocked = true
	}

	result := DecisionResult{
		Decision: decision,
		Accepted: accepted,
		Blocked:  blocked,
	}

	if err := result.Validate(); err != nil {
		return DecisionResult{}, err
	}

	if err := o.appendDecision(command.State.RunID, decision); err != nil {
		return DecisionResult{}, err
	}

	if err := o.publishDecision(ctx, command.SessionID, command.State.RunID, result); err != nil {
		return DecisionResult{}, err
	}

	return result, nil
}

// Dependency validation prevents silent no-op orchestration when required infrastructure is missing.
func (o Orchestrator) validateDependencies() error {
	if o.Ledger == nil {
		return fmt.Errorf("runtime orchestrator ledger is required")
	}

	if o.Publisher == nil {
		return fmt.Errorf("runtime orchestrator publisher is required")
	}

	if o.Clock == nil {
		return fmt.Errorf("runtime orchestrator clock is required")
	}

	if o.IDGenerator == nil {
		return fmt.Errorf("runtime orchestrator id generator is required")
	}

	return nil
}

// appendDecision records the final accepted/blocked route as append-only runtime history.
func (o Orchestrator) appendDecision(runID RunID, decision RouteDecision) error {
	entry := NewRouteDecisionEntry(
		LedgerEntryID(o.IDGenerator.NewID()),
		runID,
		decision,
		o.Clock.Now(),
	)

	return o.Ledger.Append(entry)
}

// publishDecision emits transport-neutral progress without coupling runtime to HTTP, SSE, or queues.
func (o Orchestrator) publishDecision(ctx context.Context, sessionID session.ID, runID RunID, result DecisionResult) error {
	eventType := port.EventTypeStep
	if result.Blocked {
		eventType = port.EventTypeBlocked
	}

	payload := map[string]any{
		"route_kind": string(result.Decision.Kind),
		"reason":     result.Decision.Reason,
		"accepted":   result.Accepted,
		"blocked":    result.Blocked,
	}

	if result.Decision.ToolName != "" {
		payload["tool_name"] = string(result.Decision.ToolName)
	}

	if result.Decision.Failure != nil {
		payload["failure_code"] = string(result.Decision.Failure.Code)
		payload["failure_message"] = result.Decision.Failure.Message
	}

	return o.Publisher.Publish(ctx, port.Event{
		Type:      eventType,
		RunID:     string(runID),
		SessionID: string(sessionID),
		Payload:   payload,
		CreatedAt: o.Clock.Now(),
	})
}
