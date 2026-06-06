package runtime

import (
	"testing"

	coreruntime "github.com/bogachenko/agentkit/core/runtime"
)

func TestToolResultFromFunctionResponseClassifiesSchemaValidationError(t *testing.T) {
	result := toolResultFromFunctionResponse(map[string]any{
		"error": `validating root: unexpected additional properties ["selector"]`,
	})

	if result.OK {
		t.Fatalf("expected failed tool result")
	}

	if result.ErrorKind != coreruntime.ToolErrorValidation {
		t.Fatalf("expected validation error kind, got %q", result.ErrorKind)
	}

	if result.ErrorMessage == "" {
		t.Fatalf("expected validation error message")
	}
}

func TestToolResultFromFunctionResponseClassifiesGenericSchemaErrors(t *testing.T) {
	cases := []string{
		"json schema validation failed: missing required property query",
		"cannot unmarshal string into Go struct field input.limit of type int",
		"invalid arguments: field max_results must be integer",
	}

	for _, message := range cases {
		result := toolResultFromFunctionResponse(map[string]any{
			"error": message,
		})

		if result.ErrorKind != coreruntime.ToolErrorValidation {
			t.Fatalf("message %q: expected validation error kind, got %q", message, result.ErrorKind)
		}
	}
}

func TestToolResultFromFunctionResponseKeepsNonValidationStructuredErrorFatal(t *testing.T) {
	result := toolResultFromFunctionResponse(map[string]any{
		"error": "browser process crashed while executing tool",
	})

	if result.OK {
		t.Fatalf("expected failed tool result")
	}

	if result.ErrorKind != coreruntime.ToolErrorFatal {
		t.Fatalf("expected fatal error kind, got %q", result.ErrorKind)
	}
}
