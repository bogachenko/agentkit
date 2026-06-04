package session

import (
	"testing"
	"time"

	"github.com/bogachenko/agentkit/core/llm"
)

func sessionTestTime() time.Time {
	return time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
}

func sessionTestMessage() llm.Message {
	return llm.NewMessage(llm.RoleUser, llm.TextPart("hello"))
}

func TestSessionValidateAcceptsValidSession(t *testing.T) {
	createdAt := sessionTestTime()

	value := Session{
		ID:        ID("session-1"),
		State:     map[string]any{},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}

	if err := value.Validate(); err != nil {
		t.Fatalf("expected valid session, got error: %v", err)
	}
}

func TestSessionValidateRejectsNilState(t *testing.T) {
	createdAt := sessionTestTime()

	value := Session{
		ID:        ID("session-1"),
		State:     nil,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}

	if err := value.Validate(); err == nil {
		t.Fatal("expected error for nil session state")
	}
}

func TestSessionValidateRejectsInvalidTimeOrder(t *testing.T) {
	createdAt := sessionTestTime()

	value := Session{
		ID:        ID("session-1"),
		State:     map[string]any{},
		CreatedAt: createdAt,
		UpdatedAt: createdAt.Add(-time.Second),
	}

	if err := value.Validate(); err == nil {
		t.Fatal("expected error for invalid session time order")
	}
}

func TestStateDeltaApplyToReturnsNewState(t *testing.T) {
	original := map[string]any{
		"title": "old",
	}

	delta := StateDelta{
		"title": "new",
	}

	next, err := delta.ApplyTo(original)
	if err != nil {
		t.Fatalf("expected delta apply to succeed, got error: %v", err)
	}

	if next["title"] != "new" {
		t.Fatalf("expected new title, got %v", next["title"])
	}

	original["title"] = "changed-after-apply"

	if next["title"] != "new" {
		t.Fatalf("expected next state to be independent copy, got %v", next["title"])
	}
}

func TestStateDeltaValidateRejectsEmptyKey(t *testing.T) {
	delta := StateDelta{
		" ": "value",
	}

	if err := delta.Validate(); err == nil {
		t.Fatal("expected error for empty state delta key")
	}
}

func TestMessageEventValidateAcceptsValidMessageEvent(t *testing.T) {
	event := NewMessageEvent(
		EventID("event-1"),
		ID("session-1"),
		sessionTestMessage(),
		sessionTestTime(),
	)

	if err := event.Validate(); err != nil {
		t.Fatalf("expected valid message event, got error: %v", err)
	}
}

func TestMessageEventValidateRejectsStateDelta(t *testing.T) {
	event := NewMessageEvent(
		EventID("event-1"),
		ID("session-1"),
		sessionTestMessage(),
		sessionTestTime(),
	)
	event.StateDelta = StateDelta{
		"title": "new",
	}

	if err := event.Validate(); err == nil {
		t.Fatal("expected error for message event with state delta")
	}
}

func TestStateDeltaEventValidateAcceptsValidStateDeltaEvent(t *testing.T) {
	event := NewStateDeltaEvent(
		EventID("event-1"),
		ID("session-1"),
		StateDelta{
			"title": "new",
		},
		sessionTestTime(),
	)

	if err := event.Validate(); err != nil {
		t.Fatalf("expected valid state delta event, got error: %v", err)
	}
}

func TestStateDeltaEventValidateRejectsMessagePayload(t *testing.T) {
	event := NewStateDeltaEvent(
		EventID("event-1"),
		ID("session-1"),
		StateDelta{
			"title": "new",
		},
		sessionTestTime(),
	)
	message := sessionTestMessage()
	event.Message = &message

	if err := event.Validate(); err == nil {
		t.Fatal("expected error for state delta event with message")
	}
}

func TestStateDeltaEventValidateRejectsEmptyDelta(t *testing.T) {
	event := NewStateDeltaEvent(
		EventID("event-1"),
		ID("session-1"),
		StateDelta{},
		sessionTestTime(),
	)

	if err := event.Validate(); err == nil {
		t.Fatal("expected error for empty state delta")
	}
}

func TestCompactionInputValidateRejectsInvalidMessage(t *testing.T) {
	input := CompactionInput{
		Messages: []llm.Message{
			{
				Role: llm.RoleUser,
			},
		},
	}

	if err := input.Validate(); err == nil {
		t.Fatal("expected error for invalid compaction message")
	}
}

func TestCompactionResultValidateRejectsNegativeIndex(t *testing.T) {
	result := CompactionResult{
		LastCompactedIndex: -1,
	}

	if err := result.Validate(); err == nil {
		t.Fatal("expected error for negative compacted index")
	}
}

func TestTitleInputValidateRequiresMessages(t *testing.T) {
	input := TitleInput{}

	if err := input.Validate(); err == nil {
		t.Fatal("expected error for empty title input")
	}
}

func TestWorkSummaryInputValidateRequiresMessages(t *testing.T) {
	input := WorkSummaryInput{}

	if err := input.Validate(); err == nil {
		t.Fatal("expected error for empty work summary input")
	}
}
