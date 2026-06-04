package adksession

import (
	"context"
	"iter"
	"testing"
	"time"

	corellm "github.com/bogachenko/agentkit/core/llm"
	coresession "github.com/bogachenko/agentkit/core/session"
	"google.golang.org/adk/model"
	adksdk "google.golang.org/adk/session"
	"google.golang.org/genai"
)

type fakeADKState struct {
	values map[string]any
}

func (s *fakeADKState) Get(key string) (any, error) {
	value, exists := s.values[key]
	if !exists {
		return nil, adksdk.ErrStateKeyNotExist
	}

	return value, nil
}

func (s *fakeADKState) Set(key string, value any) error {
	s.values[key] = value
	return nil
}

func (s *fakeADKState) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for key, value := range s.values {
			if !yield(key, value) {
				return
			}
		}
	}
}

type fakeADKEvents struct {
	items []*adksdk.Event
}

func (e fakeADKEvents) All() iter.Seq[*adksdk.Event] {
	return func(yield func(*adksdk.Event) bool) {
		for _, item := range e.items {
			if !yield(item) {
				return
			}
		}
	}
}

func (e fakeADKEvents) Len() int {
	return len(e.items)
}

func (e fakeADKEvents) At(index int) *adksdk.Event {
	return e.items[index]
}

type fakeADKSession struct {
	id     string
	state  *fakeADKState
	events fakeADKEvents
}

func (s fakeADKSession) ID() string {
	return s.id
}

func (s fakeADKSession) AppName() string {
	return "app"
}

func (s fakeADKSession) UserID() string {
	return "user"
}

func (s fakeADKSession) State() adksdk.State {
	return s.state
}

func (s fakeADKSession) Events() adksdk.Events {
	return s.events
}

func (s fakeADKSession) LastUpdateTime() time.Time {
	return time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
}

type fakeCompactor struct {
	input coresession.CompactionInput
}

func (c *fakeCompactor) Compact(ctx context.Context, input coresession.CompactionInput) (coresession.CompactionResult, error) {
	c.input = input

	return coresession.CompactionResult{
		Changed:            true,
		Summary:            "summary",
		Messages:           input.Messages,
		LastCompactedIndex: 0,
	}, nil
}

type fakeTitleGenerator struct {
	input coresession.TitleInput
}

func (g *fakeTitleGenerator) GenerateTitle(ctx context.Context, input coresession.TitleInput) (string, error) {
	g.input = input
	return "title", nil
}

func finalADKEvent(text string) *adksdk.Event {
	return &adksdk.Event{
		LLMResponse: model.LLMResponse{
			Content: genai.NewContentFromText(text, genai.RoleModel),
		},
		Timestamp: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
	}
}

func userADKEvent(text string) *adksdk.Event {
	return &adksdk.Event{
		LLMResponse: model.LLMResponse{
			Content: genai.NewContentFromText(text, genai.RoleUser),
		},
		Timestamp: time.Date(2026, 1, 1, 9, 59, 0, 0, time.UTC),
	}
}

func TestSummaryHookAddsStateDelta(t *testing.T) {
	compactor := &fakeCompactor{}

	hook, err := NewSummaryHook(SummaryHookConfig{
		Compactor:      compactor,
		MinMessages:    2,
		RecentMessages: 4,
	})
	if err != nil {
		t.Fatalf("create summary hook: %v", err)
	}

	sess := fakeADKSession{
		id: "session-1",
		state: &fakeADKState{
			values: map[string]any{
				DefaultSummaryStateKey: "previous",
			},
		},
		events: fakeADKEvents{
			items: []*adksdk.Event{
				userADKEvent("hello"),
			},
		},
	}

	event := finalADKEvent("answer")

	if err := hook.BeforeAppendEvent(context.Background(), sess, event); err != nil {
		t.Fatalf("before append event: %v", err)
	}

	if got := event.Actions.StateDelta[DefaultSummaryStateKey]; got != "summary" {
		t.Fatalf("expected summary state delta, got %#v", got)
	}

	if compactor.input.PreviousSummary != "previous" {
		t.Fatalf("expected previous summary, got %q", compactor.input.PreviousSummary)
	}

	if len(compactor.input.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(compactor.input.Messages))
	}
}

func TestTitleHookAddsStateDeltaWhenTitleMissing(t *testing.T) {
	generator := &fakeTitleGenerator{}

	hook, err := NewTitleHook(TitleHookConfig{
		TitleGenerator: generator,
		MinMessages:    2,
		RecentMessages: 4,
	})
	if err != nil {
		t.Fatalf("create title hook: %v", err)
	}

	sess := fakeADKSession{
		id: "session-1",
		state: &fakeADKState{
			values: map[string]any{},
		},
		events: fakeADKEvents{
			items: []*adksdk.Event{
				userADKEvent("hello"),
			},
		},
	}

	event := finalADKEvent("answer")

	if err := hook.BeforeAppendEvent(context.Background(), sess, event); err != nil {
		t.Fatalf("before append event: %v", err)
	}

	if got := event.Actions.StateDelta[DefaultTitleStateKey]; got != "title" {
		t.Fatalf("expected title state delta, got %#v", got)
	}

	if len(generator.input.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(generator.input.Messages))
	}
}

func TestTitleHookPreservesExistingTitle(t *testing.T) {
	generator := &fakeTitleGenerator{}

	hook, err := NewTitleHook(TitleHookConfig{
		TitleGenerator: generator,
		MinMessages:    2,
		RecentMessages: 4,
	})
	if err != nil {
		t.Fatalf("create title hook: %v", err)
	}

	sess := fakeADKSession{
		id: "session-1",
		state: &fakeADKState{
			values: map[string]any{
				DefaultTitleStateKey: "existing",
			},
		},
		events: fakeADKEvents{
			items: []*adksdk.Event{
				userADKEvent("hello"),
			},
		},
	}

	event := finalADKEvent("answer")

	if err := hook.BeforeAppendEvent(context.Background(), sess, event); err != nil {
		t.Fatalf("before append event: %v", err)
	}

	if event.Actions.StateDelta != nil {
		if _, exists := event.Actions.StateDelta[DefaultTitleStateKey]; exists {
			t.Fatal("expected title hook to preserve existing title")
		}
	}
}

func TestNewSummaryHookRejectsNilCompactor(t *testing.T) {
	_, err := NewSummaryHook(SummaryHookConfig{})

	if err == nil {
		t.Fatal("expected error for nil compactor")
	}
}

func TestNewTitleHookRejectsNilGenerator(t *testing.T) {
	_, err := NewTitleHook(TitleHookConfig{})

	if err == nil {
		t.Fatal("expected error for nil title generator")
	}
}

func TestFakeSessionMessageValidation(t *testing.T) {
	message := corellm.NewMessage(corellm.RoleUser, corellm.TextPart("hello"))

	if err := message.Validate(); err != nil {
		t.Fatalf("expected valid message, got error: %v", err)
	}
}
