package runtime

import "fmt"

// StepSource separates runtime execution provenance from LLM conversation authorship.
type StepSource string

const (
	StepSourceUser      StepSource = "user"
	StepSourceModel     StepSource = "model"
	StepSourceTool      StepSource = "tool"
	StepSourceRuntime   StepSource = "runtime"
	StepSourcePolicy    StepSource = "policy"
	StepSourceValidator StepSource = "validator"
)

// Validation keeps runtime-only provenance explicit and separate from llm.Role.
func (s StepSource) Validate() error {
	switch s {
	case StepSourceUser, StepSourceModel, StepSourceTool, StepSourceRuntime, StepSourcePolicy, StepSourceValidator:
		return nil
	default:
		return fmt.Errorf("unknown step source %q", string(s))
	}
}
