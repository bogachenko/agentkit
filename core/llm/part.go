package llm

import (
	"fmt"
	"strings"
)

// PartType keeps text, tool calls, and tool results.
type PartType string

const (
	PartText             PartType = "text"
	PartFunctionCall     PartType = "function_call"
	PartFunctionResponse PartType = "function_response"
)

// Part is the smallest neutral unit of LLM conversation data.
type Part struct {
	Type   PartType
	Text   string
	Name   string
	Args   map[string]any
	Result any
}

// Constructor prevents callers from manually assembling text parts inconsistently.
func TextPart(text string) Part {
	return Part{
		Type: PartText,
		Text: text,
	}
}

// Constructor normalizes empty tool arguments and keeps tool-call shape stable.
func FunctionCallPart(name string, args map[string]any) Part {
	if args == nil {
		args = map[string]any{}
	}

	return Part{
		Type: PartFunctionCall,
		Name: name,
		Args: args,
	}
}

// Constructor keeps tool results represented as conversation data, not runtime side effects.
func FunctionResponsePart(name string, result any) Part {
	return Part{
		Type:   PartFunctionResponse,
		Name:   name,
		Result: result,
	}
}

// Validation blocks malformed conversation parts before adapters or runtime consume them.
func (p Part) Validate() error {
	switch p.Type {
	case PartText:
		if strings.TrimSpace(p.Text) == "" {
			return fmt.Errorf("text part requires non-empty text")
		}
		return nil

	case PartFunctionCall:
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("function_call part requires non-empty name")
		}
		return nil

	case PartFunctionResponse:
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("function_response part requires non-empty name")
		}
		return nil

	default:
		return fmt.Errorf("unknown llm part type %q", string(p.Type))
	}
}
