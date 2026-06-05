package port

import (
	"context"
	"testing"
	"time"

	"github.com/bogachenko/agentkit/core/llm"
)

type fakeModel struct{}

func (fakeModel) Generate(ctx context.Context, request llm.Request) (llm.Message, error) {
	return llm.NewMessage(llm.RoleAssistant, llm.TextPart("ok")), nil
}

type fakePublisher struct{}

func (fakePublisher) Publish(ctx context.Context, event Event) error {
	return nil
}

type fakeClock struct{}

func (fakeClock) Now() time.Time {
	return time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
}

type fakeIDGenerator struct{}

func (fakeIDGenerator) NewID() string {
	return "id-1"
}

type fakeLogger struct{}

func (fakeLogger) Printf(format string, args ...any) {}

type fakeTracer struct{}

func (fakeTracer) Start(ctx context.Context, name string, attrs map[string]any) (context.Context, Span) {
	return ctx, fakeSpan{}
}

type fakeSpan struct{}

func (fakeSpan) SetAttributes(attrs map[string]any)         {}
func (fakeSpan) AddEvent(name string, attrs map[string]any) {}

func (fakeSpan) RecordError(err error) {}

func (fakeSpan) End() {}

func TestPortInterfacesAcceptImplementations(t *testing.T) {
	var _ Model = fakeModel{}
	var _ Publisher = fakePublisher{}
	var _ Clock = fakeClock{}
	var _ IDGenerator = fakeIDGenerator{}
	var _ Logger = fakeLogger{}
	var _ Tracer = fakeTracer{}
	var _ Span = fakeSpan{}
}

func TestModelInterfaceUsesNeutralLLMTypes(t *testing.T) {
	model := fakeModel{}

	message, err := model.Generate(context.Background(), llm.Request{
		System: "system instruction",
		Messages: []llm.Message{
			llm.NewMessage(llm.RoleUser, llm.TextPart("hello")),
		},
	})
	if err != nil {
		t.Fatalf("expected model call to succeed, got error: %v", err)
	}

	if err := message.Validate(); err != nil {
		t.Fatalf("expected valid message, got error: %v", err)
	}
}

func TestPublisherInterfaceAcceptsTransportNeutralEvent(t *testing.T) {
	publisher := fakePublisher{}

	err := publisher.Publish(context.Background(), Event{
		Type:      EventTypeStarted,
		RunID:     "run-1",
		SessionID: "session-1",
		Payload: map[string]any{
			"status": "started",
		},
		CreatedAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("expected publish to succeed, got error: %v", err)
	}
}
