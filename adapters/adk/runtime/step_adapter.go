package runtime

import (
	"fmt"
	"strings"

	coreruntime "github.com/bogachenko/agentkit/core/runtime"
	"github.com/bogachenko/agentkit/core/tool"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"
)

// StepsFromADKEvent converts one ADK event into zero or more neutral runtime steps.
func StepsFromADKEvent(event *adksession.Event) []coreruntime.Step {
	if event == nil || event.Content == nil {
		return nil
	}

	final := event.IsFinalResponse()
	steps := make([]coreruntime.Step, 0, len(event.Content.Parts))

	for _, part := range event.Content.Parts {
		if part == nil {
			continue
		}

		if part.FunctionCall != nil {
			steps = append(steps, coreruntime.Step{
				Kind:        coreruntime.StepKindToolCall,
				Source:      coreruntime.StepSourceModel,
				Status:      coreruntime.StepStatusCompleted,
				ToolCallID:  strings.TrimSpace(part.FunctionCall.ID),
				ToolName:    tool.Name(part.FunctionCall.Name),
				ToolArgs:    normalizeToolArgs(part.FunctionCall.Args),
				Description: "model called tool " + part.FunctionCall.Name,
			})
		}

		if part.FunctionResponse != nil {
			steps = append(steps, coreruntime.Step{
				Kind:        coreruntime.StepKindToolResult,
				Source:      coreruntime.StepSourceTool,
				Status:      coreruntime.StepStatusCompleted,
				ToolCallID:  strings.TrimSpace(part.FunctionResponse.ID),
				ToolName:    tool.Name(part.FunctionResponse.Name),
				ToolResult:  toolResultFromFunctionResponse(part.FunctionResponse.Response),
				Description: "tool returned result " + part.FunctionResponse.Name,
			})
		}

		if strings.TrimSpace(part.Text) != "" {
			steps = append(steps, coreruntime.Step{
				Kind:        coreruntime.StepKindAssistantText,
				Source:      sourceFromRole(event.Content.Role),
				Status:      coreruntime.StepStatusCompleted,
				Text:        strings.TrimSpace(part.Text),
				Final:       final,
				Description: assistantTextDescription(final),
			})
		}
	}

	if len(steps) == 0 && final {
		return []coreruntime.Step{
			{
				Kind:        coreruntime.StepKindStreamDone,
				Source:      coreruntime.StepSourceRuntime,
				Status:      coreruntime.StepStatusCompleted,
				Final:       true,
				Description: "ADK stream completed without text",
			},
		}
	}

	return steps
}

func normalizeToolArgs(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}

	return args
}

func sourceFromRole(role string) coreruntime.StepSource {
	switch role {
	case genai.RoleUser:
		return coreruntime.StepSourceUser
	case genai.RoleModel:
		return coreruntime.StepSourceModel
	case "tool":
		return coreruntime.StepSourceTool
	default:
		return coreruntime.StepSourceRuntime
	}
}

func assistantTextDescription(final bool) string {
	if final {
		return "assistant produced final response"
	}

	return "assistant produced text"
}

func toolResultFromFunctionResponse(raw any) coreruntime.ToolExecutionResult {
	if raw == nil {
		return coreruntime.ToolExecutionResult{
			OK:          true,
			HasEvidence: false,
			Raw:         raw,
		}
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return coreruntime.ToolExecutionResult{
			OK:          true,
			HasEvidence: true,
			Raw:         raw,
		}
	}

	if errText := extractToolErrorMessage(m); errText != "" {
		return coreruntime.ToolExecutionResult{
			OK:           false,
			HasEvidence:  false,
			ErrorKind:    classifyToolError(m),
			ErrorMessage: errText,
			Raw:          raw,
		}
	}

	return coreruntime.ToolExecutionResult{
		OK:          true,
		HasEvidence: len(m) > 0,
		Raw:         raw,
	}
}

func extractToolErrorMessage(m map[string]any) string {
	for _, key := range []string{
		"error_details",
		"error",
		"error_message",
		"message",
	} {
		value, exists := m[key]
		if !exists || value == nil {
			continue
		}

		text := strings.TrimSpace(fmt.Sprintf("%v", value))
		if text != "" {
			return text
		}
	}

	return ""
}

func classifyToolError(m map[string]any) coreruntime.ToolErrorKind {
	if kind := structuredErrorKind(m); kind != coreruntime.ToolErrorNone {
		return kind
	}

	errorText := strings.ToLower(extractToolErrorMessage(m))

	if strings.Contains(errorText, "client tool call timed out") ||
		strings.Contains(errorText, "no sse subscriber") ||
		strings.Contains(errorText, "client tool call is not waiting") {
		return coreruntime.ToolErrorClientHold
	}

	if strings.Contains(errorText, "auth") ||
		strings.Contains(errorText, "authorization") ||
		strings.Contains(errorText, "api-key") ||
		strings.Contains(errorText, "client-id") ||
		strings.Contains(errorText, "token") ||
		strings.Contains(errorText, "credentials") {
		return coreruntime.ToolErrorAuth
	}

	if strings.Contains(errorText, "validation") ||
		strings.Contains(errorText, "missing properties") ||
		strings.Contains(errorText, "request validation failed") {
		return coreruntime.ToolErrorValidation
	}

	if _, exists := m["error_details"]; exists {
		return coreruntime.ToolErrorValidation
	}

	if _, exists := m["reflection_guidance"]; exists {
		return coreruntime.ToolErrorValidation
	}

	return coreruntime.ToolErrorFatal
}

func structuredErrorKind(m map[string]any) coreruntime.ToolErrorKind {
	value, exists := m["error_kind"]
	if !exists || value == nil {
		return coreruntime.ToolErrorNone
	}

	switch strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", value))) {
	case string(coreruntime.ToolErrorValidation):
		return coreruntime.ToolErrorValidation
	case string(coreruntime.ToolErrorAuth):
		return coreruntime.ToolErrorAuth
	case string(coreruntime.ToolErrorClientHold):
		return coreruntime.ToolErrorClientHold
	case string(coreruntime.ToolErrorFatal):
		return coreruntime.ToolErrorFatal
	default:
		return coreruntime.ToolErrorNone
	}
}
