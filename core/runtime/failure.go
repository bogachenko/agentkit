package runtime

import (
	"fmt"
	"strings"
)

// FailureCode gives blocked and failed states stable machine-readable reasons.
type FailureCode string

const (
	FailureCodeInvalidState     FailureCode = "invalid_state"
	FailureCodeInvalidDecision  FailureCode = "invalid_decision"
	FailureCodePolicyViolation  FailureCode = "policy_violation"
	FailureCodeApprovalRequired FailureCode = "approval_required"
	FailureCodeToolUnavailable  FailureCode = "tool_unavailable"
	FailureCodeToolFailed       FailureCode = "tool_failed"
	FailureCodeModelFailed      FailureCode = "model_failed"
	FailureCodeInternalError    FailureCode = "internal_error"
)

// Validation prevents arbitrary error strings from becoming runtime control flow.
func (c FailureCode) Validate() error {
	switch c {
	case FailureCodeInvalidState,
		FailureCodeInvalidDecision,
		FailureCodePolicyViolation,
		FailureCodeApprovalRequired,
		FailureCodeToolUnavailable,
		FailureCodeToolFailed,
		FailureCodeModelFailed,
		FailureCodeInternalError:
		return nil
	default:
		return fmt.Errorf("unknown failure code %q", string(c))
	}
}

// Failure records explicit runtime stop reasons without hiding them in logs or free-form errors.
type Failure struct {
	Code    FailureCode
	Message string
}

// Validation ensures blocked and failed states are explainable to callers.
func (f Failure) Validate() error {
	if err := f.Code.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(f.Message) == "" {
		return fmt.Errorf("failure message is required for code %q", string(f.Code))
	}

	return nil
}
