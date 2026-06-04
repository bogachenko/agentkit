package runtime

import (
	"testing"
	"time"

	"github.com/bogachenko/agentkit/core/tool"
)

func TestStepSourceValidateRejectsUnknownSource(t *testing.T) {
	source := StepSource("system")

	if err := source.Validate(); err == nil {
		t.Fatal("expected error for unknown step source")
	}
}

func TestStepValidateAcceptsCompletedRuntimeStep(t *testing.T) {
	startedAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(time.Second)

	step := Step{
		ID:          StepID("step-1"),
		Source:      StepSourceRuntime,
		Status:      StepStatusCompleted,
		Description: "Validated route decision.",
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
	}

	if err := step.Validate(); err != nil {
		t.Fatalf("expected valid step, got error: %v", err)
	}
}

func TestStepValidateRejectsFailedStepWithoutFailure(t *testing.T) {
	step := Step{
		ID:          StepID("step-1"),
		Source:      StepSourceTool,
		Status:      StepStatusFailed,
		Description: "Executed tool.",
	}

	if err := step.Validate(); err == nil {
		t.Fatal("expected error for failed step without failure")
	}
}

func TestStepValidateRejectsNonFailedStepWithFailure(t *testing.T) {
	step := Step{
		ID:          StepID("step-1"),
		Source:      StepSourceRuntime,
		Status:      StepStatusCompleted,
		Description: "Validated route decision.",
		Failure: &Failure{
			Code:    FailureCodeInternalError,
			Message: "unexpected failure",
		},
	}

	if err := step.Validate(); err == nil {
		t.Fatal("expected error for completed step with failure")
	}
}

func TestStepValidateRejectsInvalidTimeOrder(t *testing.T) {
	startedAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	step := Step{
		ID:          StepID("step-1"),
		Source:      StepSourceRuntime,
		Status:      StepStatusCompleted,
		Description: "Validated route decision.",
		StartedAt:   startedAt,
		FinishedAt:  startedAt.Add(-time.Second),
	}

	if err := step.Validate(); err == nil {
		t.Fatal("expected error for invalid time order")
	}
}

func TestRouteDecisionValidateAcceptsToolRoute(t *testing.T) {
	decision := RouteDecision{
		Kind:     RouteKindCallTool,
		ToolName: tool.Name("search_products"),
		Reason:   "Model requested an explicit tool call.",
	}

	if err := decision.Validate(); err != nil {
		t.Fatalf("expected valid route decision, got error: %v", err)
	}
}

func TestRouteDecisionValidateRejectsToolRouteWithoutToolName(t *testing.T) {
	decision := RouteDecision{
		Kind:   RouteKindCallTool,
		Reason: "Model requested an explicit tool call.",
	}

	if err := decision.Validate(); err == nil {
		t.Fatal("expected error for call_tool route without tool name")
	}
}

func TestRouteDecisionValidateAcceptsBlockedRouteWithFailure(t *testing.T) {
	decision := RouteDecision{
		Kind:   RouteKindBlocked,
		Reason: "Policy requires user approval before write operation.",
		Failure: &Failure{
			Code:    FailureCodeApprovalRequired,
			Message: "approval is required before executing this tool",
		},
	}

	if err := decision.Validate(); err != nil {
		t.Fatalf("expected valid blocked decision, got error: %v", err)
	}
}

func TestRouteDecisionValidateRejectsBlockedRouteWithoutFailure(t *testing.T) {
	decision := RouteDecision{
		Kind:   RouteKindBlocked,
		Reason: "Policy requires user approval before write operation.",
	}

	if err := decision.Validate(); err == nil {
		t.Fatal("expected error for blocked route without failure")
	}
}

func TestRouteDecisionValidateRejectsRespondRouteWithToolName(t *testing.T) {
	decision := RouteDecision{
		Kind:     RouteKindRespond,
		ToolName: tool.Name("search_products"),
		Reason:   "Answer can be produced from current context.",
	}

	if err := decision.Validate(); err == nil {
		t.Fatal("expected error for respond route with tool name")
	}
}

func TestStateValidateAcceptsRunningState(t *testing.T) {
	startedAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	state := State{
		RunID:     RunID("run-1"),
		Status:    RunStatusRunning,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
		Steps: []Step{
			{
				ID:          StepID("step-1"),
				Source:      StepSourceUser,
				Status:      StepStatusCompleted,
				Description: "Received user input.",
				StartedAt:   startedAt,
				FinishedAt:  startedAt,
			},
		},
		Decision: &RouteDecision{
			Kind:     RouteKindCallTool,
			ToolName: tool.Name("search_products"),
			Reason:   "Model requested an explicit tool call.",
		},
	}

	if err := state.Validate(); err != nil {
		t.Fatalf("expected valid state, got error: %v", err)
	}
}

func TestStateValidateRejectsFailedRunWithoutFailure(t *testing.T) {
	state := State{
		RunID:  RunID("run-1"),
		Status: RunStatusFailed,
	}

	if err := state.Validate(); err == nil {
		t.Fatal("expected error for failed run without failure")
	}
}

func TestStateValidateRejectsRunningRunWithFailure(t *testing.T) {
	state := State{
		RunID:  RunID("run-1"),
		Status: RunStatusRunning,
		Failure: &Failure{
			Code:    FailureCodeInternalError,
			Message: "unexpected failure",
		},
	}

	if err := state.Validate(); err == nil {
		t.Fatal("expected error for running run with failure")
	}
}
