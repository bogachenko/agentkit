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

type ApprovalID string

func (id ApprovalID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("approval id is required")
	}
	return nil
}

type ToolArgsHash string

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

func (h ToolArgsHash) Validate() error {
	if strings.TrimSpace(string(h)) == "" {
		return fmt.Errorf("tool args hash is required")
	}
	return nil
}

type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
)

func (s ApprovalStatus) Validate() error {
	switch s {
	case ApprovalStatusPending, ApprovalStatusApproved, ApprovalStatusRejected:
		return nil
	default:
		return fmt.Errorf("unknown approval status %q", string(s))
	}
}

type Approval struct {
	ID           ApprovalID
	RunID        RunID
	ToolName     tool.Name
	ToolArgsHash ToolArgsHash
	Status       ApprovalStatus
	Payload      map[string]any
	Reason       string
	CreatedAt    time.Time
}

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

func (a Approval) IsApproved() bool {
	return a.Status == ApprovalStatusApproved
}
