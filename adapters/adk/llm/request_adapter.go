package llm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	corellm "github.com/bogachenko/agentkit/core/llm"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// RequestToCore isolates ADK request shape from provider-neutral AgentKit LLM contracts.
func RequestToCore(req *model.LLMRequest, runtimeText []string) corellm.Request {
	if req == nil {
		return corellm.Request{}
	}

	result := corellm.Request{
		RuntimeContext: runtimeText,
	}

	if req.Config != nil && req.Config.SystemInstruction != nil {
		result.System = ExtractText(req.Config.SystemInstruction)
	}

	for _, content := range req.Contents {
		message, ok := ContentToCoreMessage(content)
		if !ok {
			continue
		}

		result.Messages = append(result.Messages, message)
	}

	return result
}

// ApplyCoreMessages rewrites only ADK request contents and keeps system/config/tools untouched.
func ApplyCoreMessages(req *model.LLMRequest, messages []corellm.Message) error {
	if req == nil {
		return fmt.Errorf("adk llm request is required")
	}

	converted := make([]*genai.Content, 0, len(messages))
	for index, message := range messages {
		content, err := CoreMessageToContent(message)
		if err != nil {
			return fmt.Errorf("message %d: %w", index, err)
		}

		converted = append(converted, content)
	}

	req.Contents = converted
	return nil
}

// ContentToCoreMessage converts ADK content into core history without preserving runtime-only instructions.
func ContentToCoreMessage(content *genai.Content) (corellm.Message, bool) {
	if content == nil {
		return corellm.Message{}, false
	}

	message := corellm.Message{
		Role: ToCoreRole(content.Role),
	}

	for _, part := range content.Parts {
		if part == nil {
			continue
		}

		if strings.TrimSpace(part.Text) != "" {
			if IsRuntimeHarnessInstructionText(part.Text) {
				continue
			}

			message.Parts = append(message.Parts, corellm.TextPart(part.Text))
		}

		if part.FunctionCall != nil {
			message.Parts = append(message.Parts, corellm.FunctionCallPart(
				part.FunctionCall.Name,
				part.FunctionCall.Args,
			))
		}

		if part.FunctionResponse != nil {
			message.Parts = append(message.Parts, corellm.FunctionResponsePart(
				part.FunctionResponse.Name,
				part.FunctionResponse.Response,
			))
		}
	}

	if len(message.Parts) == 0 {
		return corellm.Message{}, false
	}

	return message, true
}

// CoreMessageToContent converts core history back to ADK content without runtime policy decisions.
func CoreMessageToContent(message corellm.Message) (*genai.Content, error) {
	if err := message.Validate(); err != nil {
		return nil, err
	}

	parts := make([]*genai.Part, 0, len(message.Parts))

	for _, part := range message.Parts {
		switch part.Type {
		case corellm.PartText:
			parts = append(parts, &genai.Part{
				Text: part.Text,
			})

		case corellm.PartFunctionCall:
			parts = append(parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					Name: part.Name,
					Args: part.Args,
				},
			})

		case corellm.PartFunctionResponse:
			parts = append(parts, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     part.Name,
					Response: SanitizeFunctionResponse(part.Result),
				},
			})

		default:
			return nil, fmt.Errorf("unsupported llm part type %q", string(part.Type))
		}
	}

	return &genai.Content{
		Role:  ToADKRole(message.Role),
		Parts: parts,
	}, nil
}

// SanitizeFunctionResponse prevents oversized or binary tool results from entering ADK model context.
func SanitizeFunctionResponse(value any) map[string]any {
	sanitized := sanitizeFunctionResponseValue(value, 0)

	if response, ok := sanitized.(map[string]any); ok && response != nil {
		return response
	}

	return map[string]any{
		"value": sanitized,
	}
}

// sanitizeFunctionResponseValue applies deterministic size limits without semantic rewriting.
func sanitizeFunctionResponseValue(value any, depth int) any {
	const maxDepth = 8
	const maxArrayItems = 50
	const maxStringRunes = 4000
	const maxObjectKeys = 100

	if depth > maxDepth {
		return "[truncated: max depth]"
	}

	switch typed := value.(type) {
	case nil:
		return nil

	case string:
		return TruncateRunes(typed, maxStringRunes)

	case bool:
		return typed

	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return typed

	case json.Number:
		return typed.String()

	case map[string]any:
		result := make(map[string]any, len(typed))
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		if len(keys) > maxObjectKeys {
			result["_truncated_keys"] = len(keys) - maxObjectKeys
			keys = keys[:maxObjectKeys]
		}

		for _, key := range keys {
			if shouldDropFunctionResponseKey(key) {
				result[key] = "[omitted]"
				continue
			}

			result[key] = sanitizeFunctionResponseValue(typed[key], depth+1)
		}

		return result

	case []any:
		limit := len(typed)
		truncated := 0
		if limit > maxArrayItems {
			truncated = limit - maxArrayItems
			limit = maxArrayItems
		}

		result := make([]any, 0, limit)
		for index := 0; index < limit; index++ {
			result = append(result, sanitizeFunctionResponseValue(typed[index], depth+1))
		}

		if truncated > 0 {
			result = append(result, map[string]any{
				"_truncated_items": truncated,
			})
		}

		return result

	default:
		data, err := json.Marshal(typed)
		if err == nil {
			var decoded any
			if json.Unmarshal(data, &decoded) == nil {
				return sanitizeFunctionResponseValue(decoded, depth+1)
			}
		}

		return TruncateRunes(fmt.Sprintf("%v", typed), maxStringRunes)
	}
}

// Binary-like fields are not safe model context and must be represented as omitted.
func shouldDropFunctionResponseKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))

	switch normalized {
	case "image", "images", "primary_image", "images360", "pdf", "file", "files", "binary":
		return true
	default:
		return false
	}
}
