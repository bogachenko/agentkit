package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/bogachenko/agentkit/core/port"
	coreruntime "github.com/bogachenko/agentkit/core/runtime"
	coresession "github.com/bogachenko/agentkit/core/session"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/genai"
)

type SemanticHarnessConfig struct {
	Runner     *runner.Runner
	UserID     string
	RunConfig  agent.RunConfig
	RunOptions []runner.RunOption

	Classifier coreruntime.RequestClassifier

	Publisher   port.Publisher
	Clock       port.Clock
	IDGenerator port.IDGenerator
	Tracer      port.Tracer

	MemoryStore coreruntime.SemanticMemoryStore
}

func (c SemanticHarnessConfig) Validate() error {
	if c.Runner == nil {
		return fmt.Errorf("adk semantic harness runner is required")
	}
	if strings.TrimSpace(c.UserID) == "" {
		return fmt.Errorf("adk semantic harness user id is required")
	}
	if c.Classifier == nil {
		return fmt.Errorf("adk semantic harness classifier is required")
	}
	if c.Publisher == nil {
		return fmt.Errorf("adk semantic harness publisher is required")
	}
	if c.Clock == nil {
		return fmt.Errorf("adk semantic harness clock is required")
	}
	if c.IDGenerator == nil {
		return fmt.Errorf("adk semantic harness id generator is required")
	}
	return nil
}

type SemanticHarnessCommand struct {
	RunID     coreruntime.RunID
	SessionID coresession.ID
	MaxSteps  int

	Message *genai.Content

	UserPrompt          string
	ConversationContext []string
	PendingUserInput    string

	ActiveTask    coreruntime.ActiveTaskState
	RunLedger     *coreruntime.RunLedger
	LedgerSummary coreruntime.RunLedgerSummary

	Tools                []coreruntime.ToolCatalogItem
	Artifacts            []string
	CredentialsOrSources []string
	Skills               []string
	SessionConstraints   []string

	TraceInput string
}

func (c SemanticHarnessCommand) Validate() error {
	if err := c.RunID.Validate(); err != nil {
		return err
	}
	if err := c.SessionID.Validate(); err != nil {
		return err
	}
	if c.MaxSteps <= 0 {
		return fmt.Errorf("adk semantic harness command max steps must be positive")
	}
	if c.Message == nil {
		return fmt.Errorf("adk semantic harness command message is required")
	}
	if strings.TrimSpace(c.UserPrompt) == "" {
		return fmt.Errorf("adk semantic harness command user prompt is required")
	}
	return nil
}

type SemanticHarness struct {
	Config SemanticHarnessConfig
}

func NewSemanticHarness(config SemanticHarnessConfig) (SemanticHarness, error) {
	if err := config.Validate(); err != nil {
		return SemanticHarness{}, err
	}
	return SemanticHarness{Config: config}, nil
}

func (h SemanticHarness) Run(ctx context.Context, command SemanticHarnessCommand) (coreruntime.SemanticRunState, error) {
	if err := h.Config.Validate(); err != nil {
		return coreruntime.SemanticRunState{}, err
	}
	if err := command.Validate(); err != nil {
		return coreruntime.SemanticRunState{}, err
	}

	stepSource, err := NewADKStepSource(
		h.Config.Runner,
		h.Config.UserID,
		string(command.SessionID),
		command.Message,
		h.Config.RunConfig,
		h.Config.RunOptions...,
	)
	if err != nil {
		return coreruntime.SemanticRunState{}, err
	}

	stepRunner := coreruntime.StepOrchestrator{
		StepProvider: stepSource,
		Publisher:    h.Config.Publisher,
		Clock:        h.Config.Clock,
		IDGenerator:  h.Config.IDGenerator,
		Tracer:       h.Config.Tracer,
	}
	semanticPublisher := coreruntime.NewSemanticPublisherAdapter(
		h.Config.Publisher,
		h.Config.Clock,
		command.RunID,
		command.SessionID,
	)
	coreHarness := coreruntime.SemanticHarness{
		Classifier:  h.Config.Classifier,
		StepRunner:  stepRunner,
		Publisher:   semanticPublisher,
		MemoryStore: h.Config.MemoryStore,
	}

	return coreHarness.Run(ctx, coreruntime.SemanticRunCommand{
		RunID:                command.RunID,
		SessionID:            command.SessionID,
		MaxSteps:             command.MaxSteps,
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
		TraceInput:           command.TraceInput,
	})
}
