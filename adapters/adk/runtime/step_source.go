package runtime

import (
	"context"
	"fmt"
	"strings"
	"sync"

	coreruntime "github.com/bogachenko/agentkit/core/runtime"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/genai"
)

type stepBatch struct {
	steps []coreruntime.Step
	err   error
}

// ADKStepSource converts ADK runner events into neutral AgentKit runtime steps.
type ADKStepSource struct {
	Runner     *runner.Runner
	UserID     string
	SessionID  string
	Message    *genai.Content
	RunConfig  agent.RunConfig
	RunOptions []runner.RunOption

	once sync.Once
	ch   chan stepBatch
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

// NextSteps returns the next ADK-derived step batch without exposing ADK events to core runtime.
func (s *ADKStepSource) NextSteps(ctx context.Context, state coreruntime.State) ([]coreruntime.Step, error) {
	if s == nil {
		return nil, fmt.Errorf("adk step source is nil")
	}

	s.once.Do(func() {
		s.ch = make(chan stepBatch, 64)
		go s.run(ctx)
	})

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case batch, ok := <-s.ch:
			if !ok {
				return nil, coreruntime.ErrStepSourceDone
			}

			if batch.err != nil {
				return nil, batch.err
			}

			if len(batch.steps) == 0 {
				continue
			}

			return batch.steps, nil
		}
	}
}

func (s *ADKStepSource) run(ctx context.Context) {
	defer close(s.ch)

	for event, err := range s.Runner.Run(
		ctx,
		s.UserID,
		s.SessionID,
		s.Message,
		s.RunConfig,
		s.RunOptions...,
	) {
		if err != nil {
			s.ch <- stepBatch{err: err}
			return
		}

		steps := StepsFromADKEvent(event)
		if len(steps) == 0 {
			continue
		}

		select {
		case <-ctx.Done():
			s.ch <- stepBatch{err: ctx.Err()}
			return

		case s.ch <- stepBatch{steps: steps}:
		}
	}
}
