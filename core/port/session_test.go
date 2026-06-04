package port

import (
	"context"
	"testing"

	"github.com/bogachenko/agentkit/core/session"
)

type fakeSessionStore struct{}

func (fakeSessionStore) CreateSession(ctx context.Context, value session.Session) error {
	return nil
}

func (fakeSessionStore) GetSession(ctx context.Context, id session.ID) (session.Session, error) {
	return session.Session{ID: id, State: map[string]any{}}, nil
}

func (fakeSessionStore) AppendEvent(ctx context.Context, event session.Event) error {
	return nil
}

func (fakeSessionStore) ListEvents(ctx context.Context, sessionID session.ID, limit int) ([]session.Event, error) {
	return nil, nil
}

type fakeCompactor struct{}

func (fakeCompactor) Compact(ctx context.Context, input session.CompactionInput) (session.CompactionResult, error) {
	return session.CompactionResult{}, nil
}

type fakeTitleGenerator struct{}

func (fakeTitleGenerator) GenerateTitle(ctx context.Context, input session.TitleInput) (string, error) {
	return "title", nil
}

type fakeWorkSummaryGenerator struct{}

func (fakeWorkSummaryGenerator) GenerateWorkSummary(ctx context.Context, input session.WorkSummaryInput) (string, error) {
	return "summary", nil
}

func TestSessionPortInterfacesAcceptImplementations(t *testing.T) {
	var _ SessionStore = fakeSessionStore{}
	var _ Compactor = fakeCompactor{}
	var _ TitleGenerator = fakeTitleGenerator{}
	var _ WorkSummaryGenerator = fakeWorkSummaryGenerator{}
}
