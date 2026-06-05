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

	// TraceInput is optional dev-only trace content. Leave empty when message capture is disabled.
	TraceInput string
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
	Tracer       port.Tracer
}

// Run consumes provider steps until final response, max steps, source completion, or failure.
func (o StepOrchestrator) Run(ctx context.Context, command StepRunCommand) (RunResult, error) {
	if err := o.validateDependencies(); err != nil {
		return RunResult{}, err
	}

	if err := command.Validate(); err != nil {
		return RunResult{}, err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx, runSpan := o.startTrace(ctx, "agentkit.step_run", runTraceAttrs(command))
	defer func() {
		if runSpan != nil {
			runSpan.End()
		}
	}()

	o.setTraceInput(runSpan, command.TraceInput)

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

	o.traceEvent(runSpan, "agentkit.run.started", map[string]any{
		"agentkit.run.status": string(RunStatusRunning),
	})

	updater := StateUpdater{
		Clock:       o.Clock,
		IDGenerator: o.IDGenerator,
	}

	toolCallsUsed := 0
	continuationsUsed := 0
	maxContinuations := command.MaxSteps + 8
	recoverableToolContinuationsUsed := 0
	maxRecoverableToolContinuations := 5

	for {
		rawSteps, err := o.StepProvider.NextSteps(ctx, state)
		if err != nil {
			if errors.Is(err, ErrStepSourceDone) {
				break
			}

			return o.failedResult(ctx, command, ledger, state, Failure{
				Code:    FailureCodeInternalError,
				Message: err.Error(),
			}, runSpan)
		}

		if len(rawSteps) == 0 {
			continue
		}

		for _, rawStep := range rawSteps {
			if rawStep.Kind == StepKindToolCall && toolCallsUsed >= command.MaxSteps {
				return o.failedResult(ctx, command, ledger, state, Failure{
					Code:    FailureCodeInvalidState,
					Message: "run reached max tool calls without terminal response",
				}, runSpan)
			}

			step, err := updater.Apply(&state, rawStep)
			if err != nil {
				return o.failedResult(ctx, command, ledger, state, Failure{
					Code:    FailureCodeInvalidState,
					Message: err.Error(),
				}, runSpan)
			}

			if step.Kind == StepKindToolCall {
				toolCallsUsed++
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

			o.traceStep(ctx, step)

			switch step.Kind {
			case StepKindToolResult:
				if !step.ToolResult.OK {
					if isRecoverableToolError(step) {
						recoverableToolContinuationsUsed++
						if recoverableToolContinuationsUsed > maxRecoverableToolContinuations {
							return o.failedResult(ctx, command, ledger, state, Failure{
								Code:    FailureCodeInvalidState,
								Message: repeatedRecoverableToolFailureMessage(step, recoverableToolContinuationsUsed, maxRecoverableToolContinuations),
							}, runSpan)
						}

						if o.addInternalInstruction(recoverableToolErrorInstruction(step, recoverableToolContinuationsUsed, maxRecoverableToolContinuations)) {
							continuationsUsed++
							if continuationsUsed > maxContinuations {
								return o.failedResult(ctx, command, ledger, state, Failure{
									Code:    FailureCodeInvalidState,
									Message: "run reached max continuation attempts after recoverable browser tool errors",
								}, runSpan)
							}

							continue
						}

						return o.failedResult(ctx, command, ledger, state, toolFailureFromResult(step.ToolName, step.ToolResult), runSpan)
					}

					if step.ToolResult.ErrorKind == ToolErrorValidation && o.addInternalInstruction(validationRecoveryInstruction(step)) {
						continuationsUsed++
						if continuationsUsed > maxContinuations {
							return o.failedResult(ctx, command, ledger, state, Failure{
								Code:    FailureCodeInvalidState,
								Message: "run reached max continuation attempts after validation errors",
							}, runSpan)
						}

						continue
					}

					return o.failedResult(ctx, command, ledger, state, toolFailureFromResult(step.ToolName, step.ToolResult), runSpan)
				}

			case StepKindAssistantText:
				if !step.Final {
					if o.addInternalInstruction(continueTaskInstruction()) {
						continuationsUsed++
						if continuationsUsed > maxContinuations {
							return o.failedResult(ctx, command, ledger, state, Failure{
								Code:    FailureCodeInvalidState,
								Message: "run reached max continuation attempts without terminal response",
							}, runSpan)
						}
					}

					continue
				}

				if state.ToolCalls > 0 && state.EvidenceCount == 0 {
					if o.addInternalInstruction(finalWithoutEvidenceInstruction()) {
						continuationsUsed++
						if continuationsUsed > maxContinuations {
							return o.failedResult(ctx, command, ledger, state, Failure{
								Code:    FailureCodeInvalidState,
								Message: "run reached max continuation attempts without tool evidence",
							}, runSpan)
						}

						continue
					}
				}

				return o.completedResult(ctx, command, ledger, state, step, runSpan)

			case StepKindStreamDone:
				if o.addInternalInstruction(streamDoneWithoutFinalInstruction()) {
					continuationsUsed++
					if continuationsUsed > maxContinuations {
						return o.failedResult(ctx, command, ledger, state, Failure{
							Code:    FailureCodeInvalidState,
							Message: "run finished without final response",
						}, runSpan)
					}

					continue
				}

				return o.failedResult(ctx, command, ledger, state, Failure{
					Code:    FailureCodeInvalidState,
					Message: "run finished without final response",
				}, runSpan)
			}
		}
	}

	return o.failedResult(ctx, command, ledger, state, Failure{
		Code:    FailureCodeInvalidState,
		Message: "run finished without final response",
	}, runSpan)
}

func (o StepOrchestrator) addInternalInstruction(instruction string) bool {
	receiver, ok := o.StepProvider.(InternalInstructionReceiver)
	if !ok || receiver == nil {
		return false
	}

	receiver.AddInternalInstruction(instruction)
	return true
}

func continueTaskInstruction() string {
	return "Continue the task. Do not produce user-visible progress text. Use the available tool evidence and continue using tools if the requested result has not been obtained yet. Produce a final answer only after the user's request is actually satisfied."
}

func finalWithoutEvidenceInstruction() string {
	return "The previous assistant text attempted to answer before confirmed tool evidence was available. Continue the task. Do not produce user-visible progress text. Use tools to collect the required evidence, then produce the final answer."
}

func streamDoneWithoutFinalInstruction() string {
	return "The previous turn ended without a final answer. Continue the task. Do not produce user-visible progress text. Produce the final answer only when the user's requested result is satisfied."
}

func validationRecoveryInstruction(step Step) string {
	message := strings.TrimSpace(step.ToolResult.ErrorMessage)
	if message == "" {
		message = "tool returned validation error"
	}

	return "The previous tool call failed validation: " + message + ". Correct the tool arguments and continue the task. Do not produce user-visible progress text."
}

func isRecoverableToolError(step Step) bool {
	if step.Kind != StepKindToolResult || step.ToolResult.OK {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(step.ToolResult.ErrorMessage))

	return strings.Contains(message, "stale_element_ref") ||
		strings.Contains(message, "element ref is stale") ||
		strings.Contains(message, "stale or unknown") ||
		strings.Contains(message, "element_not_found") ||
		strings.Contains(message, "element was not found")
}

func recoverableToolErrorInstruction(step Step, attempt int, maxAttempts int) string {
	message := strings.TrimSpace(step.ToolResult.ErrorMessage)
	if message == "" {
		message = "browser tool returned a recoverable stale element reference error"
	}

	return fmt.Sprintf(
		"The previous browser tool failed with a recoverable error: %s. This is recovery attempt %d/%d. Do not repeat the same ref, selector, html_mode, or browser action blindly. First call browser_observe on the same current work tab to get fresh state, then choose a different safe path if the intended element is missing. If enough evidence is already available, produce the final answer. Do not produce user-visible progress text.",
		message,
		attempt,
		maxAttempts,
	)
}

func repeatedRecoverableToolFailureMessage(step Step, attempt int, maxAttempts int) string {
	message := strings.TrimSpace(step.ToolResult.ErrorMessage)
	if message == "" {
		message = "browser tool returned a repeated recoverable error"
	}

	return fmt.Sprintf("run stopped after repeated recoverable browser tool errors: attempt=%d max=%d last_tool=%s last_error=%s", attempt, maxAttempts, step.ToolName, message)
}

func (o StepOrchestrator) completedResult(
	ctx context.Context,
	command StepRunCommand,
	ledger *Ledger,
	state State,
	step Step,
	runSpan port.Span,
) (RunResult, error) {
	state.Status = RunStatusCompleted
	state.UpdatedAt = o.Clock.Now()

	if strings.TrimSpace(command.TraceInput) != "" {
		o.setTraceOutput(runSpan, step.Text)
	}

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

	o.traceLedgerSummary(ctx, result.LedgerSummary)
	o.traceEvent(runSpan, "agentkit.ledger.summary", ledgerSummaryTraceAttrs(result.LedgerSummary))
	o.traceEvent(runSpan, "agentkit.run.completed", map[string]any{
		"agentkit.run.status":           string(RunStatusCompleted),
		"agentkit.steps_completed":      result.StepsCompleted,
		"agentkit.ledger.total_entries": result.LedgerSummary.TotalEntries,
	})

	if err := o.publish(ctx, port.EventTypeCompleted, command, map[string]any{
		"status": string(RunStatusCompleted),
	}); err != nil {
		return RunResult{}, err
	}

	return result, nil
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
	runSpan port.Span,
) (RunResult, error) {
	state.Status = RunStatusFailed
	state.Failure = &failure
	state.UpdatedAt = o.Clock.Now()

	if strings.TrimSpace(command.TraceInput) != "" {
		o.setTraceOutput(runSpan, failureOutputText(failure))
	}

	if err := state.Validate(); err != nil {
		return RunResult{}, err
	}

	o.traceError(runSpan, failure)
	o.traceLedgerSummary(ctx, ledger.Summary())
	o.traceEvent(runSpan, "agentkit.ledger.summary", ledgerSummaryTraceAttrs(ledger.Summary()))
	o.traceEvent(runSpan, "agentkit.run.failed", failureTraceAttrs(failure))

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

func (o StepOrchestrator) startTrace(ctx context.Context, name string, attrs map[string]any) (context.Context, port.Span) {
	if o.Tracer == nil {
		return ctx, nil
	}

	return o.Tracer.Start(ctx, name, attrs)
}

func (o StepOrchestrator) traceEvent(span port.Span, name string, attrs map[string]any) {
	if span == nil {
		return
	}

	span.AddEvent(name, attrs)
}

func (o StepOrchestrator) traceError(span port.Span, failure Failure) {
	if span == nil {
		return
	}

	span.RecordError(fmt.Errorf("%s: %s", string(failure.Code), failure.Message))
}

func (o StepOrchestrator) setTraceInput(span port.Span, input string) {
	input = strings.TrimSpace(input)
	if span == nil || input == "" {
		return
	}

	span.SetAttributes(map[string]any{
		"langfuse.trace.input": input,
	})
}

func (o StepOrchestrator) setTraceOutput(span port.Span, output string) {
	output = strings.TrimSpace(output)
	if span == nil || output == "" {
		return
	}

	span.SetAttributes(map[string]any{
		"langfuse.trace.output": output,
	})
}

func failureOutputText(failure Failure) string {
	return strings.TrimSpace(fmt.Sprintf("%s: %s", string(failure.Code), failure.Message))
}

func (o StepOrchestrator) traceLedgerSummary(ctx context.Context, summary LedgerSummary) {
	_, span := o.startTrace(ctx, "agentkit.ledger.summary", ledgerSummaryTraceAttrs(summary))
	if span == nil {
		return
	}
	defer span.End()

	span.SetAttributes(map[string]any{
		"langfuse.observation.output": ledgerSummaryText(summary),
	})
}

func ledgerSummaryText(summary LedgerSummary) string {
	return fmt.Sprintf(
		"run_id=%s total_entries=%d steps=%d steps_completed=%d steps_failed=%d steps_blocked=%d tool_calls=%d tool_results=%d tool_errors=%d assistant_texts=%d final_responses=%d last_entry_kind=%s",
		string(summary.RunID),
		summary.TotalEntries,
		summary.Steps,
		summary.StepsCompleted,
		summary.StepsFailed,
		summary.StepsBlocked,
		summary.ToolCalls,
		summary.ToolResults,
		summary.ToolErrors,
		summary.AssistantTexts,
		summary.FinalResponses,
		string(summary.LastEntryKind),
	)
}

func stepObservationAttrs(step Step) map[string]any {
	attrs := map[string]any{
		"langfuse.observation.input": traceStepInput(step),
	}

	if output := traceStepOutput(step); output != "" {
		attrs["langfuse.observation.output"] = output
	}

	return attrs
}

func traceStepInput(step Step) string {
	switch step.Kind {
	case StepKindToolCall:
		return truncateTraceText(fmt.Sprintf("tool=%s args=%v", string(step.ToolName), step.ToolArgs), 4000)

	case StepKindToolResult:
		return truncateTraceText(fmt.Sprintf("tool=%s call_id=%s", string(step.ToolName), step.ToolCallID), 4000)

	case StepKindAssistantText:
		return step.Description

	default:
		return step.Description
	}
}

func traceStepOutput(step Step) string {
	switch step.Kind {
	case StepKindToolResult:
		if !step.ToolResult.OK {
			return truncateTraceText(step.ToolResult.ErrorMessage, 4000)
		}
		return truncateTraceText(fmt.Sprintf("%v", step.ToolResult.Raw), 4000)

	case StepKindAssistantText:
		return truncateTraceText(step.Text, 4000)

	default:
		return ""
	}
}

func truncateTraceText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}

	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}

	return string(runes[:limit]) + fmt.Sprintf("...[truncated %d chars]", len(runes)-limit)
}

func (o StepOrchestrator) traceStep(ctx context.Context, step Step) {
	_, span := o.startTrace(ctx, traceStepName(step), stepTraceAttrs(step))
	if span == nil {
		return
	}
	defer span.End()

	span.SetAttributes(stepObservationAttrs(step))

	if step.Failure != nil {
		o.traceError(span, *step.Failure)
	}

	if step.Kind == StepKindToolResult && !step.ToolResult.OK {
		span.RecordError(fmt.Errorf("%s: %s", string(step.ToolResult.ErrorKind), step.ToolResult.ErrorMessage))
	}
}

func traceStepName(step Step) string {
	switch step.Kind {
	case StepKindToolCall:
		return "agentkit.step.tool_call"

	case StepKindToolResult:
		return "agentkit.step.tool_result"

	case StepKindAssistantText:
		if step.Final {
			return "agentkit.step.final_response"
		}

		return "agentkit.step.assistant_text"

	default:
		return "agentkit.step"
	}
}

func runTraceAttrs(command StepRunCommand) map[string]any {
	return map[string]any{
		"agentkit.run_id":     string(command.RunID),
		"agentkit.session_id": string(command.SessionID),
		"agentkit.max_steps":  command.MaxSteps,
	}
}

func stepTraceAttrs(step Step) map[string]any {
	attrs := map[string]any{
		"agentkit.step_id":          string(step.ID),
		"agentkit.step.kind":        string(step.Kind),
		"agentkit.step.source":      string(step.Source),
		"agentkit.step.status":      string(step.Status),
		"agentkit.step.description": step.Description,
		"agentkit.step.final":       step.Final,
	}

	if strings.TrimSpace(step.ToolCallID) != "" {
		attrs["agentkit.tool_call_id"] = step.ToolCallID
	}

	if strings.TrimSpace(string(step.ToolName)) != "" {
		attrs["agentkit.tool.name"] = string(step.ToolName)
	}

	if step.ToolArgs != nil {
		attrs["agentkit.tool.args_count"] = len(step.ToolArgs)
	}

	if step.Kind == StepKindToolResult {
		attrs["agentkit.tool_result.ok"] = step.ToolResult.OK
		attrs["agentkit.tool_result.has_evidence"] = step.ToolResult.HasEvidence

		if !step.ToolResult.OK {
			attrs["agentkit.tool_result.error_kind"] = string(step.ToolResult.ErrorKind)
			attrs["agentkit.tool_result.error_message"] = step.ToolResult.ErrorMessage
		}
	}

	if step.Kind == StepKindAssistantText {
		attrs["agentkit.assistant_text.len"] = len([]rune(step.Text))
	}

	if step.Failure != nil {
		attrs["agentkit.failure.code"] = string(step.Failure.Code)
		attrs["agentkit.failure.message"] = step.Failure.Message
	}

	return attrs
}

func failureTraceAttrs(failure Failure) map[string]any {
	return map[string]any{
		"agentkit.run.status":      string(RunStatusFailed),
		"agentkit.failure.code":    string(failure.Code),
		"agentkit.failure.message": failure.Message,
	}
}

func ledgerSummaryTraceAttrs(summary LedgerSummary) map[string]any {
	return map[string]any{
		"agentkit.run_id":                 string(summary.RunID),
		"agentkit.ledger.total_entries":   summary.TotalEntries,
		"agentkit.ledger.steps":           summary.Steps,
		"agentkit.ledger.steps_completed": summary.StepsCompleted,
		"agentkit.ledger.steps_failed":    summary.StepsFailed,
		"agentkit.ledger.steps_blocked":   summary.StepsBlocked,
		"agentkit.ledger.route_decisions": summary.RouteDecisions,
		"agentkit.ledger.tool_calls":      summary.ToolCalls,
		"agentkit.ledger.tool_results":    summary.ToolResults,
		"agentkit.ledger.tool_errors":     summary.ToolErrors,
		"agentkit.ledger.assistant_texts": summary.AssistantTexts,
		"agentkit.ledger.final_responses": summary.FinalResponses,
		"agentkit.ledger.notes":           summary.Notes,
		"agentkit.ledger.last_entry_id":   string(summary.LastEntryID),
		"agentkit.ledger.last_entry_kind": string(summary.LastEntryKind),
	}
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
