package runtime

import (
	"context"
	"testing"
	"time"

	coresession "github.com/bogachenko/agentkit/core/session"
	"github.com/bogachenko/agentkit/core/tool"
)

func TestSemanticHarnessRunsDirectAnswer(t *testing.T) {
	publisher := &fakeSemanticPublisher{}
	harness := SemanticHarness{
		Classifier: fakeSemanticClassifier{output: ClassifierOutput{Route: RouteDirectAnswer, UserMessage: "ok"}},
		StepRunner: semanticHarnessStepOrchestrator(&semanticHarnessDoneProvider{}),
		Publisher:  publisher,
	}

	state, err := harness.Run(context.Background(), validSemanticHarnessCommand("direct-answer"))
	if err != nil {
		t.Fatal(err)
	}
	if state.Phase != SemanticPhaseDone {
		t.Fatalf("phase = %q", state.Phase)
	}
	if publisher.final != "ok" {
		t.Fatalf("final = %q", publisher.final)
	}
}

func TestSemanticHarnessRunsExecuteTaskThroughStepRunner(t *testing.T) {
	harness := SemanticHarness{
		Classifier: fakeSemanticClassifier{output: ClassifierOutput{Route: RouteExecuteTask}},
		StepRunner: semanticHarnessStepOrchestrator(&semanticHarnessExecuteProvider{}),
		Publisher:  &fakeSemanticPublisher{},
	}

	state, err := harness.Run(context.Background(), validSemanticHarnessCommand("execute-task"))
	if err != nil {
		t.Fatal(err)
	}
	if state.Phase != SemanticPhaseDone {
		t.Fatalf("phase = %q", state.Phase)
	}
	if state.Ledger == nil {
		t.Fatal("expected semantic ledger")
	}
	if len(state.Ledger.DataRefs) == 0 {
		t.Fatalf("data refs = %#v", state.Ledger.DataRefs)
	}
	if !containsString(state.Ledger.CompletedObjectives, "final_answer_produced") {
		t.Fatalf("completed objectives = %#v", state.Ledger.CompletedObjectives)
	}
}

func TestSemanticHarnessLoadsAndSavesMemory(t *testing.T) {
	sessionID := coresession.ID("session-harness-memory")
	classifier := &memoryCapturingClassifier{output: ClassifierOutput{Route: RouteDirectAnswer, UserMessage: "ok"}}
	store := &fakeSemanticMemoryStore{loadedBySession: map[coresession.ID]SemanticMemorySnapshot{
		sessionID: {RunLedger: &RunLedger{UserGoal: "old goal", DataRefs: []string{"stored.xlsx"}}},
	}}
	harness := SemanticHarness{
		Classifier:  classifier,
		StepRunner:  semanticHarnessStepOrchestrator(&semanticHarnessDoneProvider{}),
		Publisher:   &fakeSemanticPublisher{},
		MemoryStore: store,
	}

	command := validSemanticHarnessCommand("memory")
	command.SessionID = sessionID
	_, err := harness.Run(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if classifier.received.LedgerSummary.IsZero() {
		t.Fatal("ledger summary was not loaded before classifier")
	}
	if _, ok := store.savedBySession[sessionID]; !ok {
		t.Fatal("semantic memory was not saved")
	}
}

func TestSemanticHarnessRequiresClassifier(t *testing.T) {
	harness := SemanticHarness{
		StepRunner: semanticHarnessStepOrchestrator(&semanticHarnessDoneProvider{}),
		Publisher:  &fakeSemanticPublisher{},
	}

	_, err := harness.Run(context.Background(), validSemanticHarnessCommand("missing-classifier"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSemanticHarnessRequiresStepRunnerDependencies(t *testing.T) {
	base := SemanticHarness{
		Classifier: fakeSemanticClassifier{output: ClassifierOutput{Route: RouteDirectAnswer, UserMessage: "ok"}},
		StepRunner: semanticHarnessStepOrchestrator(&semanticHarnessDoneProvider{}),
		Publisher:  &fakeSemanticPublisher{},
	}

	cases := []struct {
		name string
		mut  func(*SemanticHarness)
	}{
		{name: "provider", mut: func(h *SemanticHarness) { h.StepRunner.StepProvider = nil }},
		{name: "publisher", mut: func(h *SemanticHarness) { h.StepRunner.Publisher = nil }},
		{name: "clock", mut: func(h *SemanticHarness) { h.StepRunner.Clock = nil }},
		{name: "id_generator", mut: func(h *SemanticHarness) { h.StepRunner.IDGenerator = nil }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			harness := base
			tc.mut(&harness)
			_, err := harness.Run(context.Background(), validSemanticHarnessCommand("missing-"+tc.name))
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestSemanticRunCommandValidateRequiresIdentityAndPrompt(t *testing.T) {
	base := validSemanticHarnessCommand("validate")
	cases := []struct {
		name string
		mut  func(*SemanticRunCommand)
	}{
		{name: "run_id", mut: func(c *SemanticRunCommand) { c.RunID = "" }},
		{name: "session_id", mut: func(c *SemanticRunCommand) { c.SessionID = "" }},
		{name: "user_prompt", mut: func(c *SemanticRunCommand) { c.UserPrompt = "" }},
		{name: "max_steps", mut: func(c *SemanticRunCommand) { c.MaxSteps = 0 }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			command := base
			tc.mut(&command)
			if err := command.Validate(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func validSemanticHarnessCommand(suffix string) SemanticRunCommand {
	return SemanticRunCommand{
		RunID:      RunID("run-" + suffix),
		SessionID:  coresession.ID("session-" + suffix),
		MaxSteps:   10,
		UserPrompt: "run task",
	}
}

func semanticHarnessStepOrchestrator(provider StepProvider) StepOrchestrator {
	return StepOrchestrator{
		StepProvider: provider,
		Publisher:    &collectingPublisher{},
		Clock:        testClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		IDGenerator:  &testIDGenerator{},
	}
}

type semanticHarnessDoneProvider struct{}

func (p *semanticHarnessDoneProvider) NextSteps(context.Context, State) ([]Step, error) {
	return nil, ErrStepSourceDone
}

type semanticHarnessExecuteProvider struct {
	index int
}

func (p *semanticHarnessExecuteProvider) NextSteps(context.Context, State) ([]Step, error) {
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
