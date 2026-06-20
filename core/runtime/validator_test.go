package runtime

import (
	"errors"
	"testing"
	"time"

	"github.com/bogachenko/agentkit/core/tool"
)

func runtimeTestToolContract(name tool.Name, readOnly bool, requiresApproval bool) tool.Contract {
	return tool.Contract{
		Name:        name,
		Description: "Runtime validator test tool.",
		InputSchema: map[string]any{
			"type": "object",
		},
		OutputSchema: map[string]any{
			"type": "object",
		},
		ReadOnly:         readOnly,
		RequiresApproval: requiresApproval,
	}
}

func runtimeTestState() State {
	startedAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	return State{
		RunID:     RunID("run-1"),
		Status:    RunStatusRunning,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
}

func runtimeTestApproval(status ApprovalStatus, args map[string]any) Approval {
	argsHash, err := NewToolArgsHash(args)
	if err != nil {
		panic(err)
	}

	return Approval{
		ID:           ApprovalID("approval-1"),
		RunID:        RunID("run-1"),
		ToolName:     tool.Name("write_product"),
		ToolArgsHash: argsHash,
		Status:       status,
		Reason:       "User explicitly approved this tool.",
		CreatedAt:    time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
	}
}

func TestApprovalValidateAcceptsValidApproval(t *testing.T) {
	approval := runtimeTestApproval(ApprovalStatusApproved, nil)

	if err := approval.Validate(); err != nil {
		t.Fatalf("expected valid approval, got error: %v", err)
	}
}

func TestApprovalValidateRejectsMissingReason(t *testing.T) {
	approval := runtimeTestApproval(ApprovalStatusApproved, nil)
	approval.Reason = "   "

	if err := approval.Validate(); err == nil {
		t.Fatal("expected error for missing approval reason")
	}
}

func TestPolicyToolContractFindsRegisteredTool(t *testing.T) {
	policy := Policy{
		ToolContracts: []tool.Contract{
			runtimeTestToolContract(tool.Name("read_products"), true, false),
		},
	}

	contract, exists, err := policy.ToolContract(tool.Name("read_products"))
	if err != nil {
		t.Fatalf("expected lookup to succeed, got error: %v", err)
	}

	if !exists {
		t.Fatal("expected contract to exist")
	}

	if contract.Name != tool.Name("read_products") {
		t.Fatalf("expected read_products, got %q", contract.Name)
	}
}

func TestPolicyToolContractReturnsMissingForUnknownTool(t *testing.T) {
	policy := Policy{
		ToolContracts: []tool.Contract{
			runtimeTestToolContract(tool.Name("read_products"), true, false),
		},
	}

	_, exists, err := policy.ToolContract(tool.Name("unknown_tool"))
	if err != nil {
		t.Fatalf("expected lookup to succeed, got error: %v", err)
	}

	if exists {
		t.Fatal("expected unknown tool to be missing")
	}
}

func TestValidatorValidateDecisionAllowsReadOnlyToolWithoutApproval(t *testing.T) {
	validator := Validator{
		Policy: Policy{
			ToolContracts: []tool.Contract{
				runtimeTestToolContract(tool.Name("read_products"), true, false),
			},
		},
	}

	err := validator.ValidateDecision(ValidationInput{
		State: runtimeTestState(),
		Decision: RouteDecision{
			Kind:     RouteKindCallTool,
			ToolName: tool.Name("read_products"),
			Reason:   "Model requested an explicit read-only tool call.",
		},
	})

	if err != nil {
		t.Fatalf("expected decision to be allowed, got error: %v", err)
	}
}

func TestValidatorValidateDecisionRejectsUnknownTool(t *testing.T) {
	validator := Validator{
		Policy: Policy{
			ToolContracts: []tool.Contract{
				runtimeTestToolContract(tool.Name("read_products"), true, false),
			},
		},
	}

	err := validator.ValidateDecision(ValidationInput{
		State: runtimeTestState(),
		Decision: RouteDecision{
			Kind:     RouteKindCallTool,
			ToolName: tool.Name("unknown_tool"),
			Reason:   "Model requested an explicit tool call.",
		},
	})

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Code != ValidationCodeUnknownTool {
		t.Fatalf("expected unknown_tool code, got %q", validationErr.Code)
	}
}

func TestValidatorValidateDecisionRejectsApprovalRequiredToolWithoutApproval(t *testing.T) {
	validator := Validator{
		Policy: Policy{
			ToolContracts: []tool.Contract{
				runtimeTestToolContract(tool.Name("write_product"), false, true),
			},
		},
	}

	err := validator.ValidateDecision(ValidationInput{
		State: runtimeTestState(),
		Decision: RouteDecision{
			Kind:     RouteKindCallTool,
			ToolName: tool.Name("write_product"),
			Reason:   "Model requested an explicit write tool call.",
		},
	})

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Code != ValidationCodeApprovalRequired {
		t.Fatalf("expected approval_required code, got %q", validationErr.Code)
	}
}

func TestValidatorValidateDecisionAllowsApprovalRequiredToolWithApprovedApproval(t *testing.T) {
	validator := Validator{
		Policy: Policy{
			ToolContracts: []tool.Contract{
				runtimeTestToolContract(tool.Name("write_product"), false, true),
			},
		},
	}

	err := validator.ValidateDecision(ValidationInput{
		State: runtimeTestState(),
		Decision: RouteDecision{
			Kind:     RouteKindCallTool,
			ToolName: tool.Name("write_product"),
			Reason:   "Model requested an explicit write tool call after approval.",
		},
		Approvals: []Approval{
			runtimeTestApproval(ApprovalStatusApproved, nil),
		},
	})

	if err != nil {
		t.Fatalf("expected approved decision to be allowed, got error: %v", err)
	}
}

func TestValidatorValidateDecisionRejectsApprovalRequiredToolWithDifferentArgsApproval(t *testing.T) {
	validator := Validator{
		Policy: Policy{
			ToolContracts: []tool.Contract{
				runtimeTestToolContract(tool.Name("write_product"), false, true),
			},
		},
	}

	err := validator.ValidateDecision(ValidationInput{
		State: runtimeTestState(),
		Decision: RouteDecision{
			Kind:     RouteKindCallTool,
			ToolName: tool.Name("write_product"),
			ToolArgs: map[string]any{"sku": "B"},
			Reason:   "Model requested an explicit write tool call after approval.",
		},
		Approvals: []Approval{
			runtimeTestApproval(ApprovalStatusApproved, map[string]any{"sku": "A"}),
		},
	})

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Code != ValidationCodeApprovalRequired {
		t.Fatalf("expected approval_required code, got %q", validationErr.Code)
	}
}

func TestValidatorValidateDecisionRejectsApprovalRequiredToolWithRejectedApproval(t *testing.T) {
	validator := Validator{
		Policy: Policy{
			ToolContracts: []tool.Contract{
				runtimeTestToolContract(tool.Name("write_product"), false, true),
			},
		},
	}

	err := validator.ValidateDecision(ValidationInput{
		State: runtimeTestState(),
		Decision: RouteDecision{
			Kind:     RouteKindCallTool,
			ToolName: tool.Name("write_product"),
			Reason:   "Model requested an explicit write tool call after approval.",
		},
		Approvals: []Approval{
			runtimeTestApproval(ApprovalStatusRejected, nil),
		},
	})

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Code != ValidationCodeApprovalRejected {
		t.Fatalf("expected approval_rejected code, got %q", validationErr.Code)
	}
}

func TestValidatorValidateDecisionAllowsApprovalRequestForApprovalRequiredTool(t *testing.T) {
	validator := Validator{
		Policy: Policy{
			ToolContracts: []tool.Contract{
				runtimeTestToolContract(tool.Name("write_product"), false, true),
			},
		},
	}

	err := validator.ValidateDecision(ValidationInput{
		State: runtimeTestState(),
		Decision: RouteDecision{
			Kind:     RouteKindRequireApproval,
			ToolName: tool.Name("write_product"),
			Reason:   "Tool requires user approval before execution.",
		},
	})

	if err != nil {
		t.Fatalf("expected approval request to be allowed, got error: %v", err)
	}
}

func TestValidatorValidateDecisionRejectsApprovalRequestForReadOnlyTool(t *testing.T) {
	validator := Validator{
		Policy: Policy{
			ToolContracts: []tool.Contract{
				runtimeTestToolContract(tool.Name("read_products"), true, false),
			},
		},
	}

	err := validator.ValidateDecision(ValidationInput{
		State: runtimeTestState(),
		Decision: RouteDecision{
			Kind:     RouteKindRequireApproval,
			ToolName: tool.Name("read_products"),
			Reason:   "Tool requires user approval before execution.",
		},
	})

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Code != ValidationCodePolicyViolation {
		t.Fatalf("expected policy_violation code, got %q", validationErr.Code)
	}
}

func TestValidatorValidateDecisionRejectsApprovalWithDifferentRunID(t *testing.T) {
	validator := Validator{
		Policy: Policy{
			ToolContracts: []tool.Contract{
				runtimeTestToolContract(tool.Name("write_product"), false, true),
			},
		},
	}

	approval := runtimeTestApproval(ApprovalStatusApproved, nil)
	approval.RunID = RunID("another-run")

	err := validator.ValidateDecision(ValidationInput{
		State: runtimeTestState(),
		Decision: RouteDecision{
			Kind:     RouteKindCallTool,
			ToolName: tool.Name("write_product"),
			Reason:   "Model requested an explicit write tool call after approval.",
		},
		Approvals: []Approval{
			approval,
		},
	})

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Code != ValidationCodeInvalidState {
		t.Fatalf("expected invalid_state code, got %q", validationErr.Code)
	}
}

func TestValidatorBlockedDecisionConvertsValidationError(t *testing.T) {
	validator := Validator{}

	decision := validator.BlockedDecision(ValidationError{
		Code:    ValidationCodeApprovalRequired,
		Message: "approval is required",
	})

	if decision.Kind != RouteKindBlocked {
		t.Fatalf("expected blocked decision, got %q", decision.Kind)
	}

	if decision.Failure == nil {
		t.Fatal("expected blocked decision failure")
	}

	if decision.Failure.Code != FailureCodeApprovalRequired {
		t.Fatalf("expected approval_required failure, got %q", decision.Failure.Code)
	}
}
