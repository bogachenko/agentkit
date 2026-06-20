package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/bogachenko/agentkit/core/port"
	coresession "github.com/bogachenko/agentkit/core/session"
)

type SemanticPublisherAdapter struct {
	Publisher port.Publisher
	RunID     RunID
	SessionID coresession.ID
	Clock     port.Clock
}

func NewSemanticPublisherAdapter(publisher port.Publisher, clock port.Clock, runID RunID, sessionID coresession.ID) SemanticPublisherAdapter {
	return SemanticPublisherAdapter{
		Publisher: publisher,
		RunID:     runID,
		SessionID: sessionID,
		Clock:     clock,
	}
}

func (p SemanticPublisherAdapter) PublishFinal(ctx context.Context, message string) error {
	return p.publishSemanticEvent(ctx, port.EventTypeCompleted, message, nil)
}

func (p SemanticPublisherAdapter) PublishBlocked(ctx context.Context, message string) error {
	return p.publishSemanticEvent(ctx, port.EventTypeBlocked, message, map[string]any{
		"blocked_message": message,
	})
}

func (p SemanticPublisherAdapter) PublishFailure(ctx context.Context, message string) error {
	return p.publishSemanticEvent(ctx, port.EventTypeFailed, message, map[string]any{
		"failure_message": message,
	})
}

func (p SemanticPublisherAdapter) publishSemanticEvent(ctx context.Context, eventType port.EventType, message string, extra map[string]any) error {
	if p.Publisher == nil {
		return fmt.Errorf("semantic publisher adapter publisher is required")
	}
	if err := p.RunID.Validate(); err != nil {
		return fmt.Errorf("semantic publisher adapter run id is required: %w", err)
	}
	if err := p.SessionID.Validate(); err != nil {
		return fmt.Errorf("semantic publisher adapter session id is required: %w", err)
	}
	if p.Clock == nil {
		return fmt.Errorf("semantic publisher adapter clock is required")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("semantic publisher adapter message is required")
	}

	payload := map[string]any{
		"message":  message,
		"semantic": true,
	}
	for key, value := range extra {
		payload[key] = value
	}

	return p.Publisher.Publish(ctx, port.Event{
		Type:      eventType,
		RunID:     string(p.RunID),
		SessionID: string(p.SessionID),
		Payload:   payload,
		CreatedAt: p.Clock.Now(),
	})
}
