package runtime

import (
	"testing"

	coreruntime "github.com/bogachenko/agentkit/core/runtime"
)

func TestToolResultFromFunctionResponseClassifiesInvalidInputAsValidation(t *testing.T) {
	result := toolResultFromFunctionResponse(map[string]any{
		"error": "invalid_input: unsupported html mode",
	})

	if result.OK {
		t.Fatalf("expected failed tool result")
	}
	if result.ErrorKind != coreruntime.ToolErrorValidation {
		t.Fatalf("expected validation error kind, got %q", result.ErrorKind)
	}
	if result.ErrorMessage != "invalid_input: unsupported html mode" {
		t.Fatalf("unexpected error message: %q", result.ErrorMessage)
	}
}

func TestToolResultFromFunctionResponseClassifiesSchemaErrorAsValidation(t *testing.T) {
	result := toolResultFromFunctionResponse(map[string]any{
		"error": `validating root: unexpected additional properties ["selector"]`,
	})

	if result.OK {
		t.Fatalf("expected failed tool result")
	}
	if result.ErrorKind != coreruntime.ToolErrorValidation {
		t.Fatalf("expected validation error kind, got %q", result.ErrorKind)
	}
}

func TestToolResultFromFunctionResponseClassifiesBrowserRuntimeErrorAsClientHold(t *testing.T) {
	result := toolResultFromFunctionResponse(map[string]any{
		"error": "element_not_found: element was not found",
	})

	if result.OK {
		t.Fatalf("expected failed tool result")
	}
	if result.ErrorKind != coreruntime.ToolErrorClientHold {
		t.Fatalf("expected client_hold error kind, got %q", result.ErrorKind)
	}
}

func TestToolResultFromFunctionResponseKeepsUnknownStructuredErrorFatal(t *testing.T) {
	result := toolResultFromFunctionResponse(map[string]any{
		"error": "remote provider returned malformed response",
	})

	if result.OK {
		t.Fatalf("expected failed tool result")
	}
	if result.ErrorKind != coreruntime.ToolErrorFatal {
		t.Fatalf("expected fatal error kind, got %q", result.ErrorKind)
	}
}
