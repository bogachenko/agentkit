package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/bogachenko/agentkit/core/tool"
)

// LedgerEntryID gives every ledger record a stable identity for audit and deduplication.
type LedgerEntryID string

// Validation prevents anonymous ledger entries from entering append-only runtime history.
func (id LedgerEntryID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("ledger entry id is required")
	}

	return nil
}

// LedgerEntryKind makes recorded runtime facts explicit without inferring meaning from payload fields.
type LedgerEntryKind string

const (
	LedgerEntryKindStep          LedgerEntryKind = "step"
	LedgerEntryKindRouteDecision LedgerEntryKind = "route_decision"
	LedgerEntryKindToolCall      LedgerEntryKind = "tool_call"
	LedgerEntryKindToolResult    LedgerEntryKind = "tool_result"
	LedgerEntryKindNote          LedgerEntryKind = "note"
)

// Validation blocks unknown ledger entry kinds before they affect runtime summaries.
func (k LedgerEntryKind) Validate() error {
	switch k {
	case LedgerEntryKindStep,
		LedgerEntryKindRouteDecision,
		LedgerEntryKindToolCall,
		LedgerEntryKindToolResult,
		LedgerEntryKindNote:
		return nil
	default:
		return fmt.Errorf("unknown ledger entry kind %q", string(k))
	}
}

// LedgerEntry records one immutable runtime fact without mixing execution, routing, and reasoning logic.
type LedgerEntry struct {
	ID         LedgerEntryID
	RunID      RunID
	Kind       LedgerEntryKind
	Step       *Step
	Decision   *RouteDecision
	ToolCall   *tool.Call
	ToolResult *tool.Result
	Message    string
	CreatedAt  time.Time
}

// NewStepEntry records an already known runtime step as an audit fact.
func NewStepEntry(id LedgerEntryID, runID RunID, step Step, createdAt time.Time) LedgerEntry {
	return LedgerEntry{
		ID:        id,
		RunID:     runID,
		Kind:      LedgerEntryKindStep,
		Step:      &step,
		CreatedAt: createdAt,
	}
}

// NewRouteDecisionEntry records an already selected route decision for deterministic validation and audit.
func NewRouteDecisionEntry(id LedgerEntryID, runID RunID, decision RouteDecision, createdAt time.Time) LedgerEntry {
	return LedgerEntry{
		ID:        id,
		RunID:     runID,
		Kind:      LedgerEntryKindRouteDecision,
		Decision:  &decision,
		CreatedAt: createdAt,
	}
}

// NewToolCallEntry records an explicit tool invocation request without executing the tool.
func NewToolCallEntry(id LedgerEntryID, runID RunID, call tool.Call, createdAt time.Time) LedgerEntry {
	return LedgerEntry{
		ID:        id,
		RunID:     runID,
		Kind:      LedgerEntryKindToolCall,
		ToolCall:  &call,
		CreatedAt: createdAt,
	}
}

// NewToolResultEntry records structured tool output without deciding what to do next.
func NewToolResultEntry(id LedgerEntryID, runID RunID, result tool.Result, createdAt time.Time) LedgerEntry {
	return LedgerEntry{
		ID:         id,
		RunID:      runID,
		Kind:       LedgerEntryKindToolResult,
		ToolResult: &result,
		CreatedAt:  createdAt,
	}
}

// NewNoteEntry records deterministic runtime notes that are not LLM reasoning or semantic summaries.
func NewNoteEntry(id LedgerEntryID, runID RunID, message string, createdAt time.Time) LedgerEntry {
	return LedgerEntry{
		ID:        id,
		RunID:     runID,
		Kind:      LedgerEntryKindNote,
		Message:   message,
		CreatedAt: createdAt,
	}
}

// Validation keeps every ledger entry structurally consistent with its declared kind.
func (e LedgerEntry) Validate() error {
	if err := e.ID.Validate(); err != nil {
		return err
	}

	if err := e.RunID.Validate(); err != nil {
		return err
	}

	if err := e.Kind.Validate(); err != nil {
		return err
	}

	if e.CreatedAt.IsZero() {
		return fmt.Errorf("ledger entry %q created_at is required", string(e.ID))
	}

	switch e.Kind {
	case LedgerEntryKindStep:
		if e.Step == nil {
			return fmt.Errorf("ledger entry %q requires step payload", string(e.ID))
		}

		if err := e.Step.Validate(); err != nil {
			return fmt.Errorf("ledger entry %q step: %w", string(e.ID), err)
		}

		return e.validateOnlyPayload("step")

	case LedgerEntryKindRouteDecision:
		if e.Decision == nil {
			return fmt.Errorf("ledger entry %q requires route decision payload", string(e.ID))
		}

		if err := e.Decision.Validate(); err != nil {
			return fmt.Errorf("ledger entry %q route decision: %w", string(e.ID), err)
		}

		return e.validateOnlyPayload("decision")

	case LedgerEntryKindToolCall:
		if e.ToolCall == nil {
			return fmt.Errorf("ledger entry %q requires tool call payload", string(e.ID))
		}

		if err := e.ToolCall.Validate(); err != nil {
			return fmt.Errorf("ledger entry %q tool call: %w", string(e.ID), err)
		}

		return e.validateOnlyPayload("tool_call")

	case LedgerEntryKindToolResult:
		if e.ToolResult == nil {
			return fmt.Errorf("ledger entry %q requires tool result payload", string(e.ID))
		}

		if err := e.ToolResult.Validate(); err != nil {
			return fmt.Errorf("ledger entry %q tool result: %w", string(e.ID), err)
		}

		return e.validateOnlyPayload("tool_result")

	case LedgerEntryKindNote:
		if strings.TrimSpace(e.Message) == "" {
			return fmt.Errorf("ledger entry %q requires note message", string(e.ID))
		}

		return e.validateOnlyPayload("message")

	default:
		return fmt.Errorf("unsupported ledger entry kind %q", string(e.Kind))
	}
}

// Payload validation prevents one ledger entry from silently representing multiple runtime facts.
func (e LedgerEntry) validateOnlyPayload(expected string) error {
	unexpected := map[string]bool{
		"step":        e.Step != nil,
		"decision":    e.Decision != nil,
		"tool_call":   e.ToolCall != nil,
		"tool_result": e.ToolResult != nil,
		"message":     strings.TrimSpace(e.Message) != "",
	}

	for name, present := range unexpected {
		if name == expected {
			continue
		}

		if present {
			return fmt.Errorf("ledger entry %q has unexpected %s payload for %s entry", string(e.ID), name, string(e.Kind))
		}
	}

	return nil
}
