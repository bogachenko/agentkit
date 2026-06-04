package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/bogachenko/agentkit/core/tool"
)

// ApprovalID gives user approvals stable identity for audit and policy checks.
type ApprovalID string

// Validation prevents anonymous approvals from authorizing runtime actions.
func (id ApprovalID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("approval id is required")
	}

	return nil
}

// ApprovalStatus makes authorization state explicit instead of relying on booleans.
type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
)

// Validation blocks unknown approval states from entering policy decisions.
func (s ApprovalStatus) Validate() error {
	switch s {
	case ApprovalStatusPending, ApprovalStatusApproved, ApprovalStatusRejected:
		return nil
	default:
		return fmt.Errorf("unknown approval status %q", string(s))
	}
}

// Approval binds user authorization to one explicit tool instead of broad implicit permission.
type Approval struct {
	ID        ApprovalID
	RunID     RunID
	ToolName  tool.Name
	Status    ApprovalStatus
	Reason    string
	CreatedAt time.Time
}

// Validation ensures approvals are explicit, auditable, and scoped to one run/tool.
func (a Approval) Validate() error {
	if err := a.ID.Validate(); err != nil {
		return err
	}

	if err := a.RunID.Validate(); err != nil {
		return err
	}

	if err := a.ToolName.Validate(); err != nil {
		return err
	}

	if err := a.Status.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(a.Reason) == "" {
		return fmt.Errorf("approval reason is required")
	}

	if a.CreatedAt.IsZero() {
		return fmt.Errorf("approval created_at is required")
	}

	return nil
}

// IsApproved keeps policy checks deterministic and scoped to validated approval state.
func (a Approval) IsApproved() bool {
	return a.Status == ApprovalStatusApproved
}
