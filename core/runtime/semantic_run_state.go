package runtime

type SemanticPhase string

const (
	SemanticPhaseWorking SemanticPhase = "WORKING"
	SemanticPhaseBlocked SemanticPhase = "BLOCKED"
	SemanticPhaseDone    SemanticPhase = "DONE"
	SemanticPhaseFailed  SemanticPhase = "FAILED"
)

type SemanticRunState struct {
	Phase                          SemanticPhase
	AllowFinalWithoutFreshEvidence bool
	ActiveTask                     ActiveTaskState
	Ledger                         *RunLedger
}
