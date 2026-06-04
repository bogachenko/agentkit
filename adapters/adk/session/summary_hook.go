package adksession

import (
	"context"
	"errors"
	"fmt"
	"strings"

	adkllm "github.com/bogachenko/agentkit/adapters/adk/llm"
	"github.com/bogachenko/agentkit/core/port"
	coresession "github.com/bogachenko/agentkit/core/session"
	adksdk "google.golang.org/adk/session"
)

const DefaultSummaryStateKey = "session_summary"

// SummaryHookConfig makes compaction thresholds and storage keys explicit.
type SummaryHookConfig struct {
	Compactor      port.Compactor
	StateKey       string
	MinMessages    int
	RecentMessages int
}

// SummaryHook attaches compact session summary state to final ADK response events.
type SummaryHook struct {
	compactor      port.Compactor
	stateKey       string
	minMessages    int
	recentMessages int
}

// NewSummaryHook validates probabilistic compaction dependency before hook registration.
func NewSummaryHook(cfg SummaryHookConfig) (*SummaryHook, error) {
	if cfg.Compactor == nil {
		return nil, errors.New("compactor is required")
	}

	stateKey := strings.TrimSpace(cfg.StateKey)
	if stateKey == "" {
		stateKey = DefaultSummaryStateKey
	}

	minMessages := cfg.MinMessages
	if minMessages <= 0 {
		minMessages = 2
	}

	recentMessages := cfg.RecentMessages
	if recentMessages <= 0 {
		recentMessages = 12
	}

	return &SummaryHook{
		compactor:      cfg.Compactor,
		stateKey:       stateKey,
		minMessages:    minMessages,
		recentMessages: recentMessages,
	}, nil
}

// BeforeAppendEvent updates summary only when ADK produced a final response event.
func (h *SummaryHook) BeforeAppendEvent(ctx context.Context, sess adksdk.Session, event *adksdk.Event) error {
	if h == nil || event == nil || !event.IsFinalResponse() {
		return nil
	}

	return h.attachSummaryStateDelta(ctx, sess, event)
}

// attachSummaryStateDelta writes summary through ADK StateDelta without mutating storage directly.
func (h *SummaryHook) attachSummaryStateDelta(ctx context.Context, sess adksdk.Session, event *adksdk.Event) error {
	if sess == nil {
		return nil
	}

	if event.Actions.StateDelta == nil {
		event.Actions.StateDelta = map[string]any{}
	}

	if existing, ok := event.Actions.StateDelta[h.stateKey]; ok && strings.TrimSpace(fmt.Sprintf("%v", existing)) != "" {
		return nil
	}

	previousSummary, _, err := NewStateReader(sess.State()).GetString(h.stateKey)
	if err != nil {
		return fmt.Errorf("read previous summary state: %w", err)
	}

	messages := adkllm.EventsToRecentCoreMessages(sess.Events(), h.recentMessages-1)

	currentMessage, ok := adkllm.EventToCoreMessage(event)
	if ok {
		messages = append(messages, currentMessage)
	}

	if len(messages) < h.minMessages {
		return nil
	}

	result, err := h.compactor.Compact(ctx, coresession.CompactionInput{
		PreviousSummary: previousSummary,
		Messages:        messages,
	})
	if err != nil {
		return fmt.Errorf("compact session summary: %w", err)
	}

	if err := result.Validate(); err != nil {
		return fmt.Errorf("validate compaction result: %w", err)
	}

	summary := strings.TrimSpace(result.Summary)
	if summary == "" {
		return nil
	}

	event.Actions.StateDelta[h.stateKey] = summary
	return nil
}
