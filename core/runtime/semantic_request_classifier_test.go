package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bogachenko/agentkit/core/llm"
)

type fakeSemanticClassifierModel struct {
	called   bool
	messages []llm.Message
	message  llm.Message
	err      error
}

func (m *fakeSemanticClassifierModel) GenerateSemanticClassifierOutput(_ context.Context, messages []llm.Message) (llm.Message, error) {
	m.called = true
	m.messages = messages
	if m.err != nil {
		return llm.Message{}, m.err
	}
	return m.message, nil
}

func TestSemanticRequestClassifierBuildsPromptCallsModelAndExtractsOutput(t *testing.T) {
	model := &fakeSemanticClassifierModel{message: llm.NewMessage(llm.RoleAssistant, llm.TextPart(`{"route":"ANSWER_FROM_CONTEXT","user_message":""}`))}
	classifier := SemanticRequestClassifier{Model: model}

	output, err := classifier.Classify(context.Background(), ClassifierInput{UserPrompt: "что получилось?"})
	if err != nil {
		t.Fatal(err)
	}
	if !model.called {
		t.Fatal("model was not called")
	}
	if !strings.Contains(semanticPromptText(model.messages), "что получилось?") {
		t.Fatalf("prompt missing user prompt: %s", semanticPromptText(model.messages))
	}
	if output.Route != RouteAnswerFromContext {
		t.Fatalf("route = %q", output.Route)
	}
}

func TestSemanticRequestClassifierRequiresModel(t *testing.T) {
	_, err := (SemanticRequestClassifier{}).Classify(context.Background(), ClassifierInput{UserPrompt: "status"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSemanticRequestClassifierPropagatesModelError(t *testing.T) {
	modelErr := errors.New("model failed")
	_, err := (SemanticRequestClassifier{Model: &fakeSemanticClassifierModel{err: modelErr}}).Classify(context.Background(), ClassifierInput{UserPrompt: "status"})
	if !errors.Is(err, modelErr) {
		t.Fatalf("error = %v", err)
	}
}
