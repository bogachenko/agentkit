package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/bogachenko/agentkit/core/port"
	coreruntime "github.com/bogachenko/agentkit/core/runtime"
	coresession "github.com/bogachenko/agentkit/core/session"
	"google.golang.org/adk/runner"
	"google.golang.org/genai"
)

func TestSemanticHarnessConfigValidateRequiresDependencies(t *testing.T) {
	base := validSemanticHarnessConfig()
	cases := []struct {
		name string
		mut  func(*SemanticHarnessConfig)
	}{
		{name: "runner", mut: func(c *SemanticHarnessConfig) { c.Runner = nil }},
		{name: "user_id", mut: func(c *SemanticHarnessConfig) { c.UserID = "" }},
		{name: "classifier", mut: func(c *SemanticHarnessConfig) { c.Classifier = nil }},
		{name: "publisher", mut: func(c *SemanticHarnessConfig) { c.Publisher = nil }},
		{name: "clock", mut: func(c *SemanticHarnessConfig) { c.Clock = nil }},
		{name: "id_generator", mut: func(c *SemanticHarnessConfig) { c.IDGenerator = nil }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := base
			tc.mut(&config)
			if err := config.Validate(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestSemanticHarnessCommandValidateRequiresFields(t *testing.T) {
	base := validSemanticHarnessCommand()
	cases := []struct {
		name string
		mut  func(*SemanticHarnessCommand)
	}{
		{name: "run_id", mut: func(c *SemanticHarnessCommand) { c.RunID = "" }},
		{name: "session_id", mut: func(c *SemanticHarnessCommand) { c.SessionID = "" }},
		{name: "max_steps", mut: func(c *SemanticHarnessCommand) { c.MaxSteps = 0 }},
		{name: "message", mut: func(c *SemanticHarnessCommand) { c.Message = nil }},
		{name: "user_prompt", mut: func(c *SemanticHarnessCommand) { c.UserPrompt = "" }},
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

func TestNewSemanticHarnessValidatesConfig(t *testing.T) {
	if _, err := NewSemanticHarness(SemanticHarnessConfig{}); err == nil {
		t.Fatal("expected invalid config error")
	}

	harness, err := NewSemanticHarness(validSemanticHarnessConfig())
	if err != nil {
		t.Fatal(err)
	}
	if harness.Config.Runner == nil {
		t.Fatal("runner was not preserved")
	}
}

func TestSemanticHarnessRunRejectsInvalidCommandBeforeADKStepSource(t *testing.T) {
	harness := SemanticHarness{Config: validSemanticHarnessConfig()}
	command := validSemanticHarnessCommand()
	command.Message = nil

	_, err := harness.Run(context.Background(), command)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func validSemanticHarnessConfig() SemanticHarnessConfig {
	return SemanticHarnessConfig{
		Runner:      &runner.Runner{},
		UserID:      "user-1",
		Classifier:  fakeADKSemanticClassifier{output: coreruntime.ClassifierOutput{Route: coreruntime.RouteDirectAnswer, UserMessage: "ok"}},
		Publisher:   &capturingADKPortPublisher{},
		Clock:       adkSemanticTestClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		IDGenerator: &adkSemanticTestIDGenerator{},
	}
}

func validSemanticHarnessCommand() SemanticHarnessCommand {
	return SemanticHarnessCommand{
		RunID:      coreruntime.RunID("run-adk-semantic"),
		SessionID:  coresession.ID("session-adk-semantic"),
		MaxSteps:   10,
		Message:    &genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "run"}}},
		UserPrompt: "run",
	}
}

type fakeADKSemanticClassifier struct {
	output coreruntime.ClassifierOutput
	err    error
}

func (c fakeADKSemanticClassifier) Classify(context.Context, coreruntime.ClassifierInput) (coreruntime.ClassifierOutput, error) {
	if c.err != nil {
		return coreruntime.ClassifierOutput{}, c.err
	}
	return c.output, nil
}

type capturingADKPortPublisher struct {
	events []port.Event
}

func (p *capturingADKPortPublisher) Publish(_ context.Context, event port.Event) error {
	p.events = append(p.events, event)
	return nil
}

type adkSemanticTestClock struct {
	now time.Time
}

func (c adkSemanticTestClock) Now() time.Time {
	return c.now
}

type adkSemanticTestIDGenerator struct{}

func (g *adkSemanticTestIDGenerator) NewID() string {
	return "id-adk-semantic"
}
