package runtime

import (
	"context"
	"strings"
	"testing"
)

type fakeSemanticClassifier struct {
	output ClassifierOutput
	err    error
}

func (f fakeSemanticClassifier) Classify(context.Context, ClassifierInput) (ClassifierOutput, error) {
	return f.output, f.err
}

type fakeSemanticPublisher struct {
	final   string
	blocked string
	failure string
}

func (p *fakeSemanticPublisher) PublishFinal(_ context.Context, message string) error {
	p.final = message
	return nil
}

func (p *fakeSemanticPublisher) PublishBlocked(_ context.Context, message string) error {
	p.blocked = message
	return nil
}

func (p *fakeSemanticPublisher) PublishFailure(_ context.Context, message string) error {
	p.failure = message
	return nil
}

type fakeSemanticRunner struct {
	called                         bool
	allowFinalWithoutFreshEvidence bool
	ledger                         *RunLedger
	instructions                   []string
}

func (r *fakeSemanticRunner) Run(_ context.Context, allowFinalWithoutFreshEvidence bool, ledger *RunLedger) (SemanticRunState, error) {
	r.called = true
	r.allowFinalWithoutFreshEvidence = allowFinalWithoutFreshEvidence
	r.ledger = ledger
	return SemanticRunState{Phase: SemanticPhaseDone, Ledger: ledger}, nil
}

func (r *fakeSemanticRunner) AddInternalInstruction(instruction string) {
	if strings.TrimSpace(instruction) != "" {
		r.instructions = append(r.instructions, instruction)
	}
}

func TestSemanticOrchestratorDirectAnswerDoesNotRunStepRunner(t *testing.T) {
	publisher := &fakeSemanticPublisher{}
	runner := &fakeSemanticRunner{}
	orchestrator, err := NewSemanticOrchestrator(fakeSemanticClassifier{output: ClassifierOutput{Route: RouteDirectAnswer, UserMessage: "<b>ok</b>"}}, runner, publisher)
	if err != nil {
		t.Fatal(err)
	}

	state, err := orchestrator.Run(context.Background(), ClassifierInput{UserPrompt: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if publisher.final != "<b>ok</b>" {
		t.Fatalf("final = %q", publisher.final)
	}
	if runner.called {
		t.Fatal("runner was called")
	}
	if state.Phase != SemanticPhaseDone {
		t.Fatalf("phase = %q", state.Phase)
	}
}

func TestSemanticOrchestratorAskUserCreatesActiveTask(t *testing.T) {
	publisher := &fakeSemanticPublisher{}
	runner := &fakeSemanticRunner{}
	orchestrator, err := NewSemanticOrchestrator(fakeSemanticClassifier{output: ClassifierOutput{Route: RouteAskUser, UserMessage: "Need account ID"}}, runner, publisher)
	if err != nil {
		t.Fatal(err)
	}

	state, err := orchestrator.Run(context.Background(), ClassifierInput{UserPrompt: "Check campaign"})
	if err != nil {
		t.Fatal(err)
	}
	if publisher.blocked != "Need account ID" {
		t.Fatalf("blocked = %q", publisher.blocked)
	}
	if runner.called {
		t.Fatal("runner was called")
	}
	if state.Phase != SemanticPhaseBlocked {
		t.Fatalf("phase = %q", state.Phase)
	}
	if !state.ActiveTask.Active {
		t.Fatal("active task is not active")
	}
	if state.ActiveTask.OriginalRequest != "Check campaign" {
		t.Fatalf("original request = %q", state.ActiveTask.OriginalRequest)
	}
	if state.ActiveTask.LastResultSummary != "Need account ID" {
		t.Fatalf("last result summary = %q", state.ActiveTask.LastResultSummary)
	}
}

func TestSemanticOrchestratorRejectDoesNotRunStepRunner(t *testing.T) {
	publisher := &fakeSemanticPublisher{}
	runner := &fakeSemanticRunner{}
	orchestrator, err := NewSemanticOrchestrator(fakeSemanticClassifier{output: ClassifierOutput{Route: RouteRejectUnsupported, UserMessage: "Unsupported"}}, runner, publisher)
	if err != nil {
		t.Fatal(err)
	}

	state, err := orchestrator.Run(context.Background(), ClassifierInput{UserPrompt: "do impossible thing"})
	if err != nil {
		t.Fatal(err)
	}
	if publisher.final != "Unsupported" {
		t.Fatalf("final = %q", publisher.final)
	}
	if runner.called {
		t.Fatal("runner was called")
	}
	if state.Phase != SemanticPhaseDone {
		t.Fatalf("phase = %q", state.Phase)
	}
}

func TestSemanticOrchestratorExecuteTaskRunsWithStrictEvidenceGateFlag(t *testing.T) {
	publisher := &fakeSemanticPublisher{}
	runner := &fakeSemanticRunner{}
	orchestrator, err := NewSemanticOrchestrator(fakeSemanticClassifier{output: ClassifierOutput{Route: RouteExecuteTask}}, runner, publisher)
	if err != nil {
		t.Fatal(err)
	}

	_, err = orchestrator.Run(context.Background(), ClassifierInput{UserPrompt: "run task"})
	if err != nil {
		t.Fatal(err)
	}
	if !runner.called {
		t.Fatal("runner was not called")
	}
	if runner.allowFinalWithoutFreshEvidence {
		t.Fatal("allowFinalWithoutFreshEvidence = true")
	}
}

func TestSemanticOrchestratorAnswerFromContextRunsWithAllowFinalWithoutFreshEvidence(t *testing.T) {
	publisher := &fakeSemanticPublisher{}
	runner := &fakeSemanticRunner{}
	orchestrator, err := NewSemanticOrchestrator(fakeSemanticClassifier{output: ClassifierOutput{Route: RouteAnswerFromContext}}, runner, publisher)
	if err != nil {
		t.Fatal(err)
	}

	_, err = orchestrator.Run(context.Background(), ClassifierInput{
		UserPrompt: "summarize it",
		LedgerSummary: RunLedgerSummary{Present: true, UserGoal: "Audit WB card", CurrentPhase: "ready_to_answer"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !runner.called {
		t.Fatal("runner was not called")
	}
	if !runner.allowFinalWithoutFreshEvidence {
		t.Fatal("allowFinalWithoutFreshEvidence = false")
	}
	if len(runner.instructions) == 0 || !strings.Contains(runner.instructions[0], "Do not call additional tools") {
		t.Fatalf("instruction missing tool-call prohibition: %#v", runner.instructions)
	}
}

func TestRunLedgerSummaryIsSemanticNotCounters(t *testing.T) {
	ledger := &RunLedger{
		TaskID:               "task-1",
		UserGoal:             "Audit listing",
		CurrentPhase:         "analysis_done",
		DataRefs:             []string{"wb-card.xlsx"},
		Artifacts:            []RunLedgerArtifact{{ID: "a1", Kind: "xlsx", Name: "report", Ref: "file://report.xlsx", Summary: "audit report"}},
		CompletedObjectives:  []string{"Loaded card data"},
		FailedObjectives:     []string{"Fetch ad stats"},
		OpenQuestions:        []string{"Need campaign ID"},
		AvailableNextActions: []string{"export_excel"},
	}

	summary := ledger.Summary()
	if !summary.Present {
		t.Fatal("summary is not present")
	}
	if summary.UserGoal != "Audit listing" || summary.CurrentPhase != "analysis_done" {
		t.Fatalf("semantic fields not preserved: %#v", summary)
	}
	if len(summary.AvailableData) != 1 || summary.AvailableData[0] != "wb-card.xlsx" {
		t.Fatalf("available data not preserved: %#v", summary.AvailableData)
	}
	if len(summary.Artifacts) != 1 || summary.Artifacts[0].Name != "report" {
		t.Fatalf("artifacts not preserved: %#v", summary.Artifacts)
	}
	if len(summary.CompletedObjectives) != 1 || len(summary.FailedObjectives) != 1 || len(summary.OpenQuestions) != 1 || len(summary.AvailableNextActions) != 1 {
		t.Fatalf("semantic objective fields missing: %#v", summary)
	}
}

func TestMergeRunLedgersPreservesExistingAndIncomingSemanticMemory(t *testing.T) {
	existing := &RunLedger{CompletedObjectives: []string{"Loaded data"}, DataRefs: []string{"source.xlsx"}}
	incoming := &RunLedger{CompletedObjectives: []string{"Loaded data", "Generated report"}, Artifacts: []RunLedgerArtifact{{ID: "artifact-1", Kind: "xlsx", Name: "report"}}}

	merged := MergeRunLedgers(existing, incoming)
	if len(merged.CompletedObjectives) != 2 {
		t.Fatalf("completed objectives = %#v", merged.CompletedObjectives)
	}
	if len(merged.DataRefs) != 1 || merged.DataRefs[0] != "source.xlsx" {
		t.Fatalf("data refs = %#v", merged.DataRefs)
	}
	if len(merged.Artifacts) != 1 || merged.Artifacts[0].ID != "artifact-1" {
		t.Fatalf("artifacts = %#v", merged.Artifacts)
	}
}
