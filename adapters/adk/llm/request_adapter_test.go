package llm

import (
	"context"
	"errors"
	"iter"
	"testing"

	corellm "github.com/bogachenko/agentkit/core/llm"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type fakeADKModel struct {
	lastStream bool
}

func (m *fakeADKModel) Name() string {
	return "fake"
}

func (m *fakeADKModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	m.lastStream = stream

	return func(yield func(*model.LLMResponse, error) bool) {
		yield(&model.LLMResponse{
			Content: genai.NewContentFromText("ok", genai.RoleModel),
		}, nil)
	}
}

func TestContentToCoreMessageConvertsText(t *testing.T) {
	content := genai.NewContentFromText("hello", genai.RoleUser)

	message, ok := ContentToCoreMessage(content)
	if !ok {
		t.Fatal("expected message to be converted")
	}

	if message.Role != corellm.RoleUser {
		t.Fatalf("expected user role, got %q", message.Role)
	}

	if len(message.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(message.Parts))
	}

	if message.Parts[0].Text != "hello" {
		t.Fatalf("expected hello text, got %q", message.Parts[0].Text)
	}
}

func TestContentToCoreMessageSkipsRuntimeHarnessInstruction(t *testing.T) {
	content := genai.NewContentFromText(
		"<runtime_harness_instruction>internal</runtime_harness_instruction>",
		genai.RoleUser,
	)

	_, ok := ContentToCoreMessage(content)
	if ok {
		t.Fatal("expected runtime harness instruction to be skipped")
	}
}

func TestCoreMessageToContentConvertsText(t *testing.T) {
	message := corellm.NewMessage(corellm.RoleAssistant, corellm.TextPart("hello"))

	content, err := CoreMessageToContent(message)
	if err != nil {
		t.Fatalf("expected conversion to succeed, got error: %v", err)
	}

	if content.Role != genai.RoleModel {
		t.Fatalf("expected model role, got %q", content.Role)
	}

	if len(content.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(content.Parts))
	}

	if content.Parts[0].Text != "hello" {
		t.Fatalf("expected hello text, got %q", content.Parts[0].Text)
	}
}

func TestCoreMessageToContentConvertsFunctionCall(t *testing.T) {
	message := corellm.NewMessage(
		corellm.RoleAssistant,
		corellm.FunctionCallPart("search_products", map[string]any{
			"query": "box",
		}),
	)

	content, err := CoreMessageToContent(message)
	if err != nil {
		t.Fatalf("expected conversion to succeed, got error: %v", err)
	}

	if content.Parts[0].FunctionCall == nil {
		t.Fatal("expected function call")
	}

	if content.Parts[0].FunctionCall.Name != "search_products" {
		t.Fatalf("expected search_products, got %q", content.Parts[0].FunctionCall.Name)
	}
}

func TestCoreMessageToContentConvertsFunctionResponseWithSanitization(t *testing.T) {
	message := corellm.NewMessage(
		corellm.RoleTool,
		corellm.FunctionResponsePart("load_file", map[string]any{
			"file": "binary-content",
			"ok":   true,
		}),
	)

	content, err := CoreMessageToContent(message)
	if err != nil {
		t.Fatalf("expected conversion to succeed, got error: %v", err)
	}

	response := content.Parts[0].FunctionResponse
	if response == nil {
		t.Fatal("expected function response")
	}

	if response.Response["file"] != "[omitted]" {
		t.Fatalf("expected file to be omitted, got %v", response.Response["file"])
	}

	if response.Response["ok"] != true {
		t.Fatalf("expected ok=true, got %v", response.Response["ok"])
	}
}

func TestRequestToCoreExtractsSystemAndMessages(t *testing.T) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			genai.NewContentFromText("hello", genai.RoleUser),
		},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText("system", genai.RoleUser),
		},
	}

	result := RequestToCore(req, []string{"runtime"})

	if result.System != "system" {
		t.Fatalf("expected system instruction, got %q", result.System)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}

	if len(result.RuntimeContext) != 1 || result.RuntimeContext[0] != "runtime" {
		t.Fatalf("expected runtime context to be preserved, got %#v", result.RuntimeContext)
	}
}

func TestApplyCoreMessagesRewritesContents(t *testing.T) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			genai.NewContentFromText("old", genai.RoleUser),
		},
	}

	err := ApplyCoreMessages(req, []corellm.Message{
		corellm.NewMessage(corellm.RoleUser, corellm.TextPart("new")),
	})
	if err != nil {
		t.Fatalf("expected apply to succeed, got error: %v", err)
	}

	if len(req.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(req.Contents))
	}

	if req.Contents[0].Parts[0].Text != "new" {
		t.Fatalf("expected new content, got %q", req.Contents[0].Parts[0].Text)
	}
}

func TestApplyCoreMessagesRejectsNilRequest(t *testing.T) {
	err := ApplyCoreMessages(nil, []corellm.Message{
		corellm.NewMessage(corellm.RoleUser, corellm.TextPart("new")),
	})

	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestSanitizeFunctionResponseWrapsScalar(t *testing.T) {
	result := SanitizeFunctionResponse("ok")

	if result["value"] != "ok" {
		t.Fatalf("expected scalar to be wrapped, got %#v", result)
	}
}

func TestSanitizeFunctionResponseTruncatesLargeArray(t *testing.T) {
	values := make([]any, 0, 51)
	for index := 0; index < 51; index++ {
		values = append(values, index)
	}

	result := SanitizeFunctionResponse(map[string]any{
		"items": values,
	})

	items, ok := result["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, got %#v", result["items"])
	}

	if len(items) != 51 {
		t.Fatalf("expected 50 items plus truncation marker, got %d", len(items))
	}
}

func TestNonStreamingModelForcesStreamFalse(t *testing.T) {
	inner := &fakeADKModel{}
	wrapped := NewNonStreamingModel(inner)

	for _, err := range wrapped.GenerateContent(context.Background(), &model.LLMRequest{}, true) {
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	}

	if inner.lastStream {
		t.Fatal("expected wrapped model to force stream=false")
	}
}

func TestNewNonStreamingModelAcceptsNil(t *testing.T) {
	if NewNonStreamingModel(nil) != nil {
		t.Fatal("expected nil wrapper for nil model")
	}
}

func TestCoreMessageToContentRejectsInvalidMessage(t *testing.T) {
	_, err := CoreMessageToContent(corellm.Message{
		Role: corellm.RoleUser,
	})

	if err == nil {
		t.Fatal("expected error for invalid message")
	}
}

func TestFakeModelCanReturnError(t *testing.T) {
	expected := errors.New("model failed")
	sequence := func(yield func(*model.LLMResponse, error) bool) {
		yield(nil, expected)
	}

	for _, err := range sequence {
		if !errors.Is(err, expected) {
			t.Fatalf("expected propagated error, got %v", err)
		}
	}
}
