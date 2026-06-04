package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bogachenko/agentkit/core/llm"
	"github.com/bogachenko/agentkit/core/port"
	"github.com/bogachenko/agentkit/core/session"
	"github.com/bogachenko/agentkit/core/tool"
)

type fakeStepProvider struct {
	batches [][]Step
	index   int
}

func (p *fakeStepProvider) NextSteps(ctx context.Context, state State) ([]Step, error) {
	if p.index >= len(p.batches) {
		return nil, ErrStepSourceDone
	}

	batch := p.batches[p.index]
	p.index++

	return batch, nil
}

type collectingPublisher struct {
	events []port.Event
}

func (p *collectingPublisher) Publish(ctx context.Context, event port.Event) error {
	p.events = append(p.events, event)
	return nil
}

type testClock struct {
	now time.Time
}

func (c testClock) Now() time.Time {
	return c.now
}

type testIDGenerator struct {
	next int
}

func (g *testIDGenerator) NewID() string {
	g.next++
	return fmt.Sprintf("id-%d", g.next)
}

func TestStepOrchestratorCompletesFromADKLikeSteps(t *testing.T) {
	publisher := &collectingPublisher{}

	orchestrator := StepOrchestrator{
		StepProvider: &fakeStepProvider{
			batches: [][]Step{
				{
					{
						Kind:       StepKindToolCall,
						Source:     StepSourceModel,
						Status:     StepStatusCompleted,
						ToolCallID: "call-1",
						ToolName:   tool.Name("read_catalog"),
						ToolArgs: map[string]any{
							"query": "boxes",
						},
					},
				},
				{
					{
						Kind:       StepKindToolResult,
						Source:     StepSourceTool,
						Status:     StepStatusCompleted,
						ToolCallID: "call-1",
						ToolName:   tool.Name("read_catalog"),
						ToolResult: ToolExecutionResult{
							OK:          true,
							HasEvidence: true,
							Raw: map[string]any{
								"items": []any{"box-1", "box-2"},
							},
						},
					},
				},
				{
					{
						Kind:   StepKindAssistantText,
						Source: StepSourceModel,
						Status: StepStatusCompleted,
						Text:   "Found 2 boxes.",
						Final:  true,
					},
				},
			},
		},
		Publisher:   publisher,
		Clock:       testClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		IDGenerator: &testIDGenerator{},
	}

	result, err := orchestrator.Run(context.Background(), StepRunCommand{
		RunID:     RunID("run-1"),
		SessionID: session.ID("session-1"),
		MaxSteps:  10,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if result.Status != RunStatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}

	if result.StepsCompleted != 3 {
		t.Fatalf("expected 3 completed steps, got %d", result.StepsCompleted)
	}

	if result.FinalMessage == nil {
		t.Fatal("expected final message")
	}

	if got := textFromMessage(*result.FinalMessage); got != "Found 2 boxes." {
		t.Fatalf("unexpected final text: %q", got)
	}

	if result.LedgerSummary.TotalEntries != 3 {
		t.Fatalf("expected 3 ledger entries, got %d", result.LedgerSummary.TotalEntries)
	}

	if result.LedgerSummary.Steps != 3 {
		t.Fatalf("expected 3 step ledger entries, got %d", result.LedgerSummary.Steps)
	}

	if result.LedgerSummary.ToolCalls != 1 {
		t.Fatalf("expected 1 tool call, got %d", result.LedgerSummary.ToolCalls)
	}

	if result.LedgerSummary.ToolResults != 1 {
		t.Fatalf("expected 1 tool result, got %d", result.LedgerSummary.ToolResults)
	}

	if result.LedgerSummary.FinalResponses != 1 {
		t.Fatalf("expected 1 final response, got %d", result.LedgerSummary.FinalResponses)
	}

	if len(publisher.events) != 5 {
		t.Fatalf("expected 5 published events, got %d", len(publisher.events))
	}

	if publisher.events[0].Type != port.EventTypeStarted {
		t.Fatalf("expected started event, got %s", publisher.events[0].Type)
	}

	if publisher.events[len(publisher.events)-1].Type != port.EventTypeCompleted {
		t.Fatalf("expected completed event, got %s", publisher.events[len(publisher.events)-1].Type)
	}
}

func TestStepOrchestratorFailsOnToolResultError(t *testing.T) {
	publisher := &collectingPublisher{}

	orchestrator := StepOrchestrator{
		StepProvider: &fakeStepProvider{
			batches: [][]Step{
				{
					{
						Kind:       StepKindToolResult,
						Source:     StepSourceTool,
						Status:     StepStatusCompleted,
						ToolCallID: "call-1",
						ToolName:   tool.Name("read_catalog"),
						ToolResult: ToolExecutionResult{
							OK:           false,
							ErrorKind:    ToolErrorValidation,
							ErrorMessage: "request validation failed",
							Raw: map[string]any{
								"error": "request validation failed",
							},
						},
					},
				},
			},
		},
		Publisher:   publisher,
		Clock:       testClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		IDGenerator: &testIDGenerator{},
	}

	result, err := orchestrator.Run(context.Background(), StepRunCommand{
		RunID:     RunID("run-1"),
		SessionID: session.ID("session-1"),
		MaxSteps:  10,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if result.Status != RunStatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}

	if result.Failure == nil {
		t.Fatal("expected failure")
	}

	if result.Failure.Code != FailureCodeToolFailed {
		t.Fatalf("expected tool_failed, got %s", result.Failure.Code)
	}

	if result.StepsCompleted != 1 {
		t.Fatalf("expected 1 completed step, got %d", result.StepsCompleted)
	}

	if result.LedgerSummary.ToolResults != 1 {
		t.Fatalf("expected 1 tool result, got %d", result.LedgerSummary.ToolResults)
	}

	if result.LedgerSummary.ToolErrors != 1 {
		t.Fatalf("expected 1 tool error, got %d", result.LedgerSummary.ToolErrors)
	}

	if len(publisher.events) != 3 {
		t.Fatalf("expected 3 published events, got %d", len(publisher.events))
	}

	if publisher.events[len(publisher.events)-1].Type != port.EventTypeFailed {
		t.Fatalf("expected failed event, got %s", publisher.events[len(publisher.events)-1].Type)
	}
}

func TestStepOrchestratorFailsWithoutFinalResponse(t *testing.T) {
	orchestrator := StepOrchestrator{
		StepProvider: &fakeStepProvider{
			batches: [][]Step{
				{
					{
						Kind:   StepKindAssistantText,
						Source: StepSourceModel,
						Status: StepStatusCompleted,
						Text:   "intermediate text",
						Final:  false,
					},
				},
			},
		},
		Publisher:   &collectingPublisher{},
		Clock:       testClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		IDGenerator: &testIDGenerator{},
	}

	result, err := orchestrator.Run(context.Background(), StepRunCommand{
		RunID:     RunID("run-1"),
		SessionID: session.ID("session-1"),
		MaxSteps:  10,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if result.Status != RunStatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}

	if result.Failure == nil {
		t.Fatal("expected failure")
	}

	if result.Failure.Code != FailureCodeInvalidState {
		t.Fatalf("expected invalid_state, got %s", result.Failure.Code)
	}
}

func textFromMessage(message llm.Message) string {
	for _, part := range message.Parts {
		if part.Type == llm.PartText {
			return part.Text
		}
	}

	return ""
}
