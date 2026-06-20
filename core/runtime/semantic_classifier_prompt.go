package runtime

import (
	"fmt"
	"strings"

	"github.com/bogachenko/agentkit/core/llm"
)

func BuildSemanticClassifierPrompt(input ClassifierInput) []llm.Message {
	return []llm.Message{
		llm.NewMessage(llm.RoleUser, llm.TextPart(semanticClassifierInstructionText())),
		llm.NewMessage(llm.RoleUser, llm.TextPart(semanticClassifierInputText(input))),
	}
}

func semanticClassifierInstructionText() string {
	return strings.TrimSpace(`You are a semantic request classifier. Choose exactly one route.

Routes:

DIRECT_ANSWER:
Use only when the request can be answered without tools and without relying on unavailable fresh external data.

EXECUTE_TASK:
Use when the user asks to fetch, inspect, create, modify, export, calculate through tools, browse, use files, use connected sources, or perform a concrete task.

ANSWER_FROM_CONTEXT:
Use when the latest user request can be answered from RunLedger, LedgerSummary, ActiveTask, artifacts, or conversation context without fresh tool calls.

ASK_USER:
Use when required input, credentials, source, file, account, marketplace, or choice is missing.

REJECT_UNSUPPORTED:
Use when the request cannot be supported safely or by available tools.

Follow-up rules:
Short follow-ups such as "ok", "да", "yes", "продолжай", "continue", "что получилось?", "покажи", "скачай", "сделай", "1", "2" must be interpreted using ActiveTask and RunLedger.
If RunLedger or ActiveTask contains enough information to answer, choose ANSWER_FROM_CONTEXT.
If the follow-up asks to continue, retry, export, create, download, inspect a new source, refresh, reload, or fetch new data, choose EXECUTE_TASK.
If the follow-up depends on a missing choice or missing account/source/file, choose ASK_USER.
Do not turn task execution into DIRECT_ANSWER when tools, files, connected sources, or fresh external data are required.

Return exactly one JSON object and no markdown:

{
  "route": "DIRECT_ANSWER | EXECUTE_TASK | ANSWER_FROM_CONTEXT | ASK_USER | REJECT_UNSUPPORTED",
  "user_message": "..."
}

Rules:
- user_message is required only for DIRECT_ANSWER, ASK_USER, REJECT_UNSUPPORTED.
- user_message must be empty for EXECUTE_TASK and ANSWER_FROM_CONTEXT.`)
}

func semanticClassifierInputText(input ClassifierInput) string {
	var b strings.Builder
	writeClassifierSection(&b, "latest_user_prompt", strings.TrimSpace(input.UserPrompt))
	writeClassifierSection(&b, "conversation_context", compactListText(input.ConversationContext))
	writeClassifierSection(&b, "pending_user_input", strings.TrimSpace(input.PendingUserInput))
	writeClassifierSection(&b, "active_task", activeTaskText(input.ActiveTask))
	writeClassifierSection(&b, "run_ledger_summary", semanticRunLedgerSummaryText(effectiveLedgerSummary(input)))
	writeClassifierSection(&b, "tools", toolsText(input.Tools))
	writeClassifierSection(&b, "artifacts", compactListText(input.Artifacts))
	writeClassifierSection(&b, "credentials_or_sources", compactListText(input.CredentialsOrSources))
	writeClassifierSection(&b, "skills", compactListText(input.Skills))
	writeClassifierSection(&b, "session_constraints", compactListText(input.SessionConstraints))
	return strings.TrimSpace(b.String())
}

func writeClassifierSection(b *strings.Builder, name string, body string) {
	body = strings.TrimSpace(body)
	if body == "" {
		body = "none"
	}
	b.WriteString("<" + name + ">\n")
	b.WriteString(body)
	b.WriteString("\n</" + name + ">\n\n")
}

func toolsText(tools []ToolCatalogItem) string {
	if len(tools) == 0 {
		return ""
	}
	var b strings.Builder
	for _, item := range tools {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = "unnamed_tool"
		}
		b.WriteString("- name: " + name + "\n")
		b.WriteString(fmt.Sprintf("  available: %t\n", item.Available))
		if required := compactStrings(item.RequiredInputs); len(required) > 0 {
			b.WriteString("  required_inputs: " + strings.Join(required, ", ") + "\n")
		}
		if description := strings.TrimSpace(item.Description); description != "" {
			b.WriteString("  description: " + description + "\n")
		}
	}
	return b.String()
}

func compactListText(values []string) string {
	values = compactStrings(values)
	if len(values) == 0 {
		return ""
	}
	var b strings.Builder
	for _, value := range values {
		b.WriteString("- " + value + "\n")
	}
	return b.String()
}
