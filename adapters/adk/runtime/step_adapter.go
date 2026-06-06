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

	if explicitOK, exists, err := explicitBool(m, "ok"); err != nil {
		return failedToolResult(coreruntime.ToolErrorValidation, err.Error(), raw)
	} else if exists && !explicitOK {
		message := explicitToolErrorMessage(m)
		return failedToolResult(classifyStructuredToolErrorKind(explicitToolErrorKind(m), message), message, raw)
	}

	if explicitSuccess, exists, err := explicitBool(m, "success"); err != nil {
		return failedToolResult(coreruntime.ToolErrorValidation, err.Error(), raw)
	} else if exists && !explicitSuccess {
		message := explicitToolErrorMessage(m)
		return failedToolResult(classifyStructuredToolErrorKind(explicitToolErrorKind(m), message), message, raw)
	}

	if kind := explicitToolErrorKind(m); kind != coreruntime.ToolErrorNone {
		return failedToolResult(kind, explicitToolErrorMessage(m), raw)
	}

	if message := explicitStructuredErrorMessage(m); message != "" {
		return failedToolResult(classifyStructuredToolErrorKind(coreruntime.ToolErrorNone, message), message, raw)
	}

	return coreruntime.ToolExecutionResult{
		OK:          true,
		HasEvidence: len(m) > 0,
		Raw:         raw,
	}
}

func explicitBool(m map[string]any, key string) (bool, bool, error) {
	value, exists := m[key]
	if !exists {
		return false, false, nil
	}

	typed, ok := value.(bool)
	if !ok {
		return false, true, fmt.Errorf("tool response field %q must be boolean", key)
	}

	return typed, true, nil
}

func failedToolResult(kind coreruntime.ToolErrorKind, message string, raw any) coreruntime.ToolExecutionResult {
	if kind == coreruntime.ToolErrorNone {
		kind = coreruntime.ToolErrorFatal
	}

	message = strings.TrimSpace(message)
	if message == "" {
		message = "tool returned explicit failure"
	}

	return coreruntime.ToolExecutionResult{
		OK:           false,
		HasEvidence:  false,
		ErrorKind:    kind,
		ErrorMessage: message,
		Raw:          raw,
	}
}

func explicitToolErrorKind(m map[string]any) coreruntime.ToolErrorKind {
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
		return coreruntime.ToolErrorFatal
	}
}

func explicitToolErrorMessage(m map[string]any) string {
	for _, key := range []string{
		"error_message",
		"error",
		"error_details",
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

func classifyStructuredToolErrorKind(explicit coreruntime.ToolErrorKind, message string) coreruntime.ToolErrorKind {
	switch explicit {
	case coreruntime.ToolErrorValidation, coreruntime.ToolErrorAuth, coreruntime.ToolErrorClientHold:
		return explicit
	case coreruntime.ToolErrorNone, coreruntime.ToolErrorFatal:
		// Continue below: legacy wrappers often omit error_kind or use fatal for input-contract errors.
	default:
		return coreruntime.ToolErrorFatal
	}

	message = strings.TrimSpace(strings.ToLower(message))
	if message == "" {
		if explicit == coreruntime.ToolErrorFatal {
			return coreruntime.ToolErrorFatal
		}
		return coreruntime.ToolErrorFatal
	}

	if isStructuredValidationToolError(message) {
		return coreruntime.ToolErrorValidation
	}

	if isStructuredBrowserRuntimeToolError(message) {
		return coreruntime.ToolErrorClientHold
	}

	if explicit == coreruntime.ToolErrorFatal {
		return coreruntime.ToolErrorFatal
	}

	return coreruntime.ToolErrorFatal
}
func isStructuredValidationToolError(message string) bool {
	needles := []string{
		"validating root",
		"unexpected additional properties",
		"additional property",
		"missing required",
		"required field",
		"cannot unmarshal",
		"invalid argument",
		"invalid arguments",
		"invalid_input",
		"unsupported html mode",
		"unsupported mode",
		"schema",
	}

	for _, needle := range needles {
		if strings.Contains(message, needle) {
			return true
		}
	}

	return false
}

func isStructuredBrowserRuntimeToolError(message string) bool {
	needles := []string{
		"element_not_found",
		"stale_element_ref",
		"browser_not_initialized",
		"tab is closed",
		"browser operation timed out",
		"context canceled",
	}

	for _, needle := range needles {
		if strings.Contains(message, needle) {
			return true
		}
	}

	return false
}

func explicitStructuredErrorMessage(m map[string]any) string {
	if _, exists := m["error"]; exists {
		return explicitToolErrorMessage(m)
	}

	if _, exists := m["error_message"]; exists {
		return explicitToolErrorMessage(m)
	}

	if _, exists := m["error_details"]; exists {
		return explicitToolErrorMessage(m)
	}

	return ""
}
