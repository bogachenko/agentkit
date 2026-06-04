package port

import (
	"context"

	"github.com/bogachenko/agentkit/core/session"
)

// SessionStore isolates durable session persistence from runtime, adapters, and transport code.
type SessionStore interface {
	CreateSession(ctx context.Context, value session.Session) error
	GetSession(ctx context.Context, id session.ID) (session.Session, error)
	AppendEvent(ctx context.Context, event session.Event) error
	ListEvents(ctx context.Context, sessionID session.ID, limit int) ([]session.Event, error)
}
