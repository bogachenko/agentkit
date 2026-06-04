package main

import (
	"context"
	"fmt"
	"time"

	"github.com/bogachenko/agentkit/core/llm"
	"github.com/bogachenko/agentkit/core/port"
	agentruntime "github.com/bogachenko/agentkit/core/runtime"
	"github.com/bogachenko/agentkit/core/session"
	"github.com/bogachenko/agentkit/core/tool"
)

// fakeModel proves the harness can run without ADK or external LLM providers.
type fakeModel struct {
	calls int
}

// Generate returns explicit structured route decisions instead of relying on keyword intent detection.
func (m *fakeModel) Generate(ctx context.Context, request llm.Request) (llm.Message, error) {
	m.calls++

	if m.calls == 1 {
		return llm.NewMessage(
			llm.RoleAssistant,
			llm.FunctionCallPart(agentruntime.RouteDecisionFunctionName, map[string]any{
				"kind":      string(agentruntime.RouteKindCallTool),
				"tool_name": "read_catalog",
				"tool_args": map[string]any{
					"query": "boxes",
				},
				"reason": "Catalog data is required before producing the final answer.",
			}),
		), nil
	}

	return llm.NewMessage(
		llm.RoleAssistant,
		llm.TextPart("Found 2 catalog items for boxes: archive box and shipping box."),
	), nil
}

// fakeToolExecutor proves tools execute only after explicit validated tool calls.
type fakeToolExecutor struct{}

// ExecuteTool returns deterministic structured output for the requested tool.
func (fakeToolExecutor) ExecuteTool(ctx context.Context, call tool.Call) (tool.Result, error) {
	if call.Name != tool.Name("read_catalog") {
		return tool.Result{}, fmt.Errorf("unsupported tool %q", string(call.Name))
	}

	return tool.NewResult(call.Name, map[string]any{
		"items": []any{
			map[string]any{
				"id":   "box-archive",
				"name": "Archive box",
			},
			map[string]any{
				"id":   "box-shipping",
				"name": "Shipping box",
			},
		},
	}), nil
}

// stdoutPublisher makes runtime events visible without binding the example to HTTP/SSE/WebSocket.
type stdoutPublisher struct{}

// Publish prints transport-neutral runtime events.
func (stdoutPublisher) Publish(ctx context.Context, event port.Event) error {
	fmt.Printf("event=%s run=%s session=%s payload=%v\n", event.Type, event.RunID, event.SessionID, event.Payload)
	return nil
}

// fixedClock makes the example deterministic and test-friendly.
type fixedClock struct {
	now time.Time
}

// Now returns one stable timestamp for repeatable runtime output.
func (c fixedClock) Now() time.Time {
	return c.now
}

// incrementalIDGenerator avoids duplicate ledger entry IDs during one run.
type incrementalIDGenerator struct {
	next int
}

// NewID returns deterministic IDs without hidden randomness.
func (g *incrementalIDGenerator) NewID() string {
	g.next++
	return fmt.Sprintf("entry-%d", g.next)
}

func main() {
	ctx := context.Background()

	harness := agentruntime.Harness{
		Model:        &fakeModel{},
		ToolExecutor: fakeToolExecutor{},
		Validator: agentruntime.Validator{
			Policy: agentruntime.Policy{
				ToolContracts: []tool.Contract{
					{
						Name:        tool.Name("read_catalog"),
						Description: "Reads catalog items by query.",
						InputSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"query": map[string]any{
									"type": "string",
								},
							},
							"required": []any{"query"},
						},
						OutputSchema: map[string]any{
							"type": "object",
						},
						ReadOnly:         true,
						RequiresApproval: false,
					},
				},
			},
		},
		Publisher:   stdoutPublisher{},
		Clock:       fixedClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		IDGenerator: &incrementalIDGenerator{},
	}

	result, err := harness.Run(ctx, agentruntime.RunCommand{
		RunID:     agentruntime.RunID("run-1"),
		SessionID: session.ID("session-1"),
		System:    "Use explicit route_decision tool calls when tool execution is required.",
		UserInput: "Find boxes in the catalog.",
		MaxSteps:  4,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("status=%s steps=%d ledger_entries=%d\n", result.Status, result.StepsCompleted, result.LedgerSummary.TotalEntries)

	if result.FinalMessage != nil {
		for _, part := range result.FinalMessage.Parts {
			if part.Type == llm.PartText {
				fmt.Println(part.Text)
			}
		}
	}
}
