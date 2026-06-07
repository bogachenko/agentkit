package runtime

import (
	"strings"
	"testing"

	"github.com/bogachenko/agentkit/core/tool"
)

func TestValidationRecoveryInstructionTreatsValidationFailureAsEvidence(t *testing.T) {
	instruction := validationRecoveryInstruction(Step{
		Kind:     StepKindToolResult,
		ToolName: tool.Name("web_resource_download"),
		ToolResult: ToolExecutionResult{
			OK:           false,
			HasEvidence:  true,
			ErrorKind:    ToolErrorValidation,
			ErrorMessage: "invalid_input: resource exceeds max_bytes limit",
			Raw: map[string]any{
				"error": "invalid_input: resource exceeds max_bytes limit",
			},
		},
	})

	required := []string{
		"confirmed tool evidence",
		"produce the final answer from this error",
		"Do not claim the invalid call is prohibited",
	}

	for _, marker := range required {
		if !strings.Contains(instruction, marker) {
			t.Fatalf("validation recovery instruction missing %q:\n%s", marker, instruction)
		}
	}

	if strings.Contains(instruction, "Correct the tool arguments and continue the task") {
		t.Fatalf("validation recovery instruction must not force argument correction:\n%s", instruction)
	}
}
