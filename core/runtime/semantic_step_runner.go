package runtime

import (
	"context"
	"fmt"
	"strings"
)

type SemanticStepRunnerAdapter struct {
	Orchestrator StepOrchestrator
	Command      StepRunCommand
}

func (r *SemanticStepRunnerAdapter) AddInternalInstruction(instruction string) {
	instruction = strings.TrimSpace(instruction)
	if instruction == "" || r == nil {
		return
	}

	receiver, ok := r.Orchestrator.StepProvider.(InternalInstructionReceiver)
	if !ok || receiver == nil {
		return
	}

	receiver.AddInternalInstruction(instruction)
}

func (r *SemanticStepRunnerAdapter) Run(ctx context.Context, allowFinalWithoutFreshEvidence bool, ledger *RunLedger) (SemanticRunState, error) {
	if r == nil {
		return SemanticRunState{
			Phase:                          SemanticPhaseFailed,
			AllowFinalWithoutFreshEvidence: allowFinalWithoutFreshEvidence,
			Ledger:                         ledger,
		}, fmt.Errorf("semantic step runner adapter is nil")
	}

	command := r.Command
	command.RequireToolEvidenceBeforeFinal = !allowFinalWithoutFreshEvidence

	result, err := r.Orchestrator.Run(ctx, command)
	state := SemanticRunState{
		Phase:                          semanticPhaseFromRunStatus(result.Status),
		AllowFinalWithoutFreshEvidence: allowFinalWithoutFreshEvidence,
		Ledger:                         ledger,
	}
	if err != nil {
		return state, err
	}

	return state, nil
}

func semanticPhaseFromRunStatus(status RunStatus) SemanticPhase {
	switch status {
	case RunStatusCompleted:
		return SemanticPhaseDone
	case RunStatusBlocked:
		return SemanticPhaseBlocked
	case RunStatusFailed:
		return SemanticPhaseFailed
	default:
		return SemanticPhaseFailed
	}
}
