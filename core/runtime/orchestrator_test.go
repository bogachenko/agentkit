package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/bogachenko/agentkit/core/port"
	"github.com/bogachenko/agentkit/core/session"
	"github.com/bogachenko/agentkit/core/tool"
)

type orchestratorClock struct {
	now time.Time
}

func (c orchestratorClock) Now() time.Time {
	return c.now
}

type orchestratorIDGenerator struct {
	id string
}

func (g orchestratorIDGenerator) NewID() string {
	return g.id
}

type publishedRuntimeEvent struct {
	event port.Event
}

type orchestratorPublisher struct {
	events []publishedRuntimeEvent
}

func (p *orchestratorPublisher) Publish(ctx context.Context, event port.Event) error {
	p.events = append(p.events, publishedRuntimeEvent{event: event})
	return nil
}

func newTestOrchestrator(t *testing.T, contracts []tool.Contract) (Orchestrator, *Ledger, *orchestratorPublisher) {
	t.Helper()

	ledger, err := NewLedger(RunID("run-1"))
	if err != nil {
		t.Fatalf("expected ledger creation to succeed, got error: %v", err)
	}

	publisher := &orchestratorPublisher{}

	return Orchestrator{
		Validator: Validator{
			Policy: Policy{
				ToolContracts: contracts,
			},
		},
		Ledger:      ledger,
		Publisher:   publisher,
		Clock:       orchestratorClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		IDGenerator: orchestratorIDGenerator{id: "entry-1"},
	}, ledger, publisher
}

func testOrchestrationCommand(decision RouteDecision) Command {
	return Command{
		SessionID: session.ID("session-1"),
		State:     runtimeTestState(),
		Decision:  decision,
	}
}

func TestOrchestratorHandleDecisionAcceptsValidReadOnlyToolDecision(t *testing.T) {
	orchestrator, ledger, publisher := newTestOrchestrator(t, []tool.Contract{
		runtimeTestToolContract(tool.Name("read_products"), true, false),
	})

	result, err := orchestrator.HandleDecision(context.Background(), testOrchestrationCommand(RouteDecision{
		Kind:     RouteKindCallTool,
		ToolName: tool.Name("read_products"),
		Reason:   "Model requested an explicit read-only tool call.",
	}))
	if err != nil {
		t.Fatalf("expected orchestration to succeed, got error: %v", err)
	}

	if !result.Accepted {
		t.Fatal("expected decision to be accepted")
	}

	if result.Blocked {
		t.Fatal("expected decision not to be blocked")
	}

	if ledger.Len() != 1 {
		t.Fatalf("expected ledger len 1, got %d", ledger.Len())
	}

	entry, exists := ledger.Last()
	if !exists {
		t.Fatal("expected ledger entry")
	}

	if entry.Kind != LedgerEntryKindRouteDecision {
		t.Fatalf("expected route decision entry, got %q", entry.Kind)
	}

	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(publisher.events))
	}

	if publisher.events[0].event.Type != port.EventTypeStep {
		t.Fatalf("expected step event, got %q", publisher.events[0].event.Type)
	}
}

func TestOrchestratorHandleDecisionBlocksUnknownToolDecision(t *testing.T) {
	orchestrator, ledger, publisher := newTestOrchestrator(t, []tool.Contract{
		runtimeTestToolContract(tool.Name("read_products"), true, false),
	})

	result, err := orchestrator.HandleDecision(context.Background(), testOrchestrationCommand(RouteDecision{
		Kind:     RouteKindCallTool,
		ToolName: tool.Name("unknown_tool"),
		Reason:   "Model requested an explicit tool call.",
	}))
	if err != nil {
		t.Fatalf("expected orchestration to return blocked result without internal error, got error: %v", err)
	}

	if result.Accepted {
		t.Fatal("expected decision not to be accepted")
	}

	if !result.Blocked {
		t.Fatal("expected decision to be blocked")
	}

	if result.Decision.Kind != RouteKindBlocked {
		t.Fatalf("expected blocked decision, got %q", result.Decision.Kind)
	}

	if result.Decision.Failure == nil {
		t.Fatal("expected blocked decision failure")
	}

	if result.Decision.Failure.Code != FailureCodeToolUnavailable {
		t.Fatalf("expected tool_unavailable failure, got %q", result.Decision.Failure.Code)
	}

	if ledger.Len() != 1 {
		t.Fatalf("expected ledger len 1, got %d", ledger.Len())
	}

	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(publisher.events))
	}

	if publisher.events[0].event.Type != port.EventTypeBlocked {
		t.Fatalf("expected blocked event, got %q", publisher.events[0].event.Type)
	}
}

func TestOrchestratorHandleDecisionRequiresPublisher(t *testing.T) {
	orchestrator, _, _ := newTestOrchestrator(t, []tool.Contract{
		runtimeTestToolContract(tool.Name("read_products"), true, false),
	})
	orchestrator.Publisher = nil

	_, err := orchestrator.HandleDecision(context.Background(), testOrchestrationCommand(RouteDecision{
		Kind:     RouteKindCallTool,
		ToolName: tool.Name("read_products"),
		Reason:   "Model requested an explicit read-only tool call.",
	}))
	if err == nil {
		t.Fatal("expected error for missing publisher")
	}
}

func TestControllerHandleDecisionDelegatesToOrchestrator(t *testing.T) {
	orchestrator, _, _ := newTestOrchestrator(t, []tool.Contract{
		runtimeTestToolContract(tool.Name("read_products"), true, false),
	})

	controller := Controller{
		Orchestrator: orchestrator,
	}

	result, err := controller.HandleDecision(context.Background(), testOrchestrationCommand(RouteDecision{
		Kind:     RouteKindCallTool,
		ToolName: tool.Name("read_products"),
		Reason:   "Model requested an explicit read-only tool call.",
	}))
	if err != nil {
		t.Fatalf("expected controller to succeed, got error: %v", err)
	}

	if !result.Accepted {
		t.Fatal("expected accepted decision")
	}
}
