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
	Classifier RequestClassifier
	Runner     SemanticStepRunner
	Publisher  SemanticPublisher
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
		return SemanticRunState{Phase: SemanticPhaseDone, Ledger: input.RunLedger}, nil

	case RouteActionPublishAskUser:
		if err := o.Publisher.PublishBlocked(ctx, decision.UserMessage); err != nil {
			return SemanticRunState{}, err
		}
		return SemanticRunState{Phase: SemanticPhaseBlocked, ActiveTask: activeTaskFromAskUser(input, decision.UserMessage), Ledger: input.RunLedger}, nil

	case RouteActionPublishReject:
		if err := o.Publisher.PublishFinal(ctx, decision.UserMessage); err != nil {
			return SemanticRunState{}, err
		}
		return SemanticRunState{Phase: SemanticPhaseDone, Ledger: input.RunLedger}, nil

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
			return state, err
		}
		state.AllowFinalWithoutFreshEvidence = decision.AllowFinalWithoutFreshEvidence
		if state.Ledger == nil {
			state.Ledger = input.RunLedger
		}
		return state, nil

	default:
		return SemanticRunState{}, fmt.Errorf("unsupported semantic route action %q", string(decision.Action))
	}
}
