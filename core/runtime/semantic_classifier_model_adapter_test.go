package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bogachenko/agentkit/core/llm"
)

type fakePortModel struct {
	called  bool
	request llm.Request
	message llm.Message
	err     error
}

func (m *fakePortModel) Generate(_ context.Context, request llm.Request) (llm.Message, error) {
	m.called = true
	m.request = request
	if m.err != nil {
		return llm.Message{}, m.err
	}
	return m.message, nil
}

func TestSemanticClassifierModelAdapterCallsPortModel(t *testing.T) {
	model := &fakePortModel{message: llm.NewMessage(llm.RoleAssistant, llm.TextPart(`{"route":"ANSWER_FROM_CONTEXT","user_message":""}`))}
	messages := BuildSemanticClassifierPrompt(ClassifierInput{UserPrompt: "status"})

	message, err := (SemanticClassifierModelAdapter{Model: model}).GenerateSemanticClassifierOutput(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}
	if !model.called {
		t.Fatal("model was not called")
	}
	if len(model.request.Messages) != len(messages) {
		t.Fatalf("messages len = %d", len(model.request.Messages))
	}
	if !strings.Contains(model.request.System, "strict semantic request classifier") {
		t.Fatalf("system = %q", model.request.System)
	}
	if len(message.Parts) == 0 {
		t.Fatalf("message = %#v", message)
	}
}

func TestSemanticClassifierModelAdapterUsesCustomSystemPrompt(t *testing.T) {
	model := &fakePortModel{message: llm.NewMessage(llm.RoleAssistant, llm.TextPart(`{"route":"ANSWER_FROM_CONTEXT","user_message":""}`))}

	_, err := (SemanticClassifierModelAdapter{Model: model, SystemPrompt: "custom classifier system"}).GenerateSemanticClassifierOutput(
		context.Background(),
		BuildSemanticClassifierPrompt(ClassifierInput{UserPrompt: "status"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if model.request.System != "custom classifier system" {
		t.Fatalf("system = %q", model.request.System)
	}
}

func TestSemanticClassifierModelAdapterPassesRuntimeContext(t *testing.T) {
	model := &fakePortModel{message: llm.NewMessage(llm.RoleAssistant, llm.TextPart(`{"route":"ANSWER_FROM_CONTEXT","user_message":""}`))}

	_, err := (SemanticClassifierModelAdapter{Model: model, RuntimeContext: []string{"tool catalog v1"}}).GenerateSemanticClassifierOutput(
		context.Background(),
		BuildSemanticClassifierPrompt(ClassifierInput{UserPrompt: "status"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.request.RuntimeContext) != 1 || model.request.RuntimeContext[0] != "tool catalog v1" {
		t.Fatalf("runtime context = %#v", model.request.RuntimeContext)
	}
}

func TestSemanticClassifierModelAdapterRequiresModel(t *testing.T) {
	_, err := (SemanticClassifierModelAdapter{}).GenerateSemanticClassifierOutput(context.Background(), BuildSemanticClassifierPrompt(ClassifierInput{UserPrompt: "status"}))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSemanticClassifierModelAdapterRequiresMessages(t *testing.T) {
	_, err := (SemanticClassifierModelAdapter{Model: &fakePortModel{}}).GenerateSemanticClassifierOutput(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSemanticClassifierModelAdapterValidatesMessages(t *testing.T) {
	_, err := (SemanticClassifierModelAdapter{Model: &fakePortModel{}}).GenerateSemanticClassifierOutput(context.Background(), []llm.Message{{Role: llm.RoleUser}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewSemanticRequestClassifierFromModel(t *testing.T) {
	model := &fakePortModel{message: llm.NewMessage(llm.RoleAssistant, llm.TextPart(`{"route":"ASK_USER","user_message":"Need account"}`))}
	classifier := NewSemanticRequestClassifierFromModel(model)

	output, err := classifier.Classify(context.Background(), ClassifierInput{UserPrompt: "run"})
	if err != nil {
		t.Fatal(err)
	}
	if output.Route != RouteAskUser {
		t.Fatalf("route = %q", output.Route)
	}
}

func TestSemanticClassifierModelAdapterPropagatesModelError(t *testing.T) {
	modelErr := errors.New("model failed")
	_, err := (SemanticClassifierModelAdapter{Model: &fakePortModel{err: modelErr}}).GenerateSemanticClassifierOutput(context.Background(), BuildSemanticClassifierPrompt(ClassifierInput{UserPrompt: "status"}))
	if !errors.Is(err, modelErr) {
		t.Fatalf("error = %v", err)
	}
}
