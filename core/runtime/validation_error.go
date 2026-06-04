package runtime

import "fmt"

// ValidationCode gives policy failures stable machine-readable reasons.
type ValidationCode string

const (
	ValidationCodeInvalidState     ValidationCode = "invalid_state"
	ValidationCodeInvalidDecision  ValidationCode = "invalid_decision"
	ValidationCodeUnknownTool      ValidationCode = "unknown_tool"
	ValidationCodeApprovalRequired ValidationCode = "approval_required"
	ValidationCodeApprovalRejected ValidationCode = "approval_rejected"
	ValidationCodePolicyViolation  ValidationCode = "policy_violation"
)

// ValidationError lets callers distinguish deterministic policy failures from internal errors.
type ValidationError struct {
	Code    ValidationCode
	Message string
}

// Error exposes validation failure as standard Go error without losing the machine-readable code.
func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Failure converts validation errors into runtime Failure without semantic rewriting.
func (e ValidationError) Failure() Failure {
	switch e.Code {
	case ValidationCodeInvalidState:
		return Failure{
			Code:    FailureCodeInvalidState,
			Message: e.Message,
		}

	case ValidationCodeInvalidDecision:
		return Failure{
			Code:    FailureCodeInvalidDecision,
			Message: e.Message,
		}

	case ValidationCodeUnknownTool:
		return Failure{
			Code:    FailureCodeToolUnavailable,
			Message: e.Message,
		}

	case ValidationCodeApprovalRequired:
		return Failure{
			Code:    FailureCodeApprovalRequired,
			Message: e.Message,
		}

	case ValidationCodeApprovalRejected:
		return Failure{
			Code:    FailureCodePolicyViolation,
			Message: e.Message,
		}

	default:
		return Failure{
			Code:    FailureCodePolicyViolation,
			Message: e.Message,
		}
	}
}
