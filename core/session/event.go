package session

import (
	"fmt"
	"time"

	"github.com/bogachenko/agentkit/core/llm"
)

// EventKind separates durable session facts without embedding provider-specific event types.
type EventKind string

const (
	EventKindMessage    EventKind = "message"
	EventKindStateDelta EventKind = "state_delta"
)

// Validation prevents unknown event kinds from entering append-only session history.
func (k EventKind) Validate() error {
	switch k {
	case EventKindMessage, EventKindStateDelta:
		return nil
	default:
		return fmt.Errorf("unknown session event kind %q", string(k))
	}
}

// Event records one append-only session fact without mixing it with runtime ledger entries.
type Event struct {
	ID         EventID
	SessionID  ID
	Kind       EventKind
	Message    *llm.Message
	StateDelta StateDelta
	CreatedAt  time.Time
}

// NewMessageEvent records one provider-neutral conversation message in session history.
func NewMessageEvent(id EventID, sessionID ID, message llm.Message, createdAt time.Time) Event {
	return Event{
		ID:        id,
		SessionID: sessionID,
		Kind:      EventKindMessage,
		Message:   &message,
		CreatedAt: createdAt,
	}
}

// NewStateDeltaEvent records one explicit session state change without requiring a message.
func NewStateDeltaEvent(id EventID, sessionID ID, delta StateDelta, createdAt time.Time) Event {
	return Event{
		ID:         id,
		SessionID:  sessionID,
		Kind:       EventKindStateDelta,
		StateDelta: delta,
		CreatedAt:  createdAt,
	}
}

// Validation keeps every session event structurally consistent with its declared kind.
func (e Event) Validate() error {
	if err := e.ID.Validate(); err != nil {
		return err
	}

	if err := e.SessionID.Validate(); err != nil {
		return err
	}

	if err := e.Kind.Validate(); err != nil {
		return err
	}

	if e.CreatedAt.IsZero() {
		return fmt.Errorf("session event %q created_at is required", string(e.ID))
	}

	switch e.Kind {
	case EventKindMessage:
		if e.Message == nil {
			return fmt.Errorf("session event %q requires message payload", string(e.ID))
		}

		if err := e.Message.Validate(); err != nil {
			return fmt.Errorf("session event %q message: %w", string(e.ID), err)
		}

		if len(e.StateDelta) > 0 {
			return fmt.Errorf("message session event %q cannot include state delta", string(e.ID))
		}

	case EventKindStateDelta:
		if e.Message != nil {
			return fmt.Errorf("state_delta session event %q cannot include message", string(e.ID))
		}

		if len(e.StateDelta) == 0 {
			return fmt.Errorf("state_delta session event %q requires state delta", string(e.ID))
		}

		if err := e.StateDelta.Validate(); err != nil {
			return fmt.Errorf("session event %q state delta: %w", string(e.ID), err)
		}
	}

	return nil
}
