package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/bogachenko/agentkit/core/port"
	coresession "github.com/bogachenko/agentkit/core/session"
)

type SemanticHarness struct {
	Classifier  RequestClassifier
	StepRunner  StepOrchestrator
	Publisher   SemanticPublisher
	MemoryStore SemanticMemoryStore
}

type SemanticRunCommand struct {
	RunID     RunID
	SessionID coresession.ID
	MaxSteps  int

	UserPrompt          string
	ConversationContext []string
	PendingUserInput    string

	ActiveTask    ActiveTaskState
	RunLedger     *RunLedger
	LedgerSummary RunLedgerSummary

	Tools                []ToolCatalogItem
	Artifacts            []string
	CredentialsOrSources []string
	Skills               []string
	SessionConstraints   []string

	TraceInput string
}

func (c SemanticRunCommand) Validate() error {
	if err := c.RunID.Validate(); err != nil {
		return err
	}
	if err := c.SessionID.Validate(); err != nil {
		return err
	}
	if c.MaxSteps <= 0 {
		return fmt.Errorf("semantic run command max steps must be positive")
	}
	if strings.TrimSpace(c.UserPrompt) == "" {
		return fmt.Errorf("semantic run command user prompt is required")
	}
	return nil
}

func (h SemanticHarness) Run(ctx context.Context, command SemanticRunCommand) (SemanticRunState, error) {
	if err := h.validateDependencies(); err != nil {
		return SemanticRunState{}, err
	}
	if err := command.Validate(); err != nil {
		return SemanticRunState{}, err
	}

	runner := &SemanticStepRunnerAdapter{
		Orchestrator: h.StepRunner,
		Command: StepRunCommand{
			RunID:      command.RunID,
			SessionID:  command.SessionID,
			MaxSteps:   command.MaxSteps,
			TraceInput: command.TraceInput,
		},
	}

	orchestrator, err := NewSemanticOrchestrator(h.Classifier, runner, h.Publisher)
	if err != nil {
		return SemanticRunState{}, err
	}
	orchestrator.WithMemoryStore(h.MemoryStore)

	return orchestrator.Run(ctx, ClassifierInput{
		SessionID:            command.SessionID,
		UserPrompt:           command.UserPrompt,
		ConversationContext:  command.ConversationContext,
		PendingUserInput:     command.PendingUserInput,
		ActiveTask:           command.ActiveTask,
		RunLedger:            command.RunLedger,
		LedgerSummary:        command.LedgerSummary,
		Tools:                command.Tools,
		Artifacts:            command.Artifacts,
		CredentialsOrSources: command.CredentialsOrSources,
		Skills:               command.Skills,
		SessionConstraints:   command.SessionConstraints,
	})
}

func (h SemanticHarness) WithPortPublisher(publisher port.Publisher, clock port.Clock, runID RunID, sessionID coresession.ID) SemanticHarness {
	next := h
	next.Publisher = NewSemanticPublisherAdapter(publisher, clock, runID, sessionID)
	return next
}

func (h SemanticHarness) validateDependencies() error {
	if h.Classifier == nil {
		return fmt.Errorf("semantic harness classifier is required")
	}
	if h.Publisher == nil {
		return fmt.Errorf("semantic harness publisher is required")
	}
	if h.StepRunner.StepProvider == nil {
		return fmt.Errorf("semantic harness step runner provider is required")
	}
	if h.StepRunner.Publisher == nil {
		return fmt.Errorf("semantic harness step runner publisher is required")
	}
	if h.StepRunner.Clock == nil {
		return fmt.Errorf("semantic harness step runner clock is required")
	}
	if h.StepRunner.IDGenerator == nil {
		return fmt.Errorf("semantic harness step runner id generator is required")
	}
	return nil
}
