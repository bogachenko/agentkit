package llm

import "testing"

func TestMessageValidateAcceptsValidUserTextMessage(t *testing.T) {
	msg := NewMessage(RoleUser, TextPart("hello"))

	if err := msg.Validate(); err != nil {
		t.Fatalf("expected valid message, got error: %v", err)
	}
}

func TestMessageValidateRejectsUnknownRole(t *testing.T) {
	msg := NewMessage(Role("manager"), TextPart("hello"))

	if err := msg.Validate(); err == nil {
		t.Fatal("expected error for unknown role")
	}
}

func TestMessageValidateRejectsEmptyParts(t *testing.T) {
	msg := Message{Role: RoleUser}

	if err := msg.Validate(); err == nil {
		t.Fatal("expected error for empty parts")
	}
}

func TestPartValidateRejectsEmptyText(t *testing.T) {
	part := TextPart("   ")

	if err := part.Validate(); err == nil {
		t.Fatal("expected error for empty text part")
	}
}

func TestPartValidateRejectsFunctionCallWithoutName(t *testing.T) {
	part := FunctionCallPart("", nil)

	if err := part.Validate(); err == nil {
		t.Fatal("expected error for unnamed function call")
	}
}

func TestFunctionCallPartNormalizesNilArgs(t *testing.T) {
	part := FunctionCallPart("search", nil)

	if part.Args == nil {
		t.Fatal("expected nil args to be normalized to empty map")
	}
}
