package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bogachenko/agentkit/core/llm"
	"github.com/bogachenko/agentkit/core/port"
	coresession "github.com/bogachenko/agentkit/core/session"
)

// StepRunCommand starts one run driven by a StepProvider instead of route-decision JSON.
type StepRunCommand struct {
	RunID     RunID
	SessionID coresession.ID
	MaxSteps  int
}

// Validate rejects incomplete step-pipeline runs before the provider starts.
func (c StepRunCommand) Validate() error {
	if err := c.RunID.Validate(); err != nil {
		return err
	}

	if err := c.SessionID.Validate(); err != nil {
		return err
	}

	if c.MaxSteps <= 0 {
		return fmt.Errorf("step run command max steps must be positive")
	}

	return nil
}

// StepOrchestrator owns the generic StepProvider -> State -> Ledger -> RunResult pipeline.
type StepOrchestrator struct {
	StepProvider StepProvider
	Publisher    port.Publisher
	Clock        port.Clock
	IDGenerator  port.IDGenerator
}

// Run consumes provider steps until final response, max steps, source completion, or failure.
func (o StepOrchestrator) Run(ctx context.Context, command StepRunCommand) (RunResult, error) {
	if err := o.validateDependencies(); err != nil {
		return RunResult{}, err
	}

	if err := command.Validate(); err != nil {
		return RunResult{}, err
	}

	ledger, err := NewLedger(command.RunID)
	if err != nil {
		return RunResult{}, err
	}

	state := State{
		RunID:     command.RunID,
		Status:    RunStatusRunning,
		StartedAt: o.Clock.Now(),
		UpdatedAt: o.Clock.Now(),
	}

	if err := o.publish(ctx, port.EventTypeStarted, command, map[string]any{
		"status": string(RunStatusRunning),
	}); err != nil {
		return RunResult{}, err
	}

	updater := StateUpdater{
		Clock:       o.Clock,
		IDGenerator: o.IDGenerator,
	}

	for state.StepCount < command.MaxSteps {
		rawSteps, err := o.StepProvider.NextSteps(ctx, state)
		if err != nil {
			if errors.Is(err, ErrStepSourceDone) {
				break
			}

			return o.failedResult(ctx, command, ledger, state, Failure{
				Code:    FailureCodeInternalError,
				Message: err.Error(),
			})
		}

		if len(rawSteps) == 0 {
			continue
		}

		for _, rawStep := range rawSteps {
			if state.StepCount >= command.MaxSteps {
				return o.failedResult(ctx, command, ledger, state, Failure{
					Code:    FailureCodeInvalidState,
					Message: "run reached max steps without terminal response",
				})
			}

			step, err := updater.Apply(&state, rawStep)
			if err != nil {
				return o.failedResult(ctx, command, ledger, state, Failure{
					Code:    FailureCodeInvalidState,
					Message: err.Error(),
				})
			}

			if err := ledger.Append(NewStepEntry(
				LedgerEntryID(o.IDGenerator.NewID()),
				command.RunID,
				step,
				o.Clock.Now(),
			)); err != nil {
				return RunResult{}, err
			}

			if err := o.publish(ctx, port.EventTypeStep, command, stepPayload(step)); err != nil {
				return RunResult{}, err
			}

			if step.Kind == StepKindToolResult && !step.ToolResult.OK {
				return o.failedResult(ctx, command, ledger, state, toolFailureFromResult(step.ToolName, step.ToolResult))
			}
			if step.Kind == StepKindAssistantText && step.Final {
				state.Status = RunStatusCompleted
				state.UpdatedAt = o.Clock.Now()

				result := RunResult{
					RunID:          command.RunID,
					Status:         RunStatusCompleted,
					FinalMessage:   finalMessageFromText(step.Text),
					LedgerSummary:  ledger.Summary(),
					StepsCompleted: state.StepCount,
				}

				if err := result.Validate(); err != nil {
					return RunResult{}, err
				}

				if err := o.publish(ctx, port.EventTypeCompleted, command, map[string]any{
					"status": string(RunStatusCompleted),
				}); err != nil {
					return RunResult{}, err
				}

				return result, nil
			}
		}
	}

	return o.failedResult(ctx, command, ledger, state, Failure{
		Code:    FailureCodeInvalidState,
		Message: "run finished without final response",
	})
}

func (o StepOrchestrator) validateDependencies() error {
	if o.StepProvider == nil {
		return fmt.Errorf("runtime step provider is required")
	}

	if o.Publisher == nil {
		return fmt.Errorf("runtime publisher is required")
	}

	if o.Clock == nil {
		return fmt.Errorf("runtime clock is required")
	}

	if o.IDGenerator == nil {
		return fmt.Errorf("runtime id generator is required")
	}

	return nil
}

func (o StepOrchestrator) failedResult(
	ctx context.Context,
	command StepRunCommand,
	ledger *Ledger,
	state State,
	failure Failure,
) (RunResult, error) {
	state.Status = RunStatusFailed
	state.Failure = &failure
	state.UpdatedAt = o.Clock.Now()

	if err := state.Validate(); err != nil {
		return RunResult{}, err
	}

	if err := o.publish(ctx, port.EventTypeFailed, command, map[string]any{
		"status":       string(RunStatusFailed),
		"failure_code": string(failure.Code),
		"message":      failure.Message,
	}); err != nil {
		return RunResult{}, err
	}

	result := RunResult{
		RunID:          command.RunID,
		Status:         RunStatusFailed,
		Failure:        &failure,
		LedgerSummary:  ledger.Summary(),
		StepsCompleted: state.StepCount,
	}

	if err := result.Validate(); err != nil {
		return RunResult{}, err
	}

	return result, nil
}

func (o StepOrchestrator) publish(ctx context.Context, eventType port.EventType, command StepRunCommand, payload map[string]any) error {
	return o.Publisher.Publish(ctx, port.Event{
		Type:      eventType,
		RunID:     string(command.RunID),
		SessionID: string(command.SessionID),
		Payload:   payload,
		CreatedAt: o.Clock.Now(),
	})
}

func finalMessageFromText(text string) *llm.Message {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	message := llm.NewMessage(llm.RoleAssistant, llm.TextPart(text))
	return &message
}

func stepPayload(step Step) map[string]any {
	payload := map[string]any{
		"step_id":     string(step.ID),
		"kind":        string(step.Kind),
		"source":      string(step.Source),
		"status":      string(step.Status),
		"description": step.Description,
		"final":       step.Final,
	}

	if strings.TrimSpace(string(step.ToolName)) != "" {
		payload["tool_name"] = string(step.ToolName)
	}

	if strings.TrimSpace(step.ToolCallID) != "" {
		payload["tool_call_id"] = step.ToolCallID
	}

	if len(step.ToolArgs) > 0 {
		payload["tool_args"] = step.ToolArgs
	}

	if step.Kind == StepKindToolResult {
		payload["tool_result_ok"] = step.ToolResult.OK
		payload["tool_result_has_evidence"] = step.ToolResult.HasEvidence

		if !step.ToolResult.OK {
			payload["tool_error_kind"] = string(step.ToolResult.ErrorKind)
			payload["tool_error_message"] = step.ToolResult.ErrorMessage
		}
	}

	if strings.TrimSpace(step.Text) != "" {
		payload["text"] = step.Text
	}

	return payload
}
