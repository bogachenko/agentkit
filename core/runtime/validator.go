package runtime

import (
	"fmt"

	"github.com/bogachenko/agentkit/core/tool"
)

type Validator struct {
	Policy Policy
}

type ValidationInput struct {
	State     State
	Decision  RouteDecision
	Approvals []Approval
}

func (v Validator) ValidateDecision(input ValidationInput) error {
	if err := input.State.Validate(); err != nil {
		return ValidationError{Code: ValidationCodeInvalidState, Message: err.Error()}
	}
	if err := input.Decision.Validate(); err != nil {
		return ValidationError{Code: ValidationCodeInvalidDecision, Message: err.Error()}
	}
	for index, approval := range input.Approvals {
		if err := approval.Validate(); err != nil {
			return ValidationError{Code: ValidationCodeInvalidState, Message: fmt.Sprintf("approval %d: %v", index, err)}
		}
		if approval.RunID != input.State.RunID {
			return ValidationError{Code: ValidationCodeInvalidState, Message: fmt.Sprintf("approval %q run id does not match state run id", string(approval.ID))}
		}
	}
	switch input.Decision.Kind {
	case RouteKindCallTool:
		return v.validateToolCall(input.State.RunID, input.Decision.ToolName, input.Decision.ToolArgs, input.Approvals)
	case RouteKindRequireApproval:
		return v.validateApprovalRequest(input.Decision.ToolName)
	case RouteKindRespond, RouteKindComplete, RouteKindBlocked:
		return nil
	default:
		return ValidationError{Code: ValidationCodeInvalidDecision, Message: fmt.Sprintf("unsupported route kind %q", string(input.Decision.Kind))}
	}
}

func (v Validator) BlockedDecision(err error) RouteDecision {
	if err == nil {
		return RouteDecision{}
	}
	validationErr, ok := err.(ValidationError)
	if !ok {
		validationErr = ValidationError{Code: ValidationCodePolicyViolation, Message: err.Error()}
	}
	failure := validationErr.Failure()
	return RouteDecision{Kind: RouteKindBlocked, Reason: "Runtime validation blocked the route decision.", Failure: &failure}
}

func (v Validator) validateToolCall(runID RunID, name tool.Name, args map[string]any, approvals []Approval) error {
	contract, exists, err := v.Policy.ToolContract(name)
	if err != nil {
		return err
	}
	if !exists {
		return ValidationError{Code: ValidationCodeUnknownTool, Message: fmt.Sprintf("tool %q is not registered", string(name))}
	}
	if !contract.RequiresApproval {
		return nil
	}
	argsHash, err := NewToolArgsHash(args)
	if err != nil {
		return ValidationError{Code: ValidationCodeInvalidDecision, Message: err.Error()}
	}
	approval, exists := findApproval(runID, name, argsHash, approvals)
	if !exists {
		return ValidationError{Code: ValidationCodeApprovalRequired, Message: fmt.Sprintf("tool %q requires approval", string(name))}
	}
	if approval.Status == ApprovalStatusRejected {
		return ValidationError{Code: ValidationCodeApprovalRejected, Message: fmt.Sprintf("approval %q rejected tool %q", string(approval.ID), string(name))}
	}
	if !approval.IsApproved() {
		return ValidationError{Code: ValidationCodeApprovalRequired, Message: fmt.Sprintf("tool %q approval is not approved", string(name))}
	}
	return nil
}

func (v Validator) validateApprovalRequest(name tool.Name) error {
	contract, exists, err := v.Policy.ToolContract(name)
	if err != nil {
		return err
	}
	if !exists {
		return ValidationError{Code: ValidationCodeUnknownTool, Message: fmt.Sprintf("tool %q is not registered", string(name))}
	}
	if !contract.RequiresApproval {
		return ValidationError{Code: ValidationCodePolicyViolation, Message: fmt.Sprintf("tool %q does not require approval", string(name))}
	}
	return nil
}

func findApproval(runID RunID, name tool.Name, argsHash ToolArgsHash, approvals []Approval) (Approval, bool) {
	for _, approval := range approvals {
		if approval.RunID == runID && approval.ToolName == name && approval.ToolArgsHash == argsHash {
			return approval, true
		}
	}
	return Approval{}, false
}
