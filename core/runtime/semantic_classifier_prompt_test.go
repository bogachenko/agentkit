package runtime

import (
	"strings"
	"testing"

	"github.com/bogachenko/agentkit/core/llm"
)

func TestBuildSemanticClassifierPromptIncludesRoutesAndStrictJSON(t *testing.T) {
	text := semanticPromptText(BuildSemanticClassifierPrompt(ClassifierInput{UserPrompt: "status"}))
	for _, value := range []string{"DIRECT_ANSWER", "EXECUTE_TASK", "ANSWER_FROM_CONTEXT", "ASK_USER", "REJECT_UNSUPPORTED", "\"route\"", "\"user_message\""} {
		if !strings.Contains(text, value) {
			t.Fatalf("prompt missing %q:\n%s", value, text)
		}
	}
	if strings.Contains(text, "```") {
		t.Fatalf("prompt contains markdown fence: %s", text)
	}
}

func TestBuildSemanticClassifierPromptIncludesLedgerAndActiveTask(t *testing.T) {
	text := semanticPromptText(BuildSemanticClassifierPrompt(ClassifierInput{
		UserPrompt: "что получилось?",
		ActiveTask: ActiveTaskState{Active: true, OriginalRequest: "check campaign"},
		RunLedger:  &RunLedger{UserGoal: "audit listing", DataRefs: []string{"source.xlsx"}},
	}))

	for _, value := range []string{"что получилось?", "check campaign", "audit listing", "source.xlsx", "Short follow-ups"} {
		if !strings.Contains(text, value) {
			t.Fatalf("prompt missing %q:\n%s", value, text)
		}
	}
}

func TestBuildSemanticClassifierPromptIncludesToolsAndSources(t *testing.T) {
	text := semanticPromptText(BuildSemanticClassifierPrompt(ClassifierInput{
		UserPrompt: "run",
		Tools: []ToolCatalogItem{[
			{
				Name:           "browser_open",
				Description:    "open page",
				RequiredInputs: []string{"url"},
				Available:      true,
			},
		]},
		CredentialsOrSources: []string{"gmail connected"},
	}))

	for _, value := range []string{"browser_open", "open page", "url", "available: true", "gmail connected"} {
		if !strings.Contains(text, value) {
			t.Fatalf("prompt missing %q:\n%s", value, text)
		}
	}
}

func semanticPromptText(messages []llm.Message) string {
	var b strings.Builder
	for _, message := range messages {
		for _, part := range message.Parts {
			b.WriteString(part.Text)
			b.WriteString("\n")
		}
	}
	return b.String()
}
