package runtime

import (
	"context"
	"fmt"
)

const semanticClassifierFailureMessage = "Failed to classify the request."

type SemanticPublisher interface {
	PublishFinal(ctx context.Context, message string) error
	PublishBlocked(ctx context.Context, message string) error
	PublishFailure(ctx context.Context, message string) error
}

type SemanticStepRunner interface {
	Run(ctx context.Context, allowFinalWithoutFreshEvidence bool, ledger *RunLedger) (SemanticRunState, error)
	AddInternalInstruction(instruction string)
}

type SemanticOrchestrator struct {
	Classifier  RequestClassifier
	Runner      SemanticStepRunner
	Publisher   SemanticPublisher
	MemoryStore SemanticMemoryStore
}

func NewSemanticOrchestrator(classifier RequestClassifier, runner SemanticStepRunner, publisher SemanticPublisher) (*SemanticOrchestrator, error) {
	if classifier == nil {
		return nil, fmt.Errorf("semantic orchestrator classifier is required")
	}
	if publisher == nil {
		return nil, fmt.Errorf("semantic orchestrator publisher is required")
	}

	return &SemanticOrchestrator{Classifier: classifier, Runner: runner, Publisher: publisher}, nil
}

func (o *SemanticOrchestrator) WithMemoryStore(store SemanticMemoryStore) *SemanticOrchestrator {
	if o == nil {
		return nil
	}
	o.MemoryStore = store
	return o
}

func (o *SemanticOrchestrator) Run(ctx context.Context, input ClassifierInput) (SemanticRunState, error) {
	if o == nil {
		return SemanticRunState{}, fmt.Errorf("semantic orchestrator is nil")
	}
	if o.Classifier == nil {
		return SemanticRunState{}, fmt.Errorf("semantic orchestrator classifier is required")
	}
	if o.Publisher == nil {
		return SemanticRunState{}, fmt.Errorf("semantic orchestrator publisher is required")
	}
	if err := input.Validate(); err != nil {
		return SemanticRunState{}, err
	}
	if o.MemoryStore != nil {
		if err := validateSemanticMemorySessionID(input.SessionID); err != nil {
			return SemanticRunState{}, err
		}

		snapshot, err := o.MemoryStore.LoadSemanticMemory(ctx, input.SessionID)
		if err != nil {
			return SemanticRunState{}, err
		}
		input = applySemanticMemorySnapshot(input, snapshot)
	}

	output, err := o.Classifier.Classify(ctx, input)
	if err != nil {
		if publishErr := o.Publisher.PublishFailure(ctx, semanticClassifierFailureMessage); publishErr != nil {
			return SemanticRunState{}, publishErr
		}
		return SemanticRunState{Phase: SemanticPhaseFailed}, err
	}
	if err := output.Validate(); err != nil {
		return SemanticRunState{}, err
	}

	decision := DecideSemanticRoute(output)
	switch decision.Action {
	case RouteActionPublishDirectAnswer:
		if err := o.Publisher.PublishFinal(ctx, decision.UserMessage); err != nil {
			return SemanticRunState{}, err
		}
		state := SemanticRunState{Phase: SemanticPhaseDone, Ledger: input.RunLedger}
		if err := o.saveSemanticMemory(ctx, input, state); err != nil {
			return state, err
		}
		return state, nil

	case RouteActionPublishAskUser:
		if err := o.Publisher.PublishBlocked(ctx, decision.UserMessage); err != nil {
			return SemanticRunState{}, err
		}
		state := SemanticRunState{Phase: SemanticPhaseBlocked, ActiveTask: activeTaskFromAskUser(input, decision.UserMessage), Ledger: input.RunLedger}
		if err := o.saveSemanticMemory(ctx, input, state); err != nil {
			return state, err
		}
		return state, nil

	case RouteActionPublishReject:
		if err := o.Publisher.PublishFinal(ctx, decision.UserMessage); err != nil {
			return SemanticRunState{}, err
		}
		state := SemanticRunState{Phase: SemanticPhaseDone, Ledger: input.RunLedger}
		if err := o.saveSemanticMemory(ctx, input, state); err != nil {
			return state, err
		}
		return state, nil

	case RouteActionInitRunState:
		if o.Runner == nil {
			return SemanticRunState{}, fmt.Errorf("semantic orchestrator runner is required for route %s", output.Route)
		}
		if decision.AllowFinalWithoutFreshEvidence {
			o.Runner.AddInternalInstruction(answerFromContextInstruction(input))
		} else if !effectiveLedgerSummary(input).IsZero() {
			o.Runner.AddInternalInstruction(executionLedgerInstruction(input))
		}

		state, err := o.Runner.Run(ctx, decision.AllowFinalWithoutFreshEvidence, input.RunLedger)
		if err != nil {
			_ = o.saveSemanticMemory(ctx, input, state)
			return state, err
		}
		state.AllowFinalWithoutFreshEvidence = decision.AllowFinalWithoutFreshEvidence
		if state.Ledger == nil {
			state.Ledger = input.RunLedger
		}
		if err := o.saveSemanticMemory(ctx, input, state); err != nil {
			return state, err
		}
		return state, nil

	default:
		return SemanticRunState{}, fmt.Errorf("unsupported semantic route action %q", string(decision.Action))
	}
}

func applySemanticMemorySnapshot(input ClassifierInput, snapshot SemanticMemorySnapshot) ClassifierInput {
	if snapshot.RunLedger != nil && !snapshot.RunLedger.IsZero() {
		input.RunLedger = MergeRunLedgers(snapshot.RunLedger, input.RunLedger)
	}
	if input.ActiveTask.IsZero() && !snapshot.ActiveTask.IsZero() {
		input.ActiveTask = snapshot.ActiveTask
	}
	if input.LedgerSummary.IsZero() && input.RunLedger != nil && !input.RunLedger.IsZero() {
		input.LedgerSummary = input.RunLedger.Summary()
	}
	return input
}

func (o *SemanticOrchestrator) saveSemanticMemory(ctx context.Context, input ClassifierInput, state SemanticRunState) error {
	if o == nil || o.MemoryStore == nil {
		return nil
	}
	if err := validateSemanticMemorySessionID(input.SessionID); err != nil {
		return err
	}

	snapshot := SemanticMemorySnapshot{
		RunLedger:  state.Ledger,
		ActiveTask: semanticActiveTaskForSave(input, state),
	}
	if snapshot.RunLedger == nil {
		snapshot.RunLedger = input.RunLedger
	}

	return o.MemoryStore.SaveSemanticMemory(ctx, input.SessionID, snapshot)
}

func semanticActiveTaskForSave(input ClassifierInput, state SemanticRunState) ActiveTaskState {
	if !state.ActiveTask.IsZero() {
		return state.ActiveTask
	}
	if state.Phase == SemanticPhaseBlocked && !input.ActiveTask.IsZero() {
		return input.ActiveTask
	}
	return ActiveTaskState{}
}
