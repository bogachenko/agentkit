package runtime

import (
	"context"
	"fmt"

	"github.com/bogachenko/agentkit/core/llm"
	"github.com/bogachenko/agentkit/core/port"
	"github.com/bogachenko/agentkit/core/tool"
)

type Harness struct {
	Model        port.Model
	ToolExecutor port.ToolExecutor
	Validator    Validator
	Publisher    port.Publisher
	Clock        port.Clock
	IDGenerator  port.IDGenerator
}

func (h Harness) Run(ctx context.Context, command RunCommand) (RunResult, error) {
	if err := h.validateDependencies(); err != nil {
		return RunResult{}, err
	}
	if err := command.Validate(); err != nil {
		return RunResult{}, err
	}
	ledger, err := NewLedger(command.RunID)
	if err != nil {
		return RunResult{}, err
	}
	if err := h.publish(ctx, port.EventTypeStarted, command, map[string]any{"status": string(RunStatusRunning)}); err != nil {
		return RunResult{}, err
	}
	messages := make([]llm.Message, 0, len(command.History)+command.MaxSteps*2+1)
	messages = append(messages, command.History...)
	messages = append(messages, llm.NewMessage(llm.RoleUser, llm.TextPart(command.UserInput)))
	state := State{RunID: command.RunID, Status: RunStatusRunning, StartedAt: h.Clock.Now(), UpdatedAt: h.Clock.Now()}
	for stepIndex := 0; stepIndex < command.MaxSteps; stepIndex++ {
		response, err := h.Model.Generate(ctx, llm.Request{System: command.System, Messages: messages, RuntimeContext: command.RuntimeContext})
		if err != nil {
			return h.failedResult(ctx, command, ledger, Failure{Code: FailureCodeModelFailed, Message: err.Error()}, stepIndex)
		}
		if err := response.Validate(); err != nil {
			return h.failedResult(ctx, command, ledger, Failure{Code: FailureCodeModelFailed, Message: err.Error()}, stepIndex)
		}
		messages = append(messages, response)
		decision, err := ExtractRouteDecision(response)
		if err != nil {
			return h.failedResult(ctx, command, ledger, Failure{Code: FailureCodeInvalidDecision, Message: err.Error()}, stepIndex)
		}
		orchestrator := Orchestrator{Validator: h.Validator, Ledger: ledger, Publisher: h.Publisher, Clock: h.Clock, IDGenerator: h.IDGenerator}
		decisionResult, err := orchestrator.HandleDecision(ctx, DecisionCommand{SessionID: command.SessionID, State: state, Decision: decision, Approvals: command.Approvals})
		if err != nil {
			return RunResult{}, err
		}
		state.Decision = &decisionResult.Decision
		state.UpdatedAt = h.Clock.Now()
		if decisionResult.Blocked {
			return h.blockedResult(command, ledger, decisionResult.Decision, stepIndex+1)
		}
		switch decisionResult.Decision.Kind {
		case RouteKindRespond, RouteKindComplete:
			result := RunResult{RunID: command.RunID, Status: RunStatusCompleted, FinalMessage: &response, Decision: &decisionResult.Decision, LedgerSummary: ledger.Summary(), StepsCompleted: stepIndex + 1}
			if err := result.Validate(); err != nil {
				return RunResult{}, err
			}
			if err := h.publish(ctx, port.EventTypeCompleted, command, map[string]any{"status": string(RunStatusCompleted)}); err != nil {
				return RunResult{}, err
			}
			return result, nil
		case RouteKindRequireApproval:
			failure := Failure{Code: FailureCodeApprovalRequired, Message: fmt.Sprintf("tool %q requires approval", string(decisionResult.Decision.ToolName))}
			if err := h.publish(ctx, port.EventTypeBlocked, command, map[string]any{"status": string(RunStatusBlocked), "failure_code": string(failure.Code), "tool_name": string(decisionResult.Decision.ToolName)}); err != nil {
				return RunResult{}, err
			}
			return h.blockedResult(command, ledger, RouteDecision{Kind: RouteKindBlocked, Reason: "Runtime requires user approval before tool execution.", Failure: &failure}, stepIndex+1)
		case RouteKindCallTool:
			toolResultMessage, err := h.executeTool(ctx, command, ledger, decisionResult.Decision)
			if err != nil {
				return h.failedResult(ctx, command, ledger, Failure{Code: FailureCodeToolFailed, Message: err.Error()}, stepIndex+1)
			}
			messages = append(messages, toolResultMessage)
		default:
			return h.failedResult(ctx, command, ledger, Failure{Code: FailureCodeInvalidDecision, Message: fmt.Sprintf("unsupported route kind %q", string(decisionResult.Decision.Kind))}, stepIndex+1)
		}
	}
	return h.failedResult(ctx, command, ledger, Failure{Code: FailureCodeInvalidState, Message: "run reached max steps without terminal decision"}, command.MaxSteps)
}

func (h Harness) validateDependencies() error {
	if h.Model == nil {
		return fmt.Errorf("runtime harness model is required")
	}
	if h.ToolExecutor == nil {
		return fmt.Errorf("runtime harness tool executor is required")
	}
	if h.Publisher == nil {
		return fmt.Errorf("runtime harness publisher is required")
	}
	if h.Clock == nil {
		return fmt.Errorf("runtime harness clock is required")
	}
	if h.IDGenerator == nil {
		return fmt.Errorf("runtime harness id generator is required")
	}
	return nil
}

func (h Harness) executeTool(ctx context.Context, command RunCommand, ledger *Ledger, decision RouteDecision) (llm.Message, error) {
	call := tool.NewCall(decision.ToolName, decision.ToolArgs)
	if approval, exists := matchedApprovedApproval(command.RunID, decision, command.Approvals); exists {
		call.RuntimeData = cloneRuntimeData(approval.Payload)
	}
	if err := ledger.Append(NewToolCallEntry(LedgerEntryID(h.IDGenerator.NewID()), command.RunID, call, h.Clock.Now())); err != nil {
		return llm.Message{}, err
	}
	result, err := h.ToolExecutor.ExecuteTool(ctx, call)
	if err != nil {
		return llm.Message{}, err
	}
	if err := result.Validate(); err != nil {
		return llm.Message{}, err
	}
	if err := ledger.Append(NewToolResultEntry(LedgerEntryID(h.IDGenerator.NewID()), command.RunID, result, h.Clock.Now())); err != nil {
		return llm.Message{}, err
	}
	if err := h.publish(ctx, port.EventTypeStep, command, map[string]any{"status": "tool_completed", "tool_name": string(result.Name)}); err != nil {
		return llm.Message{}, err
	}
	return llm.NewMessage(llm.RoleTool, llm.FunctionResponsePart(string(result.Name), result.Output)), nil
}

func matchedApprovedApproval(runID RunID, decision RouteDecision, approvals []Approval) (Approval, bool) {
	argsHash, err := NewToolArgsHash(decision.ToolArgs)
	if err != nil {
		return Approval{}, false
	}
	approval, exists := findApproval(runID, decision.ToolName, argsHash, approvals)
	if !exists || !approval.IsApproved() {
		return Approval{}, false
	}
	return approval, true
}

func cloneRuntimeData(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func (h Harness) blockedResult(command RunCommand, ledger *Ledger, decision RouteDecision, stepsCompleted int) (RunResult, error) {
	if decision.Failure == nil {
		return RunResult{}, fmt.Errorf("blocked decision requires failure")
	}
	result := RunResult{RunID: command.RunID, Status: RunStatusBlocked, Decision: &decision, Failure: decision.Failure, LedgerSummary: ledger.Summary(), StepsCompleted: stepsCompleted}
	if err := result.Validate(); err != nil {
		return RunResult{}, err
	}
	return result, nil
}

func (h Harness) failedResult(ctx context.Context, command RunCommand, ledger *Ledger, failure Failure, stepsCompleted int) (RunResult, error) {
	if err := failure.Validate(); err != nil {
		return RunResult{}, err
	}
	if err := h.publish(ctx, port.EventTypeFailed, command, map[string]any{"status": string(RunStatusFailed), "failure_code": string(failure.Code), "failure_message": failure.Message}); err != nil {
		return RunResult{}, err
	}
	result := RunResult{RunID: command.RunID, Status: RunStatusFailed, Failure: &failure, LedgerSummary: ledger.Summary(), StepsCompleted: stepsCompleted}
	if err := result.Validate(); err != nil {
		return RunResult{}, err
	}
	return result, nil
}

func (h Harness) publish(ctx context.Context, eventType port.EventType, command RunCommand, payload map[string]any) error {
	return h.Publisher.Publish(ctx, port.Event{Type: eventType, RunID: string(command.RunID), SessionID: string(command.SessionID), Payload: payload, CreatedAt: h.Clock.Now()})
}
