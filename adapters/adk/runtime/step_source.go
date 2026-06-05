package runtime

import (
	"context"
	"fmt"
	"strings"

	coreruntime "github.com/bogachenko/agentkit/core/runtime"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/genai"
)

// ADKStepSource converts ADK runner events into neutral AgentKit runtime steps.
//
// It intentionally consumes ADK Runner synchronously. Runner.Run already exposes a
// pull-style iterator, and the harness must be able to decide when to continue
// by injecting internal runtime instructions between ADK turns.
type ADKStepSource struct {
	Runner     *runner.Runner
	UserID     string
	SessionID  string
	Message    *genai.Content
	RunConfig  agent.RunConfig
	RunOptions []runner.RunOption

	started             bool
	queue               []coreruntime.Step
	pendingInstructions []string
}

// NewADKStepSource validates ADK execution inputs before runtime starts consuming steps.
func NewADKStepSource(
	r *runner.Runner,
	userID string,
	sessionID string,
	message *genai.Content,
	runConfig agent.RunConfig,
	runOptions ...runner.RunOption,
) (*ADKStepSource, error) {
	if r == nil {
		return nil, fmt.Errorf("adk runner is required")
	}

	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user id is required")
	}

	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session id is required")
	}

	if message == nil {
		return nil, fmt.Errorf("message is required")
	}

	return &ADKStepSource{
		Runner:     r,
		UserID:     userID,
		SessionID:  sessionID,
		Message:    message,
		RunConfig:  runConfig,
		RunOptions: runOptions,
	}, nil
}

// AddInternalInstruction lets the deterministic harness continue an ADK run
// without exposing runtime control text as normal user conversation history.
func (s *ADKStepSource) AddInternalInstruction(instruction string) {
	if s == nil {
		return
	}

	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		return
	}

	s.pendingInstructions = append(s.pendingInstructions, instruction)
}

// NextSteps returns the next ADK-derived step batch without exposing ADK events to core runtime.
func (s *ADKStepSource) NextSteps(ctx context.Context, state coreruntime.State) ([]coreruntime.Step, error) {
	if s == nil || s.Runner == nil {
		return nil, fmt.Errorf("adk step source is not initialized")
	}

	if !s.started {
		message := s.Message
		if len(s.pendingInstructions) > 0 {
			message = s.messageWithInternalInstructions(s.Message)
		}

		if err := s.runAndQueueSteps(ctx, message); err != nil {
			return nil, err
		}

		s.started = true
	}

	for {
		if len(s.queue) > 0 {
			step := s.queue[0]
			s.queue = s.queue[1:]
			return []coreruntime.Step{step}, nil
		}

		if len(s.pendingInstructions) == 0 {
			return []coreruntime.Step{
				{
					Kind:        coreruntime.StepKindStreamDone,
					Source:      coreruntime.StepSourceRuntime,
					Status:      coreruntime.StepStatusCompleted,
					Final:       true,
					Description: "ADK stream completed",
				},
			}, nil
		}

		message := s.nextInternalInstructionMessage()
		if err := s.runAndQueueSteps(ctx, message); err != nil {
			return nil, err
		}
	}
}

func (s *ADKStepSource) runAndQueueSteps(ctx context.Context, message *genai.Content) error {
	if message == nil {
		return fmt.Errorf("adk step source message is required")
	}

	for event, err := range s.Runner.Run(
		ctx,
		s.UserID,
		s.SessionID,
		message,
		s.RunConfig,
		s.RunOptions...,
	) {
		if err != nil {
			return err
		}

		steps := StepsFromADKEvent(event)
		if len(steps) == 0 {
			continue
		}

		s.queue = append(s.queue, steps...)
	}

	return nil
}

func (s *ADKStepSource) nextInternalInstructionMessage() *genai.Content {
	return s.messageWithInternalInstructions(nil)
}

func (s *ADKStepSource) messageWithInternalInstructions(base *genai.Content) *genai.Content {
	instructions := s.pendingInstructions
	s.pendingInstructions = nil

	parts := make([]*genai.Part, 0)

	if base != nil {
		for _, part := range base.Parts {
			if part == nil {
				continue
			}

			parts = append(parts, part)
		}
	}

	if len(instructions) > 0 {
		parts = append(parts, &genai.Part{
			Text: runtimeHarnessInstructionText(instructions),
		})
	}

	return &genai.Content{
		Role:  genai.RoleUser,
		Parts: parts,
	}
}

func runtimeHarnessInstructionText(instructions []string) string {
	var builder strings.Builder

	builder.WriteString("<runtime_harness_instruction>\n")
	for _, instruction := range instructions {
		instruction = strings.TrimSpace(instruction)
		if instruction == "" {
			continue
		}

		builder.WriteString(instruction)
		builder.WriteString("\n")
	}
	builder.WriteString("</runtime_harness_instruction>")

	return builder.String()
}
