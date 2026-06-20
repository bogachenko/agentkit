package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// ToolArgsHash binds approval to the exact canonical JSON tool arguments.
type ToolArgsHash string

// NewToolArgsHash returns a stable SHA-256 fingerprint for JSON-compatible tool arguments.
func NewToolArgsHash(args map[string]any) (ToolArgsHash, error) {
	if args == nil {
		args = map[string]any{}
	}

	canonicalJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("tool args hash: %w", err)
	}

	sum := sha256.Sum256(canonicalJSON)
	return ToolArgsHash(hex.EncodeToString(sum[:])), nil
}

// Validation prevents blank argument fingerprints from authorizing tool execution.
func (h ToolArgsHash) Validate() error {
	if strings.TrimSpace(string(h)) == "" {
		return fmt.Errorf("tool args hash is required")
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

// Approval binds user authorization to one explicit tool and exact tool arguments.
type Approval struct {
	ID           ApprovalID
	RunID        RunID
	ToolName     tool.Name
	ToolArgsHash ToolArgsHash
	Status       ApprovalStatus
	Reason       string
	CreatedAt    time.Time
}

// Validation ensures approvals are explicit, auditable, and scoped to one run/tool/args fingerprint.
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

	if err := a.ToolArgsHash.Validate(); err != nil {
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
