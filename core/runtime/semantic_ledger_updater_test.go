package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/bogachenko/agentkit/core/session"
	"github.com/bogachenko/agentkit/core/tool"
)

func TestApplySemanticStepToRunLedgerToolCall(t *testing.T) {
	ledger := &RunLedger{}

	ApplySemanticStepToRunLedger(ledger, Step{Kind: StepKindToolCall, ToolName: tool.Name("lookup")})

	if ledger.CurrentPhase != "executing" {
		t.Fatalf("current phase = %q", ledger.CurrentPhase)
	}
	if len(ledger.Steps) != 1 {
		t.Fatalf("steps = %#v", ledger.Steps)
	}
	if ledger.Steps[0].Kind != "tool_call" || ledger.Steps[0].Status != "started" {
		t.Fatalf("step = %#v", ledger.Steps[0])
	}
	if !containsString(ledger.AvailableNextActions, "await_tool_result") {
		t.Fatalf("available actions = %#v", ledger.AvailableNextActions)
	}
}

func TestApplySemanticStepToRunLedgerSuccessfulToolResultWithEvidence(t *testing.T) {
	ledger := &RunLedger{}

	ApplySemanticStepToRunLedger(ledger, Step{
		ID:       StepID("s1"),
		Kind:     StepKindToolResult,
		ToolName: tool.Name("lookup"),
		ToolResult: ToolExecutionResult{
			OK:          true,
			HasEvidence: true,
		},
	})

	if ledger.CurrentPhase != "has_data" {
		t.Fatalf("current phase = %q", ledger.CurrentPhase)
	}
	if len(ledger.Steps) != 1 || ledger.Steps[0].Kind != "tool_result" || ledger.Steps[0].Status != "completed" {
		t.Fatalf("steps = %#v", ledger.Steps)
	}
	if !containsString(ledger.DataRefs, "lookup:s1") {
		t.Fatalf("data refs = %#v", ledger.DataRefs)
	}
	if !containsString(ledger.CompletedObjectives, "tool lookup returned result") {
		t.Fatalf("completed objectives = %#v", ledger.CompletedObjectives)
	}
	if !containsString(ledger.AvailableNextActions, "answer_from_evidence") {
		t.Fatalf("available actions = %#v", ledger.AvailableNextActions)
	}
}

func TestApplySemanticStepToRunLedgerFailedValidationResult(t *testing.T) {
	ledger := &RunLedger{}

	ApplySemanticStepToRunLedger(ledger, Step{
		Kind:     StepKindToolResult,
		ToolName: tool.Name("lookup"),
		ToolResult: ToolExecutionResult{
			OK:           false,
			ErrorKind:    ToolErrorValidation,
			ErrorMessage: "missing input",
		},
	})

	if ledger.CurrentPhase != "needs_retry" {
		t.Fatalf("current phase = %q", ledger.CurrentPhase)
	}
	if !containsString(ledger.FailedObjectives, "tool lookup failed") {
		t.Fatalf("failed objectives = %#v", ledger.FailedObjectives)
	}
	if !containsString(ledger.AvailableNextActions, "retry_with_corrected_arguments") {
		t.Fatalf("available actions = %#v", ledger.AvailableNextActions)
	}
	if !containsString(ledger.Summary().BlockersOrErrors, "missing input") {
		t.Fatalf("blockers = %#v", ledger.Summary().BlockersOrErrors)
	}
}

func TestApplySemanticStepToRunLedgerFailedFatalResult(t *testing.T) {
	ledger := &RunLedger{}

	ApplySemanticStepToRunLedger(ledger, Step{
		Kind:     StepKindToolResult,
		ToolName: tool.Name("lookup"),
		ToolResult: ToolExecutionResult{
			OK:           false,
			ErrorKind:    ToolErrorFatal,
			ErrorMessage: "service down",
		},
	})

	if ledger.CurrentPhase != "blocked" {
		t.Fatalf("current phase = %q", ledger.CurrentPhase)
	}
	if !containsString(ledger.AvailableNextActions, "explain_blocker") {
		t.Fatalf("available actions = %#v", ledger.AvailableNextActions)
	}
}

func TestApplySemanticStepToRunLedgerArtifactExtraction(t *testing.T) {
	ledger := &RunLedger{}

	ApplySemanticStepToRunLedger(ledger, Step{
		Kind:     StepKindToolResult,
		ToolName: tool.Name("export"),
		ToolResult: ToolExecutionResult{
			OK: true,
			Raw: map[string]any{
				"artifact": map[string]any{
					"id":      "a1",
					"kind":    "xlsx",
					"name":    "report.xlsx",
					"ref":     "file://report.xlsx",
					"summary": "generated report",
				},
			},
		},
	})

	if len(ledger.Artifacts) != 1 {
		t.Fatalf("artifacts = %#v", ledger.Artifacts)
	}
	if ledger.Artifacts[0].ID != "a1" || ledger.Artifacts[0].Name != "report.xlsx" {
		t.Fatalf("artifact = %#v", ledger.Artifacts[0])
	}
}

func TestSemanticLedgerStepProviderPopulatesLedger(t *testing.T) {
	provider := &semanticLedgerStreamProvider{}
	ledger := &RunLedger{}
	adapter := SemanticStepRunnerAdapter{
		Orchestrator: testSemanticLedgerStepOrchestrator(provider),
		Command: StepRunCommand{
			RunID:     RunID("run-semantic-ledger"),
			SessionID: session.ID("session-semantic-ledger"),
			MaxSteps:  10,
		},
	}

	state, err := adapter.Run(context.Background(), false, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if state.Phase != SemanticPhaseDone {
		t.Fatalf("phase = %q", state.Phase)
	}
	if ledger.CurrentPhase != "answered" {
		t.Fatalf("current phase = %q", ledger.CurrentPhase)
	}
	if len(ledger.DataRefs) == 0 {
		t.Fatal("expected data refs")
	}
	if !containsString(ledger.CompletedObjectives, "final_answer_produced") {
		t.Fatalf("completed objectives = %#v", ledger.CompletedObjectives)
	}
	summary := ledger.Summary()
	if !summary.Present {
		t.Fatal("summary is not present")
	}
	if len(summary.AvailableData) == 0 {
		t.Fatalf("available data = %#v", summary.AvailableData)
	}
}

func TestSemanticLedgerStepProviderPassesInternalInstructionsThrough(t *testing.T) {
	inner := &semanticLedgerStreamProvider{}
	wrapper := &semanticLedgerStepProvider{inner: inner, ledger: &RunLedger{}}

	wrapper.AddInternalInstruction("x")

	if len(inner.instructions) != 1 || inner.instructions[0] != "x" {
		t.Fatalf("instructions = %#v", inner.instructions)
	}
}

type semanticLedgerStreamProvider struct {
	index        int
	instructions []string
}

func (p *semanticLedgerStreamProvider) NextSteps(context.Context, State) ([]Step, error) {
	p.index++
	switch p.index {
	case 1:
		return []Step{{Kind: StepKindToolCall, Source: StepSourceModel, ToolCallID: "call-1", ToolName: tool.Name("lookup")}}, nil
	case 2:
		return []Step{{Kind: StepKindToolResult, Source: StepSourceTool, ToolCallID: "call-1", ToolName: tool.Name("lookup"), ToolResult: ToolExecutionResult{OK: true, HasEvidence: true}}}, nil
	case 3:
		return []Step{{Kind: StepKindAssistantText, Source: StepSourceModel, Text: "final", Final: true}}, nil
	default:
		return nil, ErrStepSourceDone
	}
}

func (p *semanticLedgerStreamProvider) AddInternalInstruction(instruction string) {
	p.instructions = append(p.instructions, instruction)
}

func testSemanticLedgerStepOrchestrator(provider StepProvider) StepOrchestrator {
	return StepOrchestrator{
		StepProvider: provider,
		Publisher:    &collectingPublisher{},
		Clock:        testClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		IDGenerator:  &testIDGenerator{},
	}
}
