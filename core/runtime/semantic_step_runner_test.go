package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bogachenko/agentkit/core/session"
	"github.com/bogachenko/agentkit/core/tool"
)

type evidenceGateProvider struct {
	instructions []string
	index        int
}

func (p *evidenceGateProvider) NextSteps(context.Context, State) ([]Step, error) {
	if len(p.instructions) == 0 {
		return []Step{
			{Kind: StepKindAssistantText, Source: StepSourceModel, Text: "premature final", Final: true},
		}, nil
	}

	p.index++
	switch p.index {
	case 1:
		return []Step{
			{Kind: StepKindToolCall, Source: StepSourceModel, ToolCallID: "call-1", ToolName: tool.Name("lookup")},
		}, nil
	case 2:
		return []Step{
			{
				Kind:       StepKindToolResult,
				Source:     StepSourceTool,
				ToolCallID: "call-1",
				ToolName:   tool.Name("lookup"),
				ToolResult: ToolExecutionResult{OK: true, HasEvidence: true, Raw: map[string]any{"ok": true}},
			},
		}, nil
	case 3:
		return []Step{
			{Kind: StepKindAssistantText, Source: StepSourceModel, Text: "final with evidence", Final: true},
		}, nil
	default:
		return nil, ErrStepSourceDone
	}
}

func (p *evidenceGateProvider) AddInternalInstruction(instruction string) {
	p.instructions = append(p.instructions, instruction)
}

type immediateFinalProvider struct{}

func (p immediateFinalProvider) NextSteps(context.Context, State) ([]Step, error) {
	return []Step{
		{Kind: StepKindAssistantText, Source: StepSourceModel, Text: "premature final", Final: true},
	}, nil
}

func TestStepOrchestratorAllowsImmediateFinalByDefault(t *testing.T) {
	provider := &evidenceGateProvider{}
	orchestrator := testEvidenceGateStepOrchestrator(provider)

	result, err := orchestrator.Run(context.Background(), StepRunCommand{
		RunID:     RunID("run-default-final"),
		SessionID: session.ID("session-default-final"),
		MaxSteps:  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunStatusCompleted {
		t.Fatalf("status = %q", result.Status)
	}
	if len(provider.instructions) != 0 {
		t.Fatalf("unexpected instructions: %#v", provider.instructions)
	}
}

func TestStepOrchestratorSuppressesImmediateFinalWhenEvidenceRequired(t *testing.T) {
	provider := &evidenceGateProvider{}
	orchestrator := testEvidenceGateStepOrchestrator(provider)

	result, err := orchestrator.Run(context.Background(), StepRunCommand{
		RunID:                          RunID("run-strict-final"),
		SessionID:                      session.ID("session-strict-final"),
		MaxSteps:                       10,
		RequireToolEvidenceBeforeFinal: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunStatusCompleted {
		t.Fatalf("status = %q", result.Status)
	}
	if len(provider.instructions) == 0 || !strings.Contains(provider.instructions[0], "confirmed tool evidence") {
		t.Fatalf("missing evidence instruction: %#v", provider.instructions)
	}
	if provider.index != 3 {
		t.Fatalf("final accepted before tool evidence, provider index = %d", provider.index)
	}
	if result.StepsCompleted != 4 {
		t.Fatalf("steps completed = %d", result.StepsCompleted)
	}
}

func TestStepOrchestratorFailsStrictImmediateFinalWhenProviderCannotReceiveInstruction(t *testing.T) {
	orchestrator := testEvidenceGateStepOrchestrator(immediateFinalProvider{})

	result, err := orchestrator.Run(context.Background(), StepRunCommand{
		RunID:                          RunID("run-strict-no-receiver"),
		SessionID:                      session.ID("session-strict-no-receiver"),
		MaxSteps:                       10,
		RequireToolEvidenceBeforeFinal: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RunStatusFailed {
		t.Fatalf("status = %q", result.Status)
	}
	if result.Failure == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(result.Failure.Message, "step provider cannot receive internal instructions") {
		t.Fatalf("failure message = %q", result.Failure.Message)
	}
	if result.FinalMessage != nil {
		t.Fatalf("unexpected final message: %#v", result.FinalMessage)
	}
}

func TestSemanticStepRunnerAdapterExecuteTaskRequiresEvidence(t *testing.T) {
	provider := &evidenceGateProvider{}
	adapter := SemanticStepRunnerAdapter{
		Orchestrator: testEvidenceGateStepOrchestrator(provider),
		Command: StepRunCommand{
			RunID:     RunID("run-semantic-execute"),
			SessionID: session.ID("session-semantic-execute"),
			MaxSteps:  10,
		},
	}

	ledger := &RunLedger{TaskID: "semantic-task"}
	state, err := adapter.Run(context.Background(), false, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if state.Phase != SemanticPhaseDone {
		t.Fatalf("phase = %q", state.Phase)
	}
	if state.AllowFinalWithoutFreshEvidence {
		t.Fatal("allow final without fresh evidence = true")
	}
	if state.Ledger != ledger {
		t.Fatal("semantic ledger was not preserved")
	}
	if len(provider.instructions) == 0 || !strings.Contains(provider.instructions[0], "confirmed tool evidence") {
		t.Fatalf("missing evidence instruction: %#v", provider.instructions)
	}
}

func TestSemanticStepRunnerAdapterAnswerFromContextAllowsFinalWithoutFreshEvidence(t *testing.T) {
	provider := &evidenceGateProvider{}
	adapter := SemanticStepRunnerAdapter{
		Orchestrator: testEvidenceGateStepOrchestrator(provider),
		Command: StepRunCommand{
			RunID:     RunID("run-semantic-context"),
			SessionID: session.ID("session-semantic-context"),
			MaxSteps:  10,
		},
	}

	state, err := adapter.Run(context.Background(), true, &RunLedger{TaskID: "context-task"})
	if err != nil {
		t.Fatal(err)
	}
	if state.Phase != SemanticPhaseDone {
		t.Fatalf("phase = %q", state.Phase)
	}
	if !state.AllowFinalWithoutFreshEvidence {
		t.Fatal("allow final without fresh evidence = false")
	}
	if len(provider.instructions) != 0 {
		t.Fatalf("unexpected instructions: %#v", provider.instructions)
	}
}

func TestSemanticStepRunnerAdapterNilReturnsError(t *testing.T) {
	var adapter *SemanticStepRunnerAdapter
	ledger := &RunLedger{TaskID: "nil-adapter"}

	state, err := adapter.Run(context.Background(), false, ledger)
	if err == nil {
		t.Fatal("expected error")
	}
	if state.Phase != SemanticPhaseFailed {
		t.Fatalf("phase = %q", state.Phase)
	}
	if state.Ledger != ledger {
		t.Fatal("semantic ledger was not preserved")
	}
}

func testEvidenceGateStepOrchestrator(provider StepProvider) StepOrchestrator {
	return StepOrchestrator{
		StepProvider: provider,
		Publisher:    &collectingPublisher{},
		Clock:        testClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		IDGenerator:  &testIDGenerator{},
	}
}
