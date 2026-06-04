package port

import (
	"context"
	"time"
)

// EventType gives external subscribers stable lifecycle event names without exposing runtime internals.
type EventType string

const (
	EventTypeStarted   EventType = "started"
	EventTypeStep      EventType = "step"
	EventTypeBlocked   EventType = "blocked"
	EventTypeCompleted EventType = "completed"
	EventTypeFailed    EventType = "failed"
)

// Event is the boundary between core execution and delivery transports such as HTTP, SSE, WebSocket, or queues.
type Event struct {
	Type      EventType
	RunID     string
	SessionID string
	Payload   map[string]any
	CreatedAt time.Time
}

// Publisher lets runtime emit progress without depending on a concrete transport.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}
