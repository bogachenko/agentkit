package runtime

import (
	"fmt"

	"github.com/bogachenko/agentkit/core/llm"
	"github.com/bogachenko/agentkit/core/tool"
)

const RouteDecisionFunctionName = "route_decision"

// ExtractRouteDecision accepts only explicit structured model routing output.
func ExtractRouteDecision(message llm.Message) (RouteDecision, error) {
	if err := message.Validate(); err != nil {
		return RouteDecision{}, err
	}

	var calls []llm.Part
	for _, part := range message.Parts {
		if part.Type == llm.PartFunctionCall {
			calls = append(calls, part)
		}
	}

	if len(calls) == 0 {
		return RouteDecision{
			Kind:   RouteKindRespond,
			Reason: "assistant returned a final response without tool routing",
		}, nil
	}

	if len(calls) != 1 {
		return RouteDecision{}, fmt.Errorf("expected exactly one route decision function call, got %d", len(calls))
	}

	call := calls[0]
	if call.Name != RouteDecisionFunctionName {
		return RouteDecision{}, fmt.Errorf("expected %q function call, got %q", RouteDecisionFunctionName, call.Name)
	}

	decision, err := routeDecisionFromArgs(call.Args)
	if err != nil {
		return RouteDecision{}, err
	}

	if err := decision.Validate(); err != nil {
		return RouteDecision{}, err
	}

	return decision, nil
}

// routeDecisionFromArgs parses structured routing fields without keyword classification.
func routeDecisionFromArgs(args map[string]any) (RouteDecision, error) {
	kindText, err := requiredString(args, "kind")
	if err != nil {
		return RouteDecision{}, err
	}

	reason, err := requiredString(args, "reason")
	if err != nil {
		return RouteDecision{}, err
	}

	decision := RouteDecision{
		Kind:   RouteKind(kindText),
		Reason: reason,
	}

	if value, ok := args["tool_name"]; ok {
		text, ok := value.(string)
		if !ok {
			return RouteDecision{}, fmt.Errorf("route decision tool_name must be string")
		}

		decision.ToolName = tool.Name(text)
	}

	if value, ok := args["tool_args"]; ok {
		toolArgs, ok := value.(map[string]any)
		if !ok {
			return RouteDecision{}, fmt.Errorf("route decision tool_args must be object")
		}

		decision.ToolArgs = toolArgs
	}

	if value, ok := args["failure"]; ok {
		failureArgs, ok := value.(map[string]any)
		if !ok {
			return RouteDecision{}, fmt.Errorf("route decision failure must be object")
		}

		failure, err := failureFromArgs(failureArgs)
		if err != nil {
			return RouteDecision{}, err
		}

		decision.Failure = &failure
	}

	return decision, nil
}

// requiredString keeps structured route decoding strict and explicit.
func requiredString(args map[string]any, key string) (string, error) {
	value, exists := args[key]
	if !exists {
		return "", fmt.Errorf("route decision %s is required", key)
	}

	text, ok := value.(string)
	if !ok || text == "" {
		return "", fmt.Errorf("route decision %s must be non-empty string", key)
	}

	return text, nil
}

// failureFromArgs preserves machine-readable failure codes from structured route output.
func failureFromArgs(args map[string]any) (Failure, error) {
	codeText, err := requiredString(args, "code")
	if err != nil {
		return Failure{}, err
	}

	message, err := requiredString(args, "message")
	if err != nil {
		return Failure{}, err
	}

	failure := Failure{
		Code:    FailureCode(codeText),
		Message: message,
	}

	if err := failure.Validate(); err != nil {
		return Failure{}, err
	}

	return failure, nil
}
