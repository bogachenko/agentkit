package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/bogachenko/agentkit/core/llm"
)

const SemanticClassifierFunctionName = "semantic_request_classifier"

func ExtractSemanticClassifierOutput(message llm.Message) (ClassifierOutput, error) {
	functionCalls := make([]llm.Part, 0, 1)
	texts := make([]string, 0, len(message.Parts))
	for _, part := range message.Parts {
		switch part.Type {
		case llm.PartFunctionCall:
			functionCalls = append(functionCalls, part)
		case llm.PartText:
			if text := strings.TrimSpace(part.Text); text != "" {
				texts = append(texts, text)
			}
		}
	}

	if len(functionCalls) > 0 {
		if len(functionCalls) != 1 {
			return ClassifierOutput{}, fmt.Errorf("semantic classifier function call expected exactly one call, got %d", len(functionCalls))
		}
		return extractSemanticClassifierFunctionCall(functionCalls[0])
	}

	if len(texts) == 0 {
		return ClassifierOutput{}, fmt.Errorf("semantic classifier output must be valid JSON object")
	}
	return extractSemanticClassifierJSON(strings.Join(texts, "\n"))
}

func extractSemanticClassifierFunctionCall(part llm.Part) (ClassifierOutput, error) {
	if part.Name != SemanticClassifierFunctionName {
		return ClassifierOutput{}, fmt.Errorf("semantic classifier function call expected %s, got %s", SemanticClassifierFunctionName, part.Name)
	}
	return classifierOutputFromMap(part.Args)
}

func extractSemanticClassifierJSON(text string) (ClassifierOutput, error) {
	text = strings.TrimSpace(text)
	if strings.Contains(text, "```") {
		return ClassifierOutput{}, fmt.Errorf("semantic classifier output must be valid JSON object")
	}

	var raw map[string]any
	decoder := json.NewDecoder(strings.NewReader(text))
	if err := decoder.Decode(&raw); err != nil {
		return ClassifierOutput{}, fmt.Errorf("semantic classifier output must be valid JSON object: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return ClassifierOutput{}, fmt.Errorf("semantic classifier output must be valid JSON object: %w", err)
	} else if err == nil {
		return ClassifierOutput{}, fmt.Errorf("semantic classifier output must be valid JSON object")
	}
	return classifierOutputFromMap(raw)
}

func classifierOutputFromMap(raw map[string]any) (ClassifierOutput, error) {
	routeValue, ok := raw["route"]
	if !ok {
		return ClassifierOutput{}, fmt.Errorf("semantic classifier output requires route")
	}
	route, ok := routeValue.(string)
	if !ok || strings.TrimSpace(route) == "" {
		return ClassifierOutput{}, fmt.Errorf("semantic classifier output requires route")
	}

	messageValue, ok := raw["user_message"]
	if !ok {
		return ClassifierOutput{}, fmt.Errorf("semantic classifier output user_message must be string")
	}
	userMessage, ok := messageValue.(string)
	if !ok {
		return ClassifierOutput{}, fmt.Errorf("semantic classifier output user_message must be string")
	}

	output := ClassifierOutput{Route: RequestRoute(strings.TrimSpace(route)), UserMessage: strings.TrimSpace(userMessage)}
	if err := output.Validate(); err != nil {
		return ClassifierOutput{}, err
	}
	return output, nil
}
