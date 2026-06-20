package runtime

import (
	"testing"

	"github.com/bogachenko/agentkit/core/llm"
)

func TestExtractSemanticClassifierOutputFromJSONText(t *testing.T) {
	output, err := ExtractSemanticClassifierOutput(llm.NewMessage(llm.RoleAssistant, llm.TextPart(`{"route":"ASK_USER","user_message":"Need account ID"}`)))
	if err != nil {
		t.Fatal(err)
	}
	if output.Route != RouteAskUser || output.UserMessage != "Need account ID" {
		t.Fatalf("output = %#v", output)
	}
}

func TestExtractSemanticClassifierOutputFromFunctionCall(t *testing.T) {
	output, err := ExtractSemanticClassifierOutput(llm.NewMessage(llm.RoleAssistant, llm.FunctionCallPart(SemanticClassifierFunctionName, map[string]any{
		"route":        "EXECUTE_TASK",
		"user_message": "",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if output.Route != RouteExecuteTask {
		t.Fatalf("route = %q", output.Route)
	}
}

func TestExtractSemanticClassifierOutputRejectsExecuteTaskWithUserMessage(t *testing.T) {
	_, err := ExtractSemanticClassifierOutput(llm.NewMessage(llm.RoleAssistant, llm.TextPart(`{"route":"EXECUTE_TASK","user_message":"ok"}`)))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractSemanticClassifierOutputRejectsDirectAnswerWithoutMessage(t *testing.T) {
	_, err := ExtractSemanticClassifierOutput(llm.NewMessage(llm.RoleAssistant, llm.TextPart(`{"route":"DIRECT_ANSWER","user_message":""}`)))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractSemanticClassifierOutputRejectsMarkdownJSON(t *testing.T) {
	_, err := ExtractSemanticClassifierOutput(llm.NewMessage(llm.RoleAssistant, llm.TextPart("```json\n{\"route\":\"ASK_USER\",\"user_message\":\"Need account ID\"}\n```")))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractSemanticClassifierOutputRejectsUnknownFunctionName(t *testing.T) {
	_, err := ExtractSemanticClassifierOutput(llm.NewMessage(llm.RoleAssistant, llm.FunctionCallPart("other", map[string]any{
		"route":        "EXECUTE_TASK",
		"user_message": "",
	})))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractSemanticClassifierOutputRejectsMultipleFunctionCalls(t *testing.T) {
	_, err := ExtractSemanticClassifierOutput(llm.NewMessage(
		llm.RoleAssistant,
		llm.FunctionCallPart(SemanticClassifierFunctionName, map[string]any{"route": "EXECUTE_TASK", "user_message": ""}),
		llm.FunctionCallPart(SemanticClassifierFunctionName, map[string]any{"route": "EXECUTE_TASK", "user_message": ""}),
	))
	if err == nil {
		t.Fatal("expected error")
	}
}
