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

const DefaultTitleStateKey = "session_title"

// TitleHookConfig makes title generation thresholds and storage keys explicit.
type TitleHookConfig struct {
	TitleGenerator port.TitleGenerator
	StateKey       string
	MinMessages    int
	RecentMessages int
}

// TitleHook attaches a generated title once, without putting naming logic into runtime.
type TitleHook struct {
	titleGenerator port.TitleGenerator
	stateKey       string
	minMessages    int
	recentMessages int
}

// NewTitleHook validates probabilistic title dependency before hook registration.
func NewTitleHook(cfg TitleHookConfig) (*TitleHook, error) {
	if cfg.TitleGenerator == nil {
		return nil, errors.New("title generator is required")
	}

	stateKey := strings.TrimSpace(cfg.StateKey)
	if stateKey == "" {
		stateKey = DefaultTitleStateKey
	}

	minMessages := cfg.MinMessages
	if minMessages <= 0 {
		minMessages = 2
	}

	recentMessages := cfg.RecentMessages
	if recentMessages <= 0 {
		recentMessages = 6
	}

	return &TitleHook{
		titleGenerator: cfg.TitleGenerator,
		stateKey:       stateKey,
		minMessages:    minMessages,
		recentMessages: recentMessages,
	}, nil
}

// BeforeAppendEvent updates title only when ADK produced a final response event.
func (h *TitleHook) BeforeAppendEvent(ctx context.Context, sess adksdk.Session, event *adksdk.Event) error {
	if h == nil || event == nil || !event.IsFinalResponse() {
		return nil
	}

	return h.attachTitleStateDelta(ctx, sess, event)
}

// attachTitleStateDelta writes title through ADK StateDelta and preserves existing titles.
func (h *TitleHook) attachTitleStateDelta(ctx context.Context, sess adksdk.Session, event *adksdk.Event) error {
	if sess == nil {
		return nil
	}

	if event.Actions.StateDelta == nil {
		event.Actions.StateDelta = map[string]any{}
	}

	if existing, ok := event.Actions.StateDelta[h.stateKey]; ok && strings.TrimSpace(fmt.Sprintf("%v", existing)) != "" {
		return nil
	}

	if existingTitle, exists, err := NewStateReader(sess.State()).GetString(h.stateKey); err != nil {
		return fmt.Errorf("read existing title state: %w", err)
	} else if exists && strings.TrimSpace(existingTitle) != "" {
		return nil
	}

	messages := adkllm.EventsToRecentCoreMessages(sess.Events(), h.recentMessages-1)

	currentMessage, ok := adkllm.EventToCoreMessage(event)
	if ok {
		messages = append(messages, currentMessage)
	}

	if len(messages) < h.minMessages {
		return nil
	}

	title, err := h.titleGenerator.GenerateTitle(ctx, coresession.TitleInput{
		Messages: messages,
	})
	if err != nil {
		return fmt.Errorf("generate session title: %w", err)
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}

	event.Actions.StateDelta[h.stateKey] = title
	return nil
}
