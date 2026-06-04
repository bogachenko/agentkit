package llm

import (
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// ExtractText reads only text parts so system instructions stay provider-neutral.
func ExtractText(content *genai.Content) string {
	if content == nil {
		return ""
	}

	var builder strings.Builder

	for _, part := range content.Parts {
		if part == nil || strings.TrimSpace(part.Text) == "" {
			continue
		}

		if IsRuntimeHarnessInstructionText(part.Text) {
			continue
		}

		if builder.Len() > 0 {
			builder.WriteString("\n")
		}

		builder.WriteString(part.Text)
	}

	return builder.String()
}

// IsRuntimeHarnessInstructionText prevents runtime-only instructions from becoming normal conversation history.
func IsRuntimeHarnessInstructionText(text string) bool {
	text = strings.TrimSpace(text)

	return strings.HasPrefix(text, "<runtime_harness_instruction>") &&
		strings.Contains(text, "</runtime_harness_instruction>")
}

// TruncateRunes enforces deterministic context-size limits without corrupting UTF-8.
func TruncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}

	return string(runes[:limit]) + fmt.Sprintf("...[truncated %d chars]", len(runes)-limit)
}
