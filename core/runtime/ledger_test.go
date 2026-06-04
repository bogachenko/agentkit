package runtime

import (
	"testing"
	"time"

	"github.com/bogachenko/agentkit/core/tool"
)

func ledgerTestTime() time.Time {
	return time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
}

func validCompletedLedgerStep() Step {
	startedAt := ledgerTestTime()

	return Step{
		ID:          StepID("step-1"),
		Source:      StepSourceRuntime,
		Status:      StepStatusCompleted,
		Description: "Validated route decision.",
		StartedAt:   startedAt,
		FinishedAt:  startedAt.Add(time.Second),
	}
}

func validFailedLedgerStep() Step {
	startedAt := ledgerTestTime()

	return Step{
		ID:          StepID("step-2"),
		Source:      StepSourceTool,
		Status:      StepStatusFailed,
		Description: "Executed tool.",
		Failure: &Failure{
			Code:    FailureCodeToolFailed,
			Message: "tool execution failed",
		},
		StartedAt:  startedAt,
		FinishedAt: startedAt.Add(time.Second),
	}
}

func TestLedgerEntryValidateAcceptsStepEntry(t *testing.T) {
	entry := NewStepEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		validCompletedLedgerStep(),
		ledgerTestTime(),
	)

	if err := entry.Validate(); err != nil {
		t.Fatalf("expected valid step entry, got error: %v", err)
	}
}

func TestLedgerEntryValidateRejectsMissingCreatedAt(t *testing.T) {
	entry := NewStepEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		validCompletedLedgerStep(),
		time.Time{},
	)

	if err := entry.Validate(); err == nil {
		t.Fatal("expected error for missing created_at")
	}
}

func TestLedgerEntryValidateRejectsMixedPayloads(t *testing.T) {
	entry := NewStepEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		validCompletedLedgerStep(),
		ledgerTestTime(),
	)
	entry.Message = "unexpected message"

	if err := entry.Validate(); err == nil {
		t.Fatal("expected error for mixed payloads")
	}
}

func TestLedgerEntryValidateAcceptsRouteDecisionEntry(t *testing.T) {
	decision := RouteDecision{
		Kind:     RouteKindCallTool,
		ToolName: tool.Name("search_products"),
		Reason:   "Model requested an explicit tool call.",
	}

	entry := NewRouteDecisionEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		decision,
		ledgerTestTime(),
	)

	if err := entry.Validate(); err != nil {
		t.Fatalf("expected valid route decision entry, got error: %v", err)
	}
}

func TestLedgerEntryValidateAcceptsToolCallEntry(t *testing.T) {
	call := tool.NewCall(tool.Name("search_products"), map[string]any{
		"query": "box",
	})

	entry := NewToolCallEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		call,
		ledgerTestTime(),
	)

	if err := entry.Validate(); err != nil {
		t.Fatalf("expected valid tool call entry, got error: %v", err)
	}
}

func TestLedgerEntryValidateAcceptsToolResultEntry(t *testing.T) {
	result := tool.NewResult(tool.Name("search_products"), map[string]any{
		"count": 3,
	})

	entry := NewToolResultEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		result,
		ledgerTestTime(),
	)

	if err := entry.Validate(); err != nil {
		t.Fatalf("expected valid tool result entry, got error: %v", err)
	}
}

func TestLedgerEntryValidateAcceptsNoteEntry(t *testing.T) {
	entry := NewNoteEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		"Runtime initialized.",
		ledgerTestTime(),
	)

	if err := entry.Validate(); err != nil {
		t.Fatalf("expected valid note entry, got error: %v", err)
	}
}

func TestLedgerEntryValidateRejectsEmptyNoteMessage(t *testing.T) {
	entry := NewNoteEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		"   ",
		ledgerTestTime(),
	)

	if err := entry.Validate(); err == nil {
		t.Fatal("expected error for empty note message")
	}
}

func TestNewLedgerRejectsEmptyRunID(t *testing.T) {
	if _, err := NewLedger(RunID("")); err == nil {
		t.Fatal("expected error for empty run id")
	}
}

func TestLedgerAppendStoresEntry(t *testing.T) {
	ledger, err := NewLedger(RunID("run-1"))
	if err != nil {
		t.Fatalf("expected ledger creation to succeed, got error: %v", err)
	}

	entry := NewStepEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		validCompletedLedgerStep(),
		ledgerTestTime(),
	)

	if err := ledger.Append(entry); err != nil {
		t.Fatalf("expected append to succeed, got error: %v", err)
	}

	if ledger.Len() != 1 {
		t.Fatalf("expected ledger len 1, got %d", ledger.Len())
	}

	got, exists := ledger.Find(LedgerEntryID("entry-1"))
	if !exists {
		t.Fatal("expected entry to exist")
	}

	if got.ID != entry.ID {
		t.Fatalf("expected entry id %q, got %q", entry.ID, got.ID)
	}
}

func TestLedgerAppendRejectsMismatchedRunID(t *testing.T) {
	ledger, err := NewLedger(RunID("run-1"))
	if err != nil {
		t.Fatalf("expected ledger creation to succeed, got error: %v", err)
	}

	entry := NewStepEntry(
		LedgerEntryID("entry-1"),
		RunID("run-2"),
		validCompletedLedgerStep(),
		ledgerTestTime(),
	)

	if err := ledger.Append(entry); err == nil {
		t.Fatal("expected error for mismatched run id")
	}
}

func TestLedgerAppendRejectsDuplicateEntryID(t *testing.T) {
	ledger, err := NewLedger(RunID("run-1"))
	if err != nil {
		t.Fatalf("expected ledger creation to succeed, got error: %v", err)
	}

	entry := NewStepEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		validCompletedLedgerStep(),
		ledgerTestTime(),
	)

	if err := ledger.Append(entry); err != nil {
		t.Fatalf("expected first append to succeed, got error: %v", err)
	}

	if err := ledger.Append(entry); err == nil {
		t.Fatal("expected duplicate entry error")
	}
}

func TestLedgerEntriesReturnsCopy(t *testing.T) {
	ledger, err := NewLedger(RunID("run-1"))
	if err != nil {
		t.Fatalf("expected ledger creation to succeed, got error: %v", err)
	}

	entry := NewStepEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		validCompletedLedgerStep(),
		ledgerTestTime(),
	)

	if err := ledger.Append(entry); err != nil {
		t.Fatalf("expected append to succeed, got error: %v", err)
	}

	entries := ledger.Entries()
	entries[0].ID = LedgerEntryID("changed")

	got, exists := ledger.Find(LedgerEntryID("entry-1"))
	if !exists {
		t.Fatal("expected original entry to exist")
	}

	if got.ID != LedgerEntryID("entry-1") {
		t.Fatalf("expected original entry id to remain unchanged, got %q", got.ID)
	}
}

func TestLedgerLastReturnsNewestEntry(t *testing.T) {
	ledger, err := NewLedger(RunID("run-1"))
	if err != nil {
		t.Fatalf("expected ledger creation to succeed, got error: %v", err)
	}

	first := NewNoteEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		"First note.",
		ledgerTestTime(),
	)

	second := NewNoteEntry(
		LedgerEntryID("entry-2"),
		RunID("run-1"),
		"Second note.",
		ledgerTestTime().Add(time.Second),
	)

	if err := ledger.Append(first); err != nil {
		t.Fatalf("append first: %v", err)
	}

	if err := ledger.Append(second); err != nil {
		t.Fatalf("append second: %v", err)
	}

	got, exists := ledger.Last()
	if !exists {
		t.Fatal("expected last entry to exist")
	}

	if got.ID != LedgerEntryID("entry-2") {
		t.Fatalf("expected newest entry id entry-2, got %q", got.ID)
	}
}

func TestLedgerSummaryCountsEntriesDeterministically(t *testing.T) {
	ledger, err := NewLedger(RunID("run-1"))
	if err != nil {
		t.Fatalf("expected ledger creation to succeed, got error: %v", err)
	}

	if err := ledger.Append(NewStepEntry(
		LedgerEntryID("entry-1"),
		RunID("run-1"),
		validCompletedLedgerStep(),
		ledgerTestTime(),
	)); err != nil {
		t.Fatalf("append completed step: %v", err)
	}

	if err := ledger.Append(NewStepEntry(
		LedgerEntryID("entry-2"),
		RunID("run-1"),
		validFailedLedgerStep(),
		ledgerTestTime().Add(time.Second),
	)); err != nil {
		t.Fatalf("append failed step: %v", err)
	}

	if err := ledger.Append(NewRouteDecisionEntry(
		LedgerEntryID("entry-3"),
		RunID("run-1"),
		RouteDecision{
			Kind:     RouteKindCallTool,
			ToolName: tool.Name("search_products"),
			Reason:   "Model requested an explicit tool call.",
		},
		ledgerTestTime().Add(2*time.Second),
	)); err != nil {
		t.Fatalf("append route decision: %v", err)
	}

	if err := ledger.Append(NewToolCallEntry(
		LedgerEntryID("entry-4"),
		RunID("run-1"),
		tool.NewCall(tool.Name("search_products"), map[string]any{"query": "box"}),
		ledgerTestTime().Add(3*time.Second),
	)); err != nil {
		t.Fatalf("append tool call: %v", err)
	}

	if err := ledger.Append(NewToolResultEntry(
		LedgerEntryID("entry-5"),
		RunID("run-1"),
		tool.NewResult(tool.Name("search_products"), map[string]any{"count": 3}),
		ledgerTestTime().Add(4*time.Second),
	)); err != nil {
		t.Fatalf("append tool result: %v", err)
	}

	summary := ledger.Summary()

	if summary.RunID != RunID("run-1") {
		t.Fatalf("expected run id run-1, got %q", summary.RunID)
	}

	if summary.TotalEntries != 5 {
		t.Fatalf("expected 5 entries, got %d", summary.TotalEntries)
	}

	if summary.Steps != 2 {
		t.Fatalf("expected 2 steps, got %d", summary.Steps)
	}

	if summary.StepsCompleted != 1 {
		t.Fatalf("expected 1 completed step, got %d", summary.StepsCompleted)
	}

	if summary.StepsFailed != 1 {
		t.Fatalf("expected 1 failed step, got %d", summary.StepsFailed)
	}

	if summary.RouteDecisions != 1 {
		t.Fatalf("expected 1 route decision, got %d", summary.RouteDecisions)
	}

	if summary.ToolCalls != 1 {
		t.Fatalf("expected 1 tool call, got %d", summary.ToolCalls)
	}

	if summary.ToolResults != 1 {
		t.Fatalf("expected 1 tool result, got %d", summary.ToolResults)
	}

	if summary.LastEntryID != LedgerEntryID("entry-5") {
		t.Fatalf("expected last entry entry-5, got %q", summary.LastEntryID)
	}

	if summary.LastFailure == nil {
		t.Fatal("expected last failure to be recorded from failed step")
	}
}
