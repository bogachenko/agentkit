package runtime

import (
	"fmt"

	"github.com/bogachenko/agentkit/core/tool"
)

// Validator enforces runtime policy deterministically before orchestration executes a decision.
type Validator struct {
	Policy Policy
}

// ValidationInput makes every policy-relevant fact explicit and testable.
type ValidationInput struct {
	State     State
	Decision  RouteDecision
	Approvals []Approval
}

// ValidateDecision rejects invalid or unauthorized route decisions without fallback behavior.
func (v Validator) ValidateDecision(input ValidationInput) error {
	if err := input.State.Validate(); err != nil {
		return ValidationError{
			Code:    ValidationCodeInvalidState,
			Message: err.Error(),
		}
	}

	if err := input.Decision.Validate(); err != nil {
		return ValidationError{
			Code:    ValidationCodeInvalidDecision,
			Message: err.Error(),
		}
	}

	for index, approval := range input.Approvals {
		if err := approval.Validate(); err != nil {
			return ValidationError{
				Code:    ValidationCodeInvalidState,
				Message: fmt.Sprintf("approval %d: %v", index, err),
			}
		}

		if approval.RunID != input.State.RunID {
			return ValidationError{
				Code:    ValidationCodeInvalidState,
				Message: fmt.Sprintf("approval %q run id does not match state run id", string(approval.ID)),
			}
		}
	}

	switch input.Decision.Kind {
	case RouteKindCallTool:
		return v.validateToolCall(input.State.RunID, input.Decision.ToolName, input.Approvals)

	case RouteKindRequireApproval:
		return v.validateApprovalRequest(input.Decision.ToolName)

	case RouteKindRespond, RouteKindComplete, RouteKindBlocked:
		return nil

	default:
		return ValidationError{
			Code:    ValidationCodeInvalidDecision,
			Message: fmt.Sprintf("unsupported route kind %q", string(input.Decision.Kind)),
		}
	}
}

// BlockedDecision converts validation failure into an explicit blocked route for callers.
func (v Validator) BlockedDecision(err error) RouteDecision {
	if err == nil {
		return RouteDecision{}
	}

	validationErr, ok := err.(ValidationError)
	if !ok {
		validationErr = ValidationError{
			Code:    ValidationCodePolicyViolation,
			Message: err.Error(),
		}
	}

	failure := validationErr.Failure()

	return RouteDecision{
		Kind:    RouteKindBlocked,
		Reason:  "Runtime validation blocked the route decision.",
		Failure: &failure,
	}
}

// validateToolCall ensures tool execution is based on registry metadata and explicit approvals only.
func (v Validator) validateToolCall(runID RunID, name tool.Name, approvals []Approval) error {
	contract, exists, err := v.Policy.ToolContract(name)
	if err != nil {
		return err
	}

	if !exists {
		return ValidationError{
			Code:    ValidationCodeUnknownTool,
			Message: fmt.Sprintf("tool %q is not registered", string(name)),
		}
	}

	if !contract.RequiresApproval {
		return nil
	}

	approval, exists := findApproval(runID, name, approvals)
	if !exists {
		return ValidationError{
			Code:    ValidationCodeApprovalRequired,
			Message: fmt.Sprintf("tool %q requires approval", string(name)),
		}
	}

	if approval.Status == ApprovalStatusRejected {
		return ValidationError{
			Code:    ValidationCodeApprovalRejected,
			Message: fmt.Sprintf("approval %q rejected tool %q", string(approval.ID), string(name)),
		}
	}

	if !approval.IsApproved() {
		return ValidationError{
			Code:    ValidationCodeApprovalRequired,
			Message: fmt.Sprintf("tool %q approval is not approved", string(name)),
		}
	}

	return nil
}

// validateApprovalRequest ensures approval prompts reference real approval-required tools.
func (v Validator) validateApprovalRequest(name tool.Name) error {
	contract, exists, err := v.Policy.ToolContract(name)
	if err != nil {
		return err
	}

	if !exists {
		return ValidationError{
			Code:    ValidationCodeUnknownTool,
			Message: fmt.Sprintf("tool %q is not registered", string(name)),
		}
	}

	if !contract.RequiresApproval {
		return ValidationError{
			Code:    ValidationCodePolicyViolation,
			Message: fmt.Sprintf("tool %q does not require approval", string(name)),
		}
	}

	return nil
}

// findApproval scopes approval lookup by run and tool without broad implicit permissions.
func findApproval(runID RunID, name tool.Name, approvals []Approval) (Approval, bool) {
	for _, approval := range approvals {
		if approval.RunID == runID && approval.ToolName == name {
			return approval, true
		}
	}

	return Approval{}, false
}
