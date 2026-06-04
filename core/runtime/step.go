package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/bogachenko/agentkit/core/tool"
)

// StepKind separates ADK/tool lifecycle facts from generic runtime progress.
type StepKind string

const (
	StepKindGeneric       StepKind = "generic"
	StepKindToolCall      StepKind = "tool_call"
	StepKindToolResult    StepKind = "tool_result"
	StepKindAssistantText StepKind = "assistant_text"
	StepKindEmpty         StepKind = "empty"
	StepKindStreamDone    StepKind = "stream_done"
)

// Validate prevents unknown step kinds from entering state, ledger, or API events.
func (k StepKind) Validate() error {
	switch k {
	case "", StepKindGeneric, StepKindToolCall, StepKindToolResult, StepKindAssistantText, StepKindEmpty, StepKindStreamDone:
		return nil
	default:
		return fmt.Errorf("unknown step kind %q", string(k))
	}
}

// ToolErrorKind keeps tool-result failure class machine-readable without coupling runtime to a provider.
type ToolErrorKind string

const (
	ToolErrorNone       ToolErrorKind = ""
	ToolErrorValidation ToolErrorKind = "validation"
	ToolErrorAuth       ToolErrorKind = "auth"
	ToolErrorClientHold ToolErrorKind = "client_hold"
	ToolErrorFatal      ToolErrorKind = "fatal"
)

// Validate rejects arbitrary tool error classes before they affect runtime state.
func (k ToolErrorKind) Validate() error {
	switch k {
	case ToolErrorNone, ToolErrorValidation, ToolErrorAuth, ToolErrorClientHold, ToolErrorFatal:
		return nil
	default:
		return fmt.Errorf("unknown tool error kind %q", string(k))
	}
}

// ToolExecutionResult records one tool response as data, not as runtime control flow.
type ToolExecutionResult struct {
	OK           bool
	HasEvidence  bool
	ErrorKind    ToolErrorKind
	ErrorMessage string
	Raw          any
}

// Validate keeps failed tool results explicit and explainable.
func (r ToolExecutionResult) Validate() error {
	if err := r.ErrorKind.Validate(); err != nil {
		return err
	}

	if r.OK {
		if r.ErrorKind != ToolErrorNone {
			return fmt.Errorf("successful tool result cannot include error kind %q", string(r.ErrorKind))
		}

		if strings.TrimSpace(r.ErrorMessage) != "" {
			return fmt.Errorf("successful tool result cannot include error message")
		}

		return nil
	}

	if r.ErrorKind == ToolErrorNone {
		return fmt.Errorf("failed tool result requires error kind")
	}

	if strings.TrimSpace(r.ErrorMessage) == "" {
		return fmt.Errorf("failed tool result requires error message")
	}

	return nil
}

// Step records one explicit runtime fact without mixing it with LLM message history.
type Step struct {
	ID          StepID
	Kind        StepKind
	Source      StepSource
	Status      StepStatus
	Description string

	ToolCallID string
	ToolName   tool.Name
	ToolArgs   map[string]any
	ToolResult ToolExecutionResult

	Text  string
	Final bool

	Failure    *Failure
	StartedAt  time.Time
	FinishedAt time.Time
}

// Validate keeps runtime steps auditable and structurally safe for ledger storage.
func (s Step) Validate() error {
	if err := s.ID.Validate(); err != nil {
		return err
	}

	if err := s.Kind.Validate(); err != nil {
		return err
	}

	if err := s.Source.Validate(); err != nil {
		return err
	}

	if err := s.Status.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(s.Description) == "" {
		return fmt.Errorf("step description is required for %q", string(s.ID))
	}

	switch s.Kind {
	case StepKindToolCall:
		if err := s.ToolName.Validate(); err != nil {
			return fmt.Errorf("tool call step tool name: %w", err)
		}

	case StepKindToolResult:
		if err := s.ToolName.Validate(); err != nil {
			return fmt.Errorf("tool result step tool name: %w", err)
		}

		if err := s.ToolResult.Validate(); err != nil {
			return fmt.Errorf("tool result step: %w", err)
		}

	case StepKindAssistantText:
		if strings.TrimSpace(s.Text) == "" {
			return fmt.Errorf("assistant text step requires text")
		}
	}

	switch s.Status {
	case StepStatusFailed, StepStatusBlocked:
		if s.Failure == nil {
			return fmt.Errorf("%s step %q requires failure", s.Status, string(s.ID))
		}

		if err := s.Failure.Validate(); err != nil {
			return fmt.Errorf("step %q failure: %w", string(s.ID), err)
		}

	default:
		if s.Failure != nil {
			return fmt.Errorf("%s step %q cannot include failure", s.Status, string(s.ID))
		}
	}

	if !s.StartedAt.IsZero() && !s.FinishedAt.IsZero() && s.FinishedAt.Before(s.StartedAt) {
		return fmt.Errorf("step %q finished before it started", string(s.ID))
	}

	return nil
}
