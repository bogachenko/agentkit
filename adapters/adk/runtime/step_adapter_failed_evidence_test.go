package runtime

import (
	"testing"

	coreruntime "github.com/bogachenko/agentkit/core/runtime"
)

func TestFailedToolResultWithRawStructuredPayloadHasEvidence(t *testing.T) {
	raw := map[string]any{
		"error": "invalid_input: resource exceeds max_bytes limit",
	}

	result := failedToolResult(coreruntime.ToolErrorValidation, "invalid_input: resource exceeds max_bytes limit", raw)

	if result.OK {
		t.Fatalf("expected failed tool result")
	}
	if !result.HasEvidence {
		t.Fatalf("expected failed tool result with raw payload to count as evidence")
	}
	if result.ErrorKind != coreruntime.ToolErrorValidation {
		t.Fatalf("expected validation error kind, got %q", result.ErrorKind)
	}
}

func TestFailedToolResultWithoutRawPayloadHasNoEvidence(t *testing.T) {
	result := failedToolResult(coreruntime.ToolErrorValidation, "validation failed", nil)

	if result.OK {
		t.Fatalf("expected failed tool result")
	}
	if result.HasEvidence {
		t.Fatalf("expected failed tool result without raw payload to have no evidence")
	}
}

func TestToolResultFromFunctionResponseValidationFailureHasEvidence(t *testing.T) {
	result := toolResultFromFunctionResponse(map[string]any{
		"ok":    false,
		"error": "invalid_input: resource exceeds max_bytes limit",
	})

	if result.OK {
		t.Fatalf("expected failed tool result")
	}
	if !result.HasEvidence {
		t.Fatalf("expected validation tool failure to count as evidence")
	}
}
