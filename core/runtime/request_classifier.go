package runtime

import (
	"context"
	"fmt"
	"strings"

	coresession "github.com/bogachenko/agentkit/core/session"
)

type RequestRoute string

const (
	RouteDirectAnswer      RequestRoute = "DIRECT_ANSWER"
	RouteExecuteTask       RequestRoute = "EXECUTE_TASK"
	RouteAnswerFromContext RequestRoute = "ANSWER_FROM_CONTEXT"
	RouteAskUser           RequestRoute = "ASK_USER"
	RouteRejectUnsupported RequestRoute = "REJECT_UNSUPPORTED"
)

type ToolCatalogItem struct {
	Name           string
	Description    string
	RequiredInputs []string
	Available      bool
}

type ClassifierInput struct {
	SessionID            coresession.ID
	UserPrompt           string
	ConversationContext  []string
	PendingUserInput     string
	ActiveTask           ActiveTaskState
	RunLedger            *RunLedger
	LedgerSummary        RunLedgerSummary
	Tools                []ToolCatalogItem
	Artifacts            []string
	CredentialsOrSources []string
	Skills               []string
	SessionConstraints   []string
}

func (i ClassifierInput) Validate() error {
	if strings.TrimSpace(i.UserPrompt) == "" {
		return fmt.Errorf("classifier input user prompt is required")
	}

	return nil
}

type ClassifierOutput struct {
	Route       RequestRoute `json:"route"`
	UserMessage string       `json:"user_message"`
}

func (o ClassifierOutput) Validate() error {
	if o.Route == "" {
		return fmt.Errorf("classifier output route is required")
	}

	hasMessage := strings.TrimSpace(o.UserMessage) != ""
	switch o.Route {
	case RouteDirectAnswer, RouteAskUser, RouteRejectUnsupported:
		if !hasMessage {
			return fmt.Errorf("route %s requires user message", o.Route)
		}
	case RouteExecuteTask, RouteAnswerFromContext:
		if hasMessage {
			return fmt.Errorf("route %s must not include user message", o.Route)
		}
	default:
		return fmt.Errorf("unsupported request route %q", string(o.Route))
	}

	return nil
}

type RequestClassifier interface {
	Classify(ctx context.Context, input ClassifierInput) (ClassifierOutput, error)
}
