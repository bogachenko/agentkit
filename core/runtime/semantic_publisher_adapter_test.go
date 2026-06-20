package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	coresession "github.com/bogachenko/agentkit/core/session"
	"github.com/bogachenko/agentkit/core/port"
)

type capturingPortPublisher struct {
	events []port.Event
	err    error
}

func (p *capturingPortPublisher) Publish(_ context.Context, event port.Event) error {
	if p.err != nil {
		return p.err
	}
	p.events = append(p.events, event)
	return nil
}

func TestSemanticPublisherAdapterPublishFinal(t *testing.T) {
	publisher := &capturingPortPublisher{}
	clock := testClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)}
	adapter := NewSemanticPublisherAdapter(publisher, clock, RunID("run-final"), coresession.ID("session-final"))

	if err := adapter.PublishFinal(context.Background(), "ok"); err != nil {
		t.Fatal(err)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("events = %#v", publisher.events)
	}
	event := publisher.events[0]
	if event.Type != port.EventTypeCompleted {
		t.Fatalf("type = %q", event.Type)
	}
	if event.Payload["message"] != "ok" || event.Payload["semantic"] != true {
		t.Fatalf("payload = %#v", event.Payload)
	}
	if event.RunID != "run-final" || event.SessionID != "session-final" {
		t.Fatalf("identity = %s/%s", event.RunID, event.SessionID)
	}
	if !event.CreatedAt.Equal(clock.now) {
		t.Fatalf("created at = %s", event.CreatedAt)
	}
}

func TestSemanticPublisherAdapterPublishBlocked(t *testing.T) {
	publisher := &capturingPortPublisher{}
	adapter := validSemanticPublisherAdapter(publisher)

	if err := adapter.PublishBlocked(context.Background(), "need input"); err != nil {
		t.Fatal(err)
	}
	event := publisher.events[0]
	if event.Type != port.EventTypeBlocked {
		t.Fatalf("type = %q", event.Type)
	}
	if event.Payload["blocked_message"] != "need input" {
		t.Fatalf("payload = %#v", event.Payload)
	}
}

func TestSemanticPublisherAdapterPublishFailure(t *testing.T) {
	publisher := &capturingPortPublisher{}
	adapter := validSemanticPublisherAdapter(publisher)

	if err := adapter.PublishFailure(context.Background(), "failed"); err != nil {
		t.Fatal(err)
	}
	event := publisher.events[0]
	if event.Type != port.EventTypeFailed {
		t.Fatalf("type = %q", event.Type)
	}
	if event.Payload["failure_message"] != "failed" {
		t.Fatalf("payload = %#v", event.Payload)
	}
}

func TestSemanticPublisherAdapterRequiresDependencies(t *testing.T) {
	base := validSemanticPublisherAdapter(&capturingPortPublisher{})
	cases := []struct {
		name    string
		adapter SemanticPublisherAdapter
		message string
	}{
		{name: "publisher", adapter: SemanticPublisherAdapter{RunID: base.RunID, SessionID: base.SessionID, Clock: base.Clock}, message: "ok"},
		{name: "run_id", adapter: SemanticPublisherAdapter{Publisher: base.Publisher, SessionID: base.SessionID, Clock: base.Clock}, message: "ok"},
		{name: "session_id", adapter: SemanticPublisherAdapter{Publisher: base.Publisher, RunID: base.RunID, Clock: base.Clock}, message: "ok"},
		{name: "clock", adapter: SemanticPublisherAdapter{Publisher: base.Publisher, RunID: base.RunID, SessionID: base.SessionID}, message: "ok"},
		{name: "message", adapter: base, message: "  "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.adapter.PublishFinal(context.Background(), tc.message); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestSemanticPublisherAdapterPropagatesPublishError(t *testing.T) {
	publishErr := errors.New("publish failed")
	adapter := validSemanticPublisherAdapter(&capturingPortPublisher{err: publishErr})

	err := adapter.PublishFinal(context.Background(), "ok")
	if !errors.Is(err, publishErr) {
		t.Fatalf("error = %v", err)
	}
}

func TestSemanticHarnessWithPortPublisherReturnsCopy(t *testing.T) {
	publisher := &capturingPortPublisher{}
	clock := testClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)}
	harness := SemanticHarness{}

	next := harness.WithPortPublisher(publisher, clock, RunID("run-copy"), coresession.ID("session-copy"))
	if harness.Publisher != nil {
		t.Fatal("original harness was mutated")
	}
	if next.Publisher == nil {
		t.Fatal("next publisher is nil")
	}
}

func validSemanticPublisherAdapter(publisher port.Publisher) SemanticPublisherAdapter {
	return NewSemanticPublisherAdapter(
		publisher,
		testClock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		RunID("run-semantic-publisher"),
		coresession.ID("session-semantic-publisher"),
	)
}
