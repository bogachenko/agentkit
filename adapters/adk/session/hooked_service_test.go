package adksession

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/adk/model"
	adksdk "google.golang.org/adk/session"
	"google.golang.org/genai"
)

type testLogger struct {
	messages []string
}

func (l *testLogger) Printf(format string, args ...any) {
	l.messages = append(l.messages, format)
}

type testHook struct {
	calls int
	err   error
}

func (h *testHook) BeforeAppendEvent(ctx context.Context, sess adksdk.Session, event *adksdk.Event) error {
	h.calls++
	return h.err
}

func TestNewHookedSessionServiceRejectsNilBase(t *testing.T) {
	_, err := NewHookedSessionService(HookedSessionServiceConfig{})

	if err == nil {
		t.Fatal("expected error for nil base service")
	}
}

func TestHookedSessionServiceAppendEventRunsHooksAndPersists(t *testing.T) {
	base := adksdk.InMemoryService()

	created, err := base.Create(context.Background(), &adksdk.CreateRequest{
		AppName:   "app",
		UserID:    "user",
		SessionID: "session-1",
		State:     map[string]any{},
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	hook := &testHook{}

	service, err := NewHookedSessionService(HookedSessionServiceConfig{
		Base:  base,
		Hooks: []EventHook{hook},
	})
	if err != nil {
		t.Fatalf("create hooked service: %v", err)
	}

	event := &adksdk.Event{
		LLMResponse: model.LLMResponse{
			Content: genai.NewContentFromText("ok", genai.RoleModel),
		},
		Timestamp: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
	}

	if err := service.AppendEvent(context.Background(), created.Session, event); err != nil {
		t.Fatalf("append event: %v", err)
	}

	if hook.calls != 1 {
		t.Fatalf("expected hook to be called once, got %d", hook.calls)
	}
}

func TestHookedSessionServiceLogsHookErrorsAndStillPersists(t *testing.T) {
	base := adksdk.InMemoryService()

	created, err := base.Create(context.Background(), &adksdk.CreateRequest{
		AppName:   "app",
		UserID:    "user",
		SessionID: "session-1",
		State:     map[string]any{},
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	logger := &testLogger{}
	hook := &testHook{
		err: errors.New("hook failed"),
	}

	service, err := NewHookedSessionService(HookedSessionServiceConfig{
		Base:   base,
		Logger: logger,
		Hooks:  []EventHook{hook},
	})
	if err != nil {
		t.Fatalf("create hooked service: %v", err)
	}

	event := &adksdk.Event{
		LLMResponse: model.LLMResponse{
			Content: genai.NewContentFromText("ok", genai.RoleModel),
		},
		Timestamp: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
	}

	if err := service.AppendEvent(context.Background(), created.Session, event); err != nil {
		t.Fatalf("append event: %v", err)
	}

	if hook.calls != 1 {
		t.Fatalf("expected hook to be called once, got %d", hook.calls)
	}

	if len(logger.messages) != 1 {
		t.Fatalf("expected one log message, got %d", len(logger.messages))
	}
}
