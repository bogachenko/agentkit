package runtime

import (
	"context"
	"errors"
	"testing"

	coresession "github.com/bogachenko/agentkit/core/session"
)

type fakeSemanticMemoryStore struct {
	loadedBySession map[coresession.ID]SemanticMemorySnapshot
	savedBySession  map[coresession.ID]SemanticMemorySnapshot
	loadErr         error
	saveErr         error
}

func (s *fakeSemanticMemoryStore) LoadSemanticMemory(_ context.Context, sessionID coresession.ID) (SemanticMemorySnapshot, error) {
	if s.loadErr != nil {
		return SemanticMemorySnapshot{}, s.loadErr
	}
	if s.loadedBySession == nil {
		return SemanticMemorySnapshot{}, nil
	}
	return s.loadedBySession[sessionID], nil
}

func (s *fakeSemanticMemoryStore) SaveSemanticMemory(_ context.Context, sessionID coresession.ID, snapshot SemanticMemorySnapshot) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	if s.savedBySession == nil {
		s.savedBySession = map[coresession.ID]SemanticMemorySnapshot{}
	}
	s.savedBySession[sessionID] = snapshot
	return nil
}

type memoryCapturingClassifier struct {
	output   ClassifierOutput
	err      error
	called   bool
	received ClassifierInput
}

func (c *memoryCapturingClassifier) Classify(_ context.Context, input ClassifierInput) (ClassifierOutput, error) {
	c.called = true
	c.received = input
	return c.output, c.err
}

type memoryStateRunner struct {
	state SemanticRunState
	err   error
}

func (r *memoryStateRunner) Run(context.Context, bool, *RunLedger) (SemanticRunState, error) {
	return r.state, r.err
}

func (r *memoryStateRunner) AddInternalInstruction(string) {}

func TestSemanticOrchestratorLoadsMemoryBeforeClassifier(t *testing.T) {
	sessionID := coresession.ID("session-memory-load")
	classifier := &memoryCapturingClassifier{output: ClassifierOutput{Route: RouteDirectAnswer, UserMessage: "ok"}}
	store := &fakeSemanticMemoryStore{loadedBySession: map[coresession.ID]SemanticMemorySnapshot{
		sessionID: {
			RunLedger:  &RunLedger{UserGoal: "old goal", DataRefs: []string{"old.xlsx"}},
			ActiveTask: ActiveTaskState{Active: true, OriginalRequest: "old request"},
		},
	}}
	orchestrator, err := NewSemanticOrchestrator(classifier, nil, &fakeSemanticPublisher{})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator.WithMemoryStore(store)

	_, err = orchestrator.Run(context.Background(), ClassifierInput{SessionID: sessionID, UserPrompt: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if classifier.received.RunLedger == nil || classifier.received.RunLedger.UserGoal != "old goal" {
		t.Fatalf("run ledger = %#v", classifier.received.RunLedger)
	}
	if classifier.received.ActiveTask.IsZero() {
		t.Fatal("active task was not loaded")
	}
	if classifier.received.LedgerSummary.IsZero() {
		t.Fatal("ledger summary was not populated")
	}
}

func TestSemanticOrchestratorMergesIncomingLedgerWithStoredLedger(t *testing.T) {
	sessionID := coresession.ID("session-memory-merge")
	classifier := &memoryCapturingClassifier{output: ClassifierOutput{Route: RouteDirectAnswer, UserMessage: "ok"}}
	store := &fakeSemanticMemoryStore{loadedBySession: map[coresession.ID]SemanticMemorySnapshot{
		sessionID: {RunLedger: &RunLedger{DataRefs: []string{"stored.xlsx"}}},
	}}
	orchestrator, err := NewSemanticOrchestrator(classifier, nil, &fakeSemanticPublisher{})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator.WithMemoryStore(store)

	_, err = orchestrator.Run(context.Background(), ClassifierInput{
		SessionID:  sessionID,
		UserPrompt: "status",
		RunLedger:  &RunLedger{Artifacts: []RunLedgerArtifact{{ID: "a1"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(classifier.received.RunLedger.DataRefs, "stored.xlsx") {
		t.Fatalf("data refs = %#v", classifier.received.RunLedger.DataRefs)
	}
	if len(classifier.received.RunLedger.Artifacts) != 1 || classifier.received.RunLedger.Artifacts[0].ID != "a1" {
		t.Fatalf("artifacts = %#v", classifier.received.RunLedger.Artifacts)
	}
}

func TestSemanticOrchestratorAskUserSavesActiveTask(t *testing.T) {
	sessionID := coresession.ID("session-memory-ask")
	classifier := &memoryCapturingClassifier{output: ClassifierOutput{Route: RouteAskUser, UserMessage: "Need account ID"}}
	store := &fakeSemanticMemoryStore{}
	orchestrator, err := NewSemanticOrchestrator(classifier, nil, &fakeSemanticPublisher{})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator.WithMemoryStore(store)

	_, err = orchestrator.Run(context.Background(), ClassifierInput{SessionID: sessionID, UserPrompt: "check account"})
	if err != nil {
		t.Fatal(err)
	}
	saved := store.savedBySession[sessionID]
	if !saved.ActiveTask.Active {
		t.Fatal("active task was not saved")
	}
	if saved.ActiveTask.LastResultSummary != "Need account ID" {
		t.Fatalf("last result summary = %q", saved.ActiveTask.LastResultSummary)
	}
}

func TestSemanticOrchestratorExecuteTaskSavesPopulatedRunLedger(t *testing.T) {
	sessionID := coresession.ID("session-memory-execute")
	ledger := &RunLedger{DataRefs: []string{"tool:step-1"}, CompletedObjectives: []string{"tool lookup returned result"}}
	classifier := &memoryCapturingClassifier{output: ClassifierOutput{Route: RouteExecuteTask}}
	store := &fakeSemanticMemoryStore{}
	runner := &memoryStateRunner{state: SemanticRunState{Phase: SemanticPhaseDone, Ledger: ledger}}
	orchestrator, err := NewSemanticOrchestrator(classifier, runner, &fakeSemanticPublisher{})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator.WithMemoryStore(store)

	_, err = orchestrator.Run(context.Background(), ClassifierInput{SessionID: sessionID, UserPrompt: "run task"})
	if err != nil {
		t.Fatal(err)
	}
	saved := store.savedBySession[sessionID]
	if saved.RunLedger != ledger {
		t.Fatalf("saved ledger = %#v", saved.RunLedger)
	}
	if !containsString(saved.RunLedger.DataRefs, "tool:step-1") {
		t.Fatalf("data refs = %#v", saved.RunLedger.DataRefs)
	}
}

func TestSemanticOrchestratorDoneClearsActiveTask(t *testing.T) {
	sessionID := coresession.ID("session-memory-clear")
	classifier := &memoryCapturingClassifier{output: ClassifierOutput{Route: RouteDirectAnswer, UserMessage: "done"}}
	store := &fakeSemanticMemoryStore{loadedBySession: map[coresession.ID]SemanticMemorySnapshot{
		sessionID: {ActiveTask: ActiveTaskState{Active: true, OriginalRequest: "old request"}},
	}}
	orchestrator, err := NewSemanticOrchestrator(classifier, nil, &fakeSemanticPublisher{})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator.WithMemoryStore(store)

	_, err = orchestrator.Run(context.Background(), ClassifierInput{SessionID: sessionID, UserPrompt: "thanks"})
	if err != nil {
		t.Fatal(err)
	}
	if !store.savedBySession[sessionID].ActiveTask.IsZero() {
		t.Fatalf("active task was not cleared: %#v", store.savedBySession[sessionID].ActiveTask)
	}
}

func TestSemanticOrchestratorMemoryStoreRequiresSessionID(t *testing.T) {
	classifier := &memoryCapturingClassifier{output: ClassifierOutput{Route: RouteDirectAnswer, UserMessage: "ok"}}
	orchestrator, err := NewSemanticOrchestrator(classifier, nil, &fakeSemanticPublisher{})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator.WithMemoryStore(&fakeSemanticMemoryStore{})

	_, err = orchestrator.Run(context.Background(), ClassifierInput{UserPrompt: "status"})
	if err == nil {
		t.Fatal("expected error")
	}
	if classifier.called {
		t.Fatal("classifier was called")
	}
}

func TestSemanticOrchestratorMemoryLoadErrorStopsBeforeClassifier(t *testing.T) {
	sessionID := coresession.ID("session-memory-load-error")
	classifier := &memoryCapturingClassifier{output: ClassifierOutput{Route: RouteDirectAnswer, UserMessage: "ok"}}
	orchestrator, err := NewSemanticOrchestrator(classifier, nil, &fakeSemanticPublisher{})
	if err != nil {
		t.Fatal(err)
	}
	orchestrator.WithMemoryStore(&fakeSemanticMemoryStore{loadErr: errors.New("load failed")})

	_, err = orchestrator.Run(context.Background(), ClassifierInput{SessionID: sessionID, UserPrompt: "status"})
	if err == nil {
		t.Fatal("expected error")
	}
	if classifier.called {
		t.Fatal("classifier was called")
	}
}

func TestSemanticOrchestratorMemorySaveErrorReturned(t *testing.T) {
	sessionID := coresession.ID("session-memory-save-error")
	classifier := &memoryCapturingClassifier{output: ClassifierOutput{Route: RouteDirectAnswer, UserMessage: "ok"}}
	publisher := &fakeSemanticPublisher{}
	orchestrator, err := NewSemanticOrchestrator(classifier, nil, publisher)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator.WithMemoryStore(&fakeSemanticMemoryStore{saveErr: errors.New("save failed")})

	state, err := orchestrator.Run(context.Background(), ClassifierInput{SessionID: sessionID, UserPrompt: "status"})
	if err == nil {
		t.Fatal("expected error")
	}
	if state.Phase != SemanticPhaseDone {
		t.Fatalf("phase = %q", state.Phase)
	}
	if publisher.final != "ok" {
		t.Fatalf("final = %q", publisher.final)
	}
}
