package runtime

import (
	"fmt"
	"strings"

	"github.com/bogachenko/agentkit/core/port"
	"github.com/bogachenko/agentkit/core/tool"
)

// StateUpdater applies normalized steps to run state without knowing where steps came from.
type StateUpdater struct {
	Clock       port.Clock
	IDGenerator port.IDGenerator
}

// Apply enriches, validates, records, and applies one step to state.
func (u StateUpdater) Apply(state *State, step Step) (Step, error) {
	if state == nil {
		return Step{}, fmt.Errorf("runtime state is required")
	}

	prepared, err := u.prepare(step)
	if err != nil {
		return Step{}, err
	}

	if err := prepared.Validate(); err != nil {
		return Step{}, err
	}

	state.Steps = append(state.Steps, prepared)
	state.StepCount = len(state.Steps)
	state.UpdatedAt = prepared.FinishedAt

	switch prepared.Kind {
	case StepKindToolCall:
		state.ToolCalls++
		state.LastToolName = toolNameString(prepared.ToolName)

	case StepKindToolResult:
		state.ToolResults++
		state.LastToolName = toolNameString(prepared.ToolName)

		if prepared.ToolResult.OK {
			state.LastToolError = ""
			state.LastToolErrorKind = ToolErrorNone

			if prepared.ToolResult.HasEvidence {
				state.EvidenceCount++
			}

			return prepared, nil
		}

		state.ToolErrors++
		state.LastToolError = prepared.ToolResult.ErrorMessage
		state.LastToolErrorKind = prepared.ToolResult.ErrorKind

	case StepKindAssistantText:
		if prepared.Final {
			state.FinalText = prepared.Text
		}
	}

	return prepared, nil
}

// prepare fills deterministic runtime metadata that adapters must not invent.
func (u StateUpdater) prepare(step Step) (Step, error) {
	if u.Clock == nil {
		return Step{}, fmt.Errorf("state updater clock is required")
	}

	if u.IDGenerator == nil {
		return Step{}, fmt.Errorf("state updater id generator is required")
	}

	if strings.TrimSpace(string(step.ID)) == "" {
		step.ID = StepID(u.IDGenerator.NewID())
	}

	now := u.Clock.Now()

	if step.StartedAt.IsZero() {
		step.StartedAt = now
	}

	if step.FinishedAt.IsZero() {
		step.FinishedAt = now
	}

	if step.Status == "" {
		step.Status = StepStatusCompleted
	}

	if step.Kind == "" {
		step.Kind = StepKindGeneric
	}

	if strings.TrimSpace(step.Description) == "" {
		step.Description = describeStep(step)
	}

	if step.Kind == StepKindToolCall && step.ToolArgs == nil {
		step.ToolArgs = map[string]any{}
	}

	return step, nil
}

func describeStep(step Step) string {
	switch step.Kind {
	case StepKindToolCall:
		return "model called tool " + string(step.ToolName)

	case StepKindToolResult:
		if step.ToolResult.OK {
			return "tool returned result " + string(step.ToolName)
		}

		return "tool returned error " + string(step.ToolName)

	case StepKindAssistantText:
		if step.Final {
			return "assistant produced final response"
		}

		return "assistant produced text"

	case StepKindEmpty:
		return "empty runtime event"

	case StepKindStreamDone:
		return "runtime stream completed"

	default:
		if strings.TrimSpace(string(step.ToolName)) != "" {
			return "runtime step for tool " + string(step.ToolName)
		}

		return "runtime step"
	}
}

func toolFailureFromResult(name tool.Name, result ToolExecutionResult) Failure {
	message := result.ErrorMessage
	if strings.TrimSpace(message) == "" {
		message = "tool " + string(name) + " failed"
	}

	return Failure{
		Code:    FailureCodeToolFailed,
		Message: message,
	}
}
