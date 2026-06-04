package runtime

import (
	"testing"

	coreruntime "github.com/bogachenko/agentkit/core/runtime"
)

func TestToolResultFromFunctionResponseTreatsNormalMessageAsSuccess(t *testing.T) {
	result := toolResultFromFunctionResponse(map[string]any{
		"message": "page loaded",
		"title":   "Example Domain",
	})

	if !result.OK {
		t.Fatalf("expected success, got error kind=%s message=%q", result.ErrorKind, result.ErrorMessage)
	}

	if !result.HasEvidence {
		t.Fatal("expected evidence")
	}
}

func TestToolResultFromFunctionResponseUsesExplicitOKFalse(t *testing.T) {
	result := toolResultFromFunctionResponse(map[string]any{
		"ok":            false,
		"error_kind":    string(coreruntime.ToolErrorValidation),
		"error_message": "request validation failed",
	})

	if result.OK {
		t.Fatal("expected failure")
	}

	if result.ErrorKind != coreruntime.ToolErrorValidation {
		t.Fatalf("expected validation error, got %s", result.ErrorKind)
	}

	if result.ErrorMessage != "request validation failed" {
		t.Fatalf("unexpected error message: %q", result.ErrorMessage)
	}
}

func TestToolResultFromFunctionResponseUsesExplicitSuccessFalse(t *testing.T) {
	result := toolResultFromFunctionResponse(map[string]any{
		"success": false,
		"error":   "browser unavailable",
	})

	if result.OK {
		t.Fatal("expected failure")
	}

	if result.ErrorKind != coreruntime.ToolErrorFatal {
		t.Fatalf("expected fatal error, got %s", result.ErrorKind)
	}

	if result.ErrorMessage != "browser unavailable" {
		t.Fatalf("unexpected error message: %q", result.ErrorMessage)
	}
}

func TestToolResultFromFunctionResponseRejectsInvalidOKType(t *testing.T) {
	result := toolResultFromFunctionResponse(map[string]any{
		"ok": "false",
	})

	if result.OK {
		t.Fatal("expected failure")
	}

	if result.ErrorKind != coreruntime.ToolErrorValidation {
		t.Fatalf("expected validation error, got %s", result.ErrorKind)
	}
}

func TestToolResultFromFunctionResponseDoesNotClassifyByErrorText(t *testing.T) {
	result := toolResultFromFunctionResponse(map[string]any{
		"ok":            false,
		"error_message": "auth token missing",
	})

	if result.OK {
		t.Fatal("expected failure")
	}

	if result.ErrorKind != coreruntime.ToolErrorFatal {
		t.Fatalf("expected default fatal error without explicit error_kind, got %s", result.ErrorKind)
	}
}
