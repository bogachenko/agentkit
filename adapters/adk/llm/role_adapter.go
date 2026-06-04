package llm

import (
	corellm "github.com/bogachenko/agentkit/core/llm"
	"google.golang.org/genai"
)

// ToCoreRole maps ADK/GenAI roles into AgentKit conversation authorship.
func ToCoreRole(role string) corellm.Role {
	switch role {
	case genai.RoleUser:
		return corellm.RoleUser
	case "tool":
		return corellm.RoleTool
	default:
		return corellm.RoleAssistant
	}
}

// ToADKRole maps AgentKit conversation authorship back to ADK/GenAI roles.
func ToADKRole(role corellm.Role) string {
	switch role {
	case corellm.RoleUser:
		return genai.RoleUser
	case corellm.RoleTool:
		return "tool"
	default:
		return genai.RoleModel
	}
}
